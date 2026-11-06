package main

import (
	"context"
	"errors"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"sda-filesystem/certs"
	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx         context.Context
	ph          *ProjectHandler
	lh          *LogHandler
	initLock    sync.Mutex // Frontend may call InitializeAPI() concurrently when reloaded in dev mode
	mountLock   sync.Mutex
	mountpoint  string
	preventQuit atomic.Bool
	paniced     bool
	fuseStarted atomic.Bool // Frontend may call InitFuse() again when reloaded in dev mode
	mounted     atomic.Bool
}

// NewApp creates a new App application struct
func NewApp(ph *ProjectHandler, lh *LogHandler) *App {
	return &App{ph: ph, lh: lh}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	filesystem.SetSignalBridge(a.Panic)
}

func (a *App) beforeClose(_ context.Context) (prevent bool) {
	return a.preventQuit.Load()
}

// Quit can be used in frontend to quit application
func (a *App) Quit() {
	if a.mounted.Load() {
		if err := filesystem.UnmountFilesystem(); err != nil {
			logs.Error(err)
			wailsruntime.EventsEmit(a.ctx, "unmountFailed")

			return
		}
	}

	a.preventQuit.Store(false)
	wailsruntime.Quit(a.ctx)
}

func (a *App) Panic() {
	if a.paniced {
		return
	}
	a.paniced = true

	quitButton := "Save logs and quit"
	options := wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.ErrorDialog,
		Buttons:       []string{quitButton, "Ignore"},
		DefaultButton: quitButton,
		Title:         "Data Gateway failed to load correctly",
		Message:       "Save logs to find out why this happened and quit the application, or continue at your own peril...",
	}
	result, err := wailsruntime.MessageDialog(a.ctx, options)
	if err != nil {
		logs.Errorf("dialog gave an error, could not respond to user decision: %w", err)
	} else if result == quitButton {
		wailsruntime.EventsEmit(a.ctx, "saveLogsAndQuit")
	}
}

func (a *App) GetDefaultMountPoint() string {
	mount, err := mountpoint.DefaultMountPoint()
	if err != nil {
		logs.Warning(err)
	}
	logs.Info("Default mount point is ", mount)
	a.setMountPoint(mount)

	return mount
}

func (a *App) getMountPoint() string {
	a.mountLock.Lock()
	defer a.mountLock.Unlock()

	return a.mountpoint
}

func (a *App) setMountPoint(mount string) {
	a.mountLock.Lock()
	defer a.mountLock.Unlock()
	a.mountpoint = mount
}

func (a *App) GetVersion() string {
	return api.GetVersion()
}

func (a *App) GetUsername() string {
	return api.GetUsername()
}

func (a *App) InitializeAPI() (bool, error) {
	a.initLock.Lock()
	defer a.initLock.Unlock()

	defer func() {
		reps := make(map[string]bool)
		enabled := api.GetRepositories()
		for _, r := range api.GetAllRepositories() {
			reps[r.ForPath()] = !slices.Contains(enabled, r)
		}
		wailsruntime.EventsEmit(a.ctx, "setRepositories", reps)
	}()

	if err := api.Setup(certs.Files); err != nil {
		logs.Error(err)
		outer, _ := logs.Wrapper(err)

		return false, errors.New(outer)
	}
	access, err := api.GetProfile(a.informSessionExpired, a.informScanningResult)
	if err != nil {
		logs.Error(err)
		outer, _ := logs.Wrapper(err)

		return false, errors.New(outer)
	}
	if !access {
		logs.Errorf("Your session has expired")
	}

	if airlock.ExportPossible() {
		wailsruntime.EventsEmit(a.ctx, "exportPossible")
	}
	if api.FindataUpload() {
		wailsruntime.EventsEmit(a.ctx, "findataProject", api.GetUserEmail())
	}

	return access, nil
}

func (a *App) informSessionExpired() {
	wailsruntime.EventsEmit(a.ctx, "sessionExpired")
}

func (a *App) informScanningResult(found bool) {
	if found {
		wailsruntime.EventsEmit(a.ctx, "virusFound")
	} else {
		wailsruntime.EventsEmit(
			a.ctx, "showToast", "Virus scanning was interrupted",
			"Please check the logs for details or contact servicedesk@csc.fi (subject: SD Services) for support.",
		)
	}
}

func (a *App) UpdateRepositorySelection(selected map[string]bool) {
	api.SetRepositories(selected)
}

func (a *App) ChangeMountPoint() (string, error) {
	home, _ := os.UserHomeDir()
	options := wailsruntime.OpenDialogOptions{DefaultDirectory: home, CanCreateDirectories: true}
	mount, err := wailsruntime.OpenDirectoryDialog(a.ctx, options)
	if mount == "" {
		return a.getMountPoint(), nil
	}
	if err != nil {
		logs.Error(err)

		return "", err
	}

	mount = filepath.Clean(mount)
	logs.Debugf("Trying to change mount point to %s", mount)

	if err := mountpoint.CheckMountPoint(mount); err != nil {
		logs.Error(err)

		return "", err
	}

	logs.Infof("Data Gateway will be mounted at %s", mount)
	a.setMountPoint(mount)

	return mount, nil
}

func (a *App) InitFuse() {
	if !a.fuseStarted.CompareAndSwap(false, true) {
		logs.Debug("Data Gateway has already been initialized")
		// If the filesystem has not yet been mounted, fuseReady will be emitted later
		if a.mounted.Load() {
			wailsruntime.EventsEmit(a.ctx, "fuseReady")
		}

		return
	}

	a.preventQuit.Store(true)
	go func() {
		var wait = make(chan any)
		go func() {
			<-wait // Wait for fuse to be ready
			a.mounted.Store(true)

			var notify = make(chan any, 1)
			go filesystem.WaitForUpdateSignal(notify)
			go func() {
				for {
					<-notify
					wailsruntime.EventsEmit(a.ctx, "update")
				}
			}()
			go func() {
				buckets := filesystem.GetNodeChildren(api.SDConnect.ForPath() + "/" + api.GetProjectName())
				if len(buckets) > 0 {
					wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
				}
				wailsruntime.EventsEmit(a.ctx, "fuseReady")
			}()
		}()
		ret := filesystem.MountFilesystem(a.getMountPoint(), a.ph.trackProjectProgress, wait)
		if ret > 0 {
			logs.Errorf("Exit status %d", ret)
		}
		a.mounted.Store(false)
		a.fuseStarted.Store(false)
	}()
}

func (a *App) OpenFuse() {
	var cmd *exec.Cmd
	userPath := a.getMountPoint()

	_, err := os.Stat(userPath)
	if err != nil {
		logs.Errorf("Failed to find directory %s: %w", userPath, err)

		return
	}

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", userPath)
	case "linux":
		cmd = exec.Command("xdg-open", userPath)
	default:
		logs.Errorf("Unrecognized OS")

		return
	}

	if err = cmd.Run(); err != nil {
		logs.Errorf("Could not open directory %s: %w", userPath, err)
	}
}

func (a *App) FilesOpen() bool {
	isOpen := filesystem.FilesOpen()
	if isOpen {
		continueButton := "Continue and update"
		options := wailsruntime.MessageDialogOptions{
			Type:          wailsruntime.WarningDialog,
			Buttons:       []string{continueButton, "Cancel"},
			DefaultButton: continueButton,
			Title:         "Files are still in use",
			Message:       "You are trying to update Data Gateway while you are still actively using it. Are you sure you want to continue?",
		}
		result, err := wailsruntime.MessageDialog(a.ctx, options)
		if err != nil {
			logs.Errorf("dialog gave an error, could not respond to user decision: %w", err)
		} else if result == continueButton {
			isOpen = false // Ignore open files
		}
	}

	return isOpen
}

func (a *App) UpdateFuse() {
	time.Sleep(200 * time.Millisecond) // So that user can see the progressbar from the beginning

	a.ph.DeleteProjects()
	filesystem.UpdateFilesystem()

	buckets := filesystem.GetNodeChildren(api.SDConnect.ForPath() + "/" + api.GetProjectName())
	if len(buckets) > 0 {
		wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
	}

	time.Sleep(time.Second) // So that progressbar animation is detectable
	wailsruntime.EventsEmit(a.ctx, "fuseReady")
}

func (a *App) SelectFiles() ([]string, error) {
	home, _ := os.UserHomeDir()
	options := wailsruntime.OpenDialogOptions{DefaultDirectory: home}
	files, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, options)
	if err != nil {
		logs.Error(err)

		return nil, err
	}

	return files, nil
}

func (a *App) CheckObjectExistences(set airlock.UploadSet) ([]bool, error) {
	err := airlock.CheckObjectExistences(&set, nil)
	if err != nil {
		logs.Error(err)
		message, _ := logs.Wrapper(err)

		err = errors.New(message)
	}

	return set.Exists, err
}

func (a *App) CheckBucketExistence(bucket string) (bool, error) {
	exists, err := api.BucketExists(api.SDConnect, bucket)
	if err != nil {
		logs.Error(err)
		message, _ := logs.Wrapper(err)

		err = errors.New(message)
	}

	return exists, err
}

func (a *App) WalkDirs(selection, currentObjects []string, prefix string) (airlock.UploadSet, error) {
	set, err := airlock.WalkDirs(selection, currentObjects, prefix)
	if err != nil {
		logs.Error(err)
		message, _ := logs.Wrapper(err)

		err = errors.New(message)
	}

	return set, err
}

func (a *App) ValidateEmail(email string) string {
	e, err := mail.ParseAddress(email)
	if err != nil {
		return ""
	}

	return e.Address
}

func (a *App) ExportFiles(set airlock.UploadSet, exists bool, metadata map[string]string) error {
	var err error
	time.Sleep(1000 * time.Millisecond) // So that progressbar animation is detectable
	if !exists {
		logs.Info("Creating bucket ", set.Bucket)
		err = api.CreateBucket(api.SDConnect, set.Bucket)
	}

	if err == nil {
		err = airlock.Upload(set, metadata)
	}

	if err != nil {
		logs.Error(err)
		message, _ := logs.Wrapper(err)

		return errors.New(message)
	}

	return nil
}
