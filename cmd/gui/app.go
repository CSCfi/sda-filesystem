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
	ctx        context.Context
	ph         *ProjectHandler
	fs         *filesystem.Fuse
	mountpoint string
}

// NewApp creates a new App application struct
func NewApp(ph *ProjectHandler) *App {
	return &App{ph: ph}
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

func (a *App) Panic() {

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
	for _, rep := range api.GetAllRepositories() {
		if err := api.GetEnvs(rep); err != nil {
			logs.Error(err)
		} else {
			noneAvailable = false
		}
	}

	if noneAvailable {
		return fmt.Errorf("No services available")
	}

	return nil
}

func (a *App) Login(username, password string) (bool, error) {
	success, err := api.ValidateLogin(username, password)
	if err != nil {
		logs.Error(err)
	}
	if !success {
		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			return false, nil
		} else {
			message, _ := logs.Wrapper(err)
			return false, fmt.Errorf(message)
		}
	}

	isManager, err := airlock.IsProjectManager()
	if err != nil {
		logs.Errorf("Resolving project manager status failed: %w", err)
	} else if !isManager {
		logs.Info("You are not the project manager")
	} else {
		logs.Info("You are the project manager")
		if err = airlock.GetPublicKey(); err != nil {
			logs.Error(err)
		} else {
			wailsruntime.EventsEmit(a.ctx, "isProjectManager")
		}
	}

	a.fs = filesystem.InitializeFileSystem(a.ph.AddProject)
	a.ph.sendProjects()
	logs.Info("Login successful")
	return true, nil
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
			wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
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
		return fmt.Errorf("You have files in use and thus updating is not possible")
	}
	logs.Info("Updating Data Gateway")
	time.Sleep(200 * time.Millisecond)

	newFs := filesystem.InitializeFileSystem(a.ph.AddProject)
	newFs.PopulateFilesystem(a.ph.trackContainers)
	a.fs.RefreshFilesystem(newFs)

	buckets := a.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName())
	wailsruntime.EventsEmit(a.ctx, "setBuckets", buckets)
	wailsruntime.EventsEmit(a.ctx, "fuseReady")

	return nil
}

func (a *App) CheckEncryption(file, bucket string) (string, string, bool) {
	if encrypted, err := airlock.CheckEncryption(file); err != nil {
		logs.Error(err)
		return "", "", false
	} else {
		chld := a.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName() + "/" + bucket)
		if encrypted {
			exists := slices.Contains(chld, filepath.Base(file))
			return "", file, exists
		} else {
			fileEncrypted := file + ".c4gh"
			exists := slices.Contains(chld, filepath.Base(fileEncrypted))
			return file, fileEncrypted, exists
		}
	}
}

func (a *App) ExportFile(folder, origFile, file string) string {
	time.Sleep(1000 * time.Millisecond)
	err := airlock.Upload(origFile, file, folder, "", 4000, origFile != "")
	if err != nil {
		logs.Error(err)
		return fmt.Sprintf("Exporting file %s failed", file)
	}
	return ""
}
