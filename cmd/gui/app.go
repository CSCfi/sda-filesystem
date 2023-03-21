package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/exp/slices"
)

// App struct
type App struct {
	ctx         context.Context
	ph          *ProjectHandler
	lh          *LogHandler
	fs          *filesystem.Fuse
	mountpoint  string
	paniced     bool
	preventQuit bool
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

func (a *App) shutdown(ctx context.Context) {
	filesystem.UnmountFilesystem()
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return a.preventQuit
}

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
		Message:       "Save logs to find out why this happened and quit the application or continue at your own peril...",
	}
	result, err := wailsruntime.MessageDialog(a.ctx, options)
	if err != nil {
		logs.Error(fmt.Errorf("Dialog gave an error, could not respond to user decision: %w", err))
	} else if result == quitButton {
		a.lh.SaveLogs()
		wailsruntime.Quit(a.ctx)
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

func (a *App) InitializeAPI() error {
	err := api.GetCommonEnvs()
	if err != nil {
		logs.Error(err)

		return fmt.Errorf("Required environmental variables missing")
	}

	err = api.InitializeCache()
	if err != nil {
		logs.Error(err)

		return fmt.Errorf("Initializing cache failed")
	}

	err = api.InitializeClient()
	if err != nil {
		logs.Error(err)

		return fmt.Errorf("Initializing HTTP client failed")
	}

	noneAvailable := true
	reps := make(map[string][2]bool)

	for _, r := range api.GetAllRepositories() {
		if err = api.GetEnvs(r); err != nil {
			logs.Error(err)
		} else {
			noneAvailable = false
		}
		reps[r] = [2]bool{err != nil, r == api.SDConnect}
	}

	wailsruntime.EventsEmit(a.ctx, "setRepositories", reps)

	if noneAvailable {
		return fmt.Errorf("No services available")
	}

	return nil
}

func (a *App) Authenticate(repository string) error {
	if err := api.Authenticate(repository); err != nil {
		logs.Error(err)
		message, _ := logs.Wrapper(err)

		return fmt.Errorf(message)
	}

	return nil
}

func (a *App) Login(repository, username, password string) (bool, error) {
	token := api.BasicToken(username, password)
	if err := api.Authenticate(repository, token, ""); err != nil {
		logs.Error(err)
		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			return false, nil
		}
		message, _ := logs.Wrapper(err)

		return false, fmt.Errorf(message)
	}

	logs.Info("Login successful")

	isManager, err := airlock.IsProjectManager("")
	switch {
	case err != nil:
		logs.Errorf("Resolving project manager status failed: %w", err)
	case !isManager:
		logs.Info("You are not the project manager")
	default:
		logs.Info("You are the project manager")
		if err = airlock.GetPublicKey(); err != nil {
			logs.Error(err)
		} else {
			wailsruntime.EventsEmit(a.ctx, "isProjectManager")
		}
	}

	return true, nil
}

func (a *App) InitFuse() {
	a.preventQuit = true
	a.fs = filesystem.InitializeFileSystem(a.ph.AddProject)
	a.ph.sendProjects()
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

func (a *App) LoadFuse() {
	go func() {
		defer filesystem.CheckPanic()
		a.fs.PopulateFilesystem(a.ph.trackContainers)

		go func() {
			time.Sleep(time.Second)
			buckets := a.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName())
			if len(buckets) > 0 {
				wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
			}
			wailsruntime.EventsEmit(a.ctx, "fuseReady")
		}()

		filesystem.MountFilesystem(a.fs, a.mountpoint)
		os.Exit(0)
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

func (a *App) RefreshFuse() error {
	if a.fs.FilesOpen(a.mountpoint) {
		return fmt.Errorf("You have files in use which prevents updating Data Gateway")
	}
	logs.Info("Updating Data Gateway")
	time.Sleep(200 * time.Millisecond)

	a.ph.deleteProjects()
	newFs := filesystem.InitializeFileSystem(a.ph.AddProject)
	newFs.PopulateFilesystem(a.ph.trackContainers)
	a.fs.RefreshFilesystem(newFs)

	buckets := a.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName())
	if len(buckets) > 0 {
		wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
	}
	wailsruntime.EventsEmit(a.ctx, "fuseReady")

	return nil
}

func (a *App) SelectFile() (string, error) {
	home, _ := os.UserHomeDir()
	options := wailsruntime.OpenDialogOptions{DefaultDirectory: home}
	file, err := wailsruntime.OpenFileDialog(a.ctx, options)
	if err != nil {
		logs.Error(err)

		return "", err
	}

	return file, nil
}

func (a *App) CheckEncryption(file, bucket string) (exists bool, err error) {
	var encrypted bool
	if encrypted, err = airlock.CheckEncryption(file); err != nil {
		logs.Error(err)

		return
	}

	chld := a.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName() + "/" + bucket)
	if encrypted {
		exists = slices.Contains(chld, filepath.Base(file))
		wailsruntime.EventsEmit(a.ctx, "setExportFilenames", "", file)
	} else {
		fileEncrypted := file + ".c4gh"
		exists = slices.Contains(chld, filepath.Base(fileEncrypted))
		wailsruntime.EventsEmit(a.ctx, "setExportFilenames", file, fileEncrypted)
	}

	return
}

func (a *App) ExportFile(folder, origFile, encFile string) error {
	time.Sleep(1000 * time.Millisecond)
	err := airlock.Upload(origFile, encFile, folder, "", 4000, origFile != "")
	if err != nil {
		logs.Error(err)
		message, _ := logs.Wrapper(err)

		return fmt.Errorf(message)
	}

	return nil
}
