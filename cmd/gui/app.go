package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

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
	if err := api.Setup(); err != nil {
		logs.Error(err)
		outer, _ := logs.Wrapper(err)

		return false, errors.New(outer)
	}
	access, err := api.GetProfile()
	if err != nil {
		logs.Error(err)
		outer, _ := logs.Wrapper(err)

		return false, errors.New(outer)
	}
	if !access {
		logs.Errorf("Your session has expired")
	}

	reps := make(map[string]bool)
	enabled := api.GetRepositories()
	for _, r := range api.GetAllRepositories() {
		reps[r] = !slices.Contains(enabled, r)
	}
	wailsruntime.EventsEmit(a.ctx, "setRepositories", reps)

	/*
		if airlock.ExportPossible() {
			wailsruntime.EventsEmit(a.ctx, "exportPossible")
		}
	*/
	return access, nil
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

	a.ph.deleteProjects()
	filesystem.RefreshFilesystem()

	buckets := filesystem.GetNodeChildren(api.SDConnect + "/" + api.GetProjectName())
	if len(buckets) > 0 {
		wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
	}
	wailsruntime.EventsEmit(a.ctx, "fuseReady")
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

func (a *App) CheckExistence(file, bucket string) (found bool) {
	chld := filesystem.GetNodeChildren(api.SDConnect + "/" + api.GetProjectName() + "/" + bucket)

	return slices.Contains(chld, filepath.Base(file+".c4gh"))
}

func (a *App) ExportFile(_, _ string) error {
	time.Sleep(1000 * time.Millisecond)
	/*err := airlock.Upload(file, folder, 4000)
	if err != nil {
		logs.Error(err)
		message, _ := logs.Wrapper(err)

		return errors.New(message)
	}*/

	return nil
}
