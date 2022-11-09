package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/qml"
	"github.com/therecipe/qt/quickcontrols2"
	"golang.org/x/exp/slices"

	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"
)

var projectModel = NewProjectModel(nil)
var logModel = NewLogModel(nil)

// QmlBridge is the link between QML and Go
type QmlBridge struct {
	core.QObject

	_ func() `constructor:"init"`

	_ func(string, string)                        `slot:"login,auto"`
	_ func()                                      `slot:"loadFuse,auto"`
	_ func()                                      `slot:"openFuse,auto"`
	_ func() string                               `slot:"refreshFuse,auto"`
	_ func(string) bool                           `slot:"isFile,auto"`
	_ func(string, string)                        `slot:"checkEncryption,auto"`
	_ func(string, string, string)                `slot:"exportFile,auto"`
	_ func(string) string                         `slot:"changeMountPoint,auto"`
	_ func()                                      `slot:"shutdown,auto"`
	_ func()                                      `signal:"loginFail"`
	_ func(message string)                        `signal:"popupError"`
	_ func(message string)                        `signal:"initError"`
	_ func()                                      `signal:"fuseReady"`
	_ func(fileOrig, fileEnc string, exists bool) `signal:"encryptionChecked"`
	_ func()                                      `signal:"preventExport"`
	_ func(success bool)                          `signal:"exportFinished"`
	_ func()                                      `signal:"panic"`

	_ []string `property:"buckets"`
	_ string   `property:"mountPoint"`
	_ bool     `property:"loggedIn"`
	_ bool     `property:"isProjectManager"`

	fs *filesystem.Fuse
}

func (qb *QmlBridge) init() {
	mount, err := mountpoint.DefaultMountPoint()
	qb.SetMountPoint(mount)
	if err != nil {
		logs.Warning(err)
	}

	qb.SetLoggedIn(false)
	filesystem.SetSignalBridge(qb.Panic)
}

func (qb *QmlBridge) initializeAPI() {
	err := api.GetCommonEnvs()
	if err != nil {
		logs.Error(err)
		qb.InitError("Required environmental varibles missing")
		return
	}

	err = api.InitializeCache()
	if err != nil {
		logs.Error(err)
		qb.InitError("Initializing cache failed")
		return
	}

	err = api.InitializeClient()
	if err != nil {
		logs.Error(err)
		qb.InitError("Initializing HTTP client failed")
		return
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
		qb.InitError("No services available")
	}
}

func (qb *QmlBridge) login(username, password string) {
	go func() {
		success, err := api.ValidateLogin(username, password)
		if err != nil {
			logs.Error(err)
		}
		if !success {
			var re *api.RequestError
			if errors.As(err, &re) && re.StatusCode == 401 {
				qb.LoginFail()
			} else {
				message, _ := logs.Wrapper(err)
				qb.PopupError(message)
			}
			return
		}

		isManager, err := airlock.IsProjectManager()
		qb.SetIsProjectManager(isManager)
		if err != nil {
			logs.Errorf("Resolving project manager status failed: %w", err)
			qb.PreventExport()
		} else if isManager {
			logs.Info("You are the project manager")
		} else {
			logs.Info("You are not the project manager")
		}

		if err = airlock.GetPublicKey(); err != nil {
			logs.Error(err)
			qb.PreventExport()
		}

		qb.fs = filesystem.InitializeFileSystem(projectModel.AddProject)
		logs.Info("Login successful")
		qb.SetLoggedIn(true)
	}()
}

func (qb *QmlBridge) loadFuse() {
	go func() {
		defer filesystem.CheckPanic()
		qb.fs.PopulateFilesystem(projectModel.AddToCount)

		go func() {
			time.Sleep(time.Second)
			qb.SetBuckets(qb.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName()))
			qb.FuseReady()
		}()

		filesystem.MountFilesystem(qb.fs, qb.MountPoint())
		os.Exit(0)
	}()
}

func (qb *QmlBridge) openFuse() {
	var cmd *exec.Cmd
	userPath := qb.MountPoint()

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

func (qb *QmlBridge) refreshFuse() string {
	if qb.fs.FilesOpen(qb.MountPoint()) {
		return "You have files in use and thus updating is not possible"
	}
	go func() {
		logs.Info("Updating Data Gateway")
		projectModel.PrepareForRefresh()
		time.Sleep(200 * time.Millisecond)
		newFs := filesystem.InitializeFileSystem(projectModel.AddProject)
		projectModel.DeleteExtraProjects()
		newFs.PopulateFilesystem(projectModel.AddToCount)
		qb.fs.RefreshFilesystem(newFs)
		qb.SetBuckets(qb.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName()))
		qb.FuseReady()
	}()
	return ""
}

func (qb *QmlBridge) isFile(url string) bool {
	return core.NewQUrl3(url, 0).IsLocalFile()
}

func (qb *QmlBridge) checkEncryption(url, bucket string) {
	file := core.NewQUrl3(url, 0).ToLocalFile()

	if encrypted, err := airlock.CheckEncryption(file); err != nil {
		logs.Error(err)
		qb.EncryptionChecked("", "", false)
	} else {
		chld := qb.fs.GetNodeChildren(api.SDConnect + "/" + airlock.GetProjectName() + "/" + bucket)
		if encrypted {
			exists := slices.Contains(chld, filepath.Base(file))
			qb.EncryptionChecked("", file, exists)
		} else {
			fileEncrypted := file + ".c4gh"
			exists := slices.Contains(chld, filepath.Base(fileEncrypted))
			qb.EncryptionChecked(file, fileEncrypted, exists)
		}
	}
}

func (qb *QmlBridge) exportFile(folder, origFile, file string) {
	go func() {
		time.Sleep(1000 * time.Millisecond)
		err := airlock.Upload(origFile, file, folder, "", 4000, origFile != "")
		if err != nil {
			logs.Error(err)
			qb.PopupError(fmt.Sprintf("Exporting file %s failed", file))
		}
		qb.ExportFinished(err == nil)
	}()
}

func (qb *QmlBridge) changeMountPoint(url string) string {
	mount := core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())
	mount = filepath.Clean(mount)
	logs.Debugf("Trying to change mount point to %s", mount)

	if err := mountpoint.CheckMountPoint(mount); err != nil {
		logs.Error(err)
		return err.Error()
	}

	logs.Infof("Data Gateway will be mounted at %s", mount)
	qb.SetMountPoint(mount)
	return ""
}

func (qb *QmlBridge) shutdown() {
	logs.Info("Shutting down Data Gateway")
	filesystem.UnmountFilesystem()
}

func init() {
	debug := flag.Bool("debug", false, "print debug logs")
	flag.Parse()

	if *debug {
		logs.SetLevel("debug")
	} else {
		logs.SetLevel("info")
	}

	logs.SetSignal(logModel.AddLog)
	logModel.SetIncludeDebug(*debug)
}

func main() {
	core.QCoreApplication_SetApplicationName("Data Gateway")
	core.QCoreApplication_SetOrganizationName("CSC")
	core.QCoreApplication_SetOrganizationDomain("csc.fi")
	core.QCoreApplication_SetApplicationVersion("1.3.0")
	core.QCoreApplication_SetAttribute(core.Qt__AA_EnableHighDpiScaling, true)

	gui.NewQGuiApplication(len(os.Args), os.Args)

	var font = gui.NewQFont2("Helvetica", 12, -1, false)
	font.SetStyleHint(gui.QFont__SansSerif, gui.QFont__PreferDefault)
	gui.QGuiApplication_SetFont(font)

	// Inbuilt styles are:
	// Default, Material, Fusion, Imagine, Universal
	quickcontrols2.QQuickStyle_SetStyle("Material")

	var app = qml.NewQQmlApplicationEngine(nil)

	var qmlBridge = NewQmlBridge(nil)
	app.RootContext().SetContextProperty("QmlBridge", qmlBridge)
	app.RootContext().SetContextProperty("ProjectModel", projectModel)
	app.RootContext().SetContextProperty("LogModel", logModel)

	app.RootContext().SetContextProperty("LogLevel", NewLogLevel(nil))

	app.AddImportPath("qrc:/qml/")
	app.Load(core.NewQUrl3("qrc:/qml/main/main.qml", 0))

	//fmt.Println(core.QThread_CurrentThread().Pointer())

	qmlBridge.initializeAPI()
	gui.QGuiApplication_Exec()
}
