package main

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
	mountpoint  string
	loginRepo   string
	paniced     bool
	preventQuit bool
}

// NewApp creates a new App application struct
func NewApp(ph *ProjectHandler, lh *LogHandler) *App {
	return &App{ph: ph, lh: lh, loginRepo: api.SDConnect}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	filesystem.SetSignalBridge(a.Panic)
}

func (a *App) shutdown(_ context.Context) {
	filesystem.UnmountFilesystem()
}

func (a *App) beforeClose(_ context.Context) (prevent bool) {
	return a.preventQuit
}

// Quit can be used in frontend to quit application
func (a *App) Quit() {
	a.preventQuit = false
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
		logs.Error(fmt.Errorf("dialog gave an error, could not respond to user decision: %w", err))
	} else if result == quitButton {
		wailsruntime.EventsEmit(a.ctx, "saveLogsAndQuit")
	}
}

func (a *App) GetDefaultMountPoint() string {
	var err error
	a.mountpoint, err = mountpoint.DefaultMountPoint()
	if err != nil {
		logs.Warning(err)
	}

	return a.mountpoint
}

func (a *App) GetUsername() string {
	return api.GetUsername()
}

func (a *App) InitializeAPI() (bool, error) {
	defer func() {
		reps := make(map[string]bool)
		enabled := api.GetRepositories()
		for _, r := range api.GetAllRepositories() {
			reps[r] = !slices.Contains(enabled, r)
		}
		wailsruntime.EventsEmit(a.ctx, "setRepositories", reps)
	}()

	ch := make(chan bool)
	go a.monitorScans(ch)

	if err := api.Setup(certs.Files); err != nil {
		logs.Error(err)
		outer, _ := logs.Wrapper(err)

		return false, errors.New(outer)
	}
	access, err := api.GetProfile(ch)
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
	if api.GetProjectType() != "default" {
		wailsruntime.EventsEmit(a.ctx, "findataProject", api.GetUserEmail())
	}

	return access, nil
}

func (a *App) monitorScans(ch <-chan bool) {
	for ok := range ch {
		if ok {
			wailsruntime.EventsEmit(a.ctx, "virusFound")
		} else {
			wailsruntime.EventsEmit(
				a.ctx, "showToast", "Virus scanning was interrupted",
				"Please check the logs for details or contact servicedesk@csc.fi (subject: SD Services) for support.",
			)
		}
	}
}

func (a *App) ChangeMountPoint() (string, error) {
	home, _ := os.UserHomeDir()
	options := wailsruntime.OpenDialogOptions{DefaultDirectory: home, CanCreateDirectories: true}
	mount, err := wailsruntime.OpenDirectoryDialog(a.ctx, options)
	if mount == "" {
		return a.mountpoint, nil
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
	a.mountpoint = mount

	return mount, nil
}

func (a *App) InitFuse() {
	a.preventQuit = true
	go func() {
		var wait = make(chan any)
		go func() {
			<-wait // Wait for fuse to be ready

			var cmd = make(chan []string)
			go mountpoint.WaitForUpdateSignal(cmd)
			go func() {
				for {
					<-cmd
					wailsruntime.EventsEmit(a.ctx, "refresh")
				}
			}()
			go func() {
				buckets := filesystem.GetNodeChildren(api.SDConnect + "/" + api.GetProjectName())
				if len(buckets) > 0 {
					wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
				}
				wailsruntime.EventsEmit(a.ctx, "fuseReady")
			}()
		}()
		ret := filesystem.MountFilesystem(a.mountpoint, a.ph.trackProjectProgress, wait)
		if ret > 0 {
			logs.Errorf("Exit status %d", ret)
		}
	}()
}

func (a *App) OpenFuse() {
	var cmd *exec.Cmd
	userPath := a.mountpoint

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
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", userPath)
	default:
		logs.Errorf("Unrecognized OS")

		return
	}

	if err = cmd.Run(); err != nil {
		logs.Errorf("Could not open directory %s: %w", userPath, err)
	}
}

func (a *App) FilesOpen() bool {
	return filesystem.FilesOpen()
}

func (a *App) RefreshFuse() {
	time.Sleep(200 * time.Millisecond)

	a.ph.DeleteProjects()
	filesystem.RefreshFilesystem()

	buckets := filesystem.GetNodeChildren(api.SDConnect + "/" + api.GetProjectName())
	if len(buckets) > 0 {
		wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
	}
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
