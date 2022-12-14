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
	fs         *filesystem.Fuse
	mountPoint string
}

type Project struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
	//Containers string `json:"containers"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	var err error
	a.mountPoint, err = mountpoint.DefaultMountPoint()
	if err != nil {
		logs.Warning(err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	filesystem.UnmountFilesystem()
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
	//qb.SetIsProjectManager(isManager)
	if err != nil {
		logs.Errorf("Resolving project manager status failed: %w", err)
		//qb.PreventExport()
	} else if isManager {
		logs.Info("You are the project manager")
	} else {
		logs.Info("You are not the project manager")
	}

	if err = airlock.GetPublicKey(); err != nil {
		logs.Error(err)
		//qb.PreventExport()
	}

	a.fs = filesystem.InitializeFileSystem(a.AddProject)
	logs.Info("Login successful")
	return true, nil
}

func (a *App) AddProject(rep, pr string) {
	/*if idx, ok := pm.nameToIndex[rep+"/"+pr]; ok {
		delete(pm.deletedIdxs, idx)
		return
	}*/

	wailsruntime.EventsEmit(a.ctx, "newLogEntry", Project{Name: pr, Repository: rep})
}

func (a *App) LoadFuse() {
	go func() {
		defer filesystem.CheckPanic()
		//a.fs.PopulateFilesystem(projectModel.AddToCount)

		go func() {
			time.Sleep(time.Second)
			//a.SetBuckets(qb.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName()))
			wailsruntime.EventsEmit(a.ctx, "fuseReady")
		}()

		filesystem.MountFilesystem(a.fs, a.mountPoint)
		os.Exit(0)
	}()
}

func (a *App) OpenFuse() {
	var cmd *exec.Cmd
	userPath := a.mountPoint

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

func (a *App) RefreshFuse() string {
	if a.fs.FilesOpen(a.mountPoint) {
		return "You have files in use and thus updating is not possible"
	}
	logs.Info("Updating Data Gateway")
	//projectModel.PrepareForRefresh()
	time.Sleep(200 * time.Millisecond)
	//newFs := filesystem.InitializeFileSystem(projectModel.AddProject)
	//projectModel.DeleteExtraProjects()
	//newFs.PopulateFilesystem(projectModel.AddToCount)
	//qb.fs.RefreshFilesystem(newFs)
	//qb.SetBuckets(qb.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName()))
	wailsruntime.EventsEmit(a.ctx, "fuseReady")
	return ""
}

func (a *App) CheckEncryption(url, bucket string) (string, string, bool) {
	file := "" // core.NewQUrl3(url, 0).ToLocalFile()

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

func (a *App) ChangeMountPoint(url string) string {
	mount := "" //core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())
	mount = filepath.Clean(mount)
	logs.Debugf("Trying to change mount point to %s", mount)

	if err := mountpoint.CheckMountPoint(mount); err != nil {
		logs.Error(err)
		return err.Error()
	}

	logs.Infof("Data Gateway will be mounted at %s", mount)
	a.mountPoint = mount
	return ""
}
