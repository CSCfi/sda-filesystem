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

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"
)

var projectModel = NewProjectModel(nil)
var logModel = NewLogModel(nil)
var loginModel = NewLoginModel(nil)

// QmlBridge is the link between QML and Go
type QmlBridge struct {
	core.QObject

	_ func() `constructor:"init"`

	_ func(int)                     `slot:"loginWithToken,auto"`
	_ func(int, string, string)     `slot:"loginWithPassword,auto"`
	_ func()                        `slot:"initFuse,auto"`
	_ func()                        `slot:"loadFuse,auto"`
	_ func()                        `slot:"openFuse,auto"`
	_ func(string) string           `slot:"changeMountPoint,auto"`
	_ func()                        `slot:"shutdown,auto"`
	_ func(idx int)                 `signal:"login401"`
	_ func(idx int, message string) `signal:"loginError"`
	_ func(message string)          `signal:"initError"`
	_ func()                        `signal:"fuseReady"`
	_ func()                        `signal:"panic"`

	_ string `property:"mountPoint"`

	fs *filesystem.Fuse
}

func (qb *QmlBridge) init() {
	mount, err := mountpoint.DefaultMountPoint()
	qb.SetMountPoint(mount)
	if err != nil {
		logs.Warning(err)
	}

	filesystem.SetSignalBridge(qb.Panic)
}

func (qb *QmlBridge) initializeAPI() {
	err := api.InitializeCache()
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
}

func (qb *QmlBridge) loginWithToken(idx int) {
	go qb.login(idx)
}

func (qb *QmlBridge) loginWithPassword(idx int, username, password string) {
	go qb.login(idx, username, password)
}

func (qb *QmlBridge) login(idx int, auth ...string) {
	rep := loginModel.getRepository(idx)
	if err := api.AddRepository(rep); err != nil {
		logs.Error(err)
		qb.LoginError(idx, "Environment variables are not valid")
		return
	}

	if err := api.ValidateLogin(rep, auth...); err != nil {
		logs.Error(err)

		var re *api.RequestError
		if errors.As(err, &re) && (re.StatusCode == 401 || re.StatusCode == 404) {
			qb.Login401(idx)
			return
		}

		qb.LoginError(idx, fmt.Sprintf("%s authentication failed", rep))
		return
	}

	loginModel.setLoggedIn(idx, true)
	logs.Info(rep, " login successful")
}

func (qb *QmlBridge) initFuse() {
	qb.fs = filesystem.InitializeFileSystem(projectModel.AddProject)
}

func (qb *QmlBridge) loadFuse() {
	go func() {
		defer filesystem.CheckPanic()
		qb.fs.PopulateFilesystem(projectModel.AddToCount)

		go func() {
			time.Sleep(time.Second)
			qb.FuseReady()
		}()

		filesystem.MountFilesystem(qb.fs, qb.MountPoint())
		os.Exit(0)
	}()
}

func (qb *QmlBridge) openFuse() {
	var command string

	_, err := os.Stat(qb.MountPoint())
	if err != nil {
		logs.Errorf("Failed to find directory %q: %w", qb.MountPoint(), err)
		return
	}
	switch runtime.GOOS {
	case "darwin":
		command = "open"
	case "linux":
		command = "xdg-open"
	case "windows":
		command = "start"
	default:
		logs.Errorf("Unrecognized OS")
		return
	}

	userPath := qb.MountPoint()
	cmd := exec.Command(command, userPath)
	err = cmd.Run()
	if err != nil {
		logs.Errorf("Could not open directory %s: %w", userPath, err)
	}
}

func (qb *QmlBridge) shutdown() {
	logs.Info("Shutting down SDA Filesystem")
	filesystem.UnmountFilesystem()
}

func (qb *QmlBridge) changeMountPoint(url string) string {
	mount := core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())
	mount = filepath.Clean(mount)
	logs.Debugf("Trying to change mount point to %s", mount)

	if err := mountpoint.CheckMountPoint(mount); err != nil {
		logs.Error(err)
		return err.Error()
	}

	logs.Infof("Filesystem will be mounted at %s", mount)
	qb.SetMountPoint(mount)
	return ""
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
}

func main() {
	core.QCoreApplication_SetApplicationName("SDA Filesystem")
	core.QCoreApplication_SetOrganizationName("CSC")
	core.QCoreApplication_SetOrganizationDomain("csc.fi")
	core.QCoreApplication_SetApplicationVersion("1.0.0")
	core.QCoreApplication_SetAttribute(core.Qt__AA_EnableHighDpiScaling, true)

	gui.NewQGuiApplication(len(os.Args), os.Args)

	var font = gui.NewQFont2("Helvetica", -1, -1, false)
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
	app.RootContext().SetContextProperty("LoginModel", loginModel)

	app.RootContext().SetContextProperty("LogLevel", NewLogLevel(nil))
	app.RootContext().SetContextProperty("LoginMethod", NewLoginMethod(nil))

	app.AddImportPath("qrc:/qml/")
	app.Load(core.NewQUrl3("qrc:/qml/main/main.qml", 0))

	//fmt.Println(core.QThread_CurrentThread().Pointer())

	qmlBridge.initializeAPI()
	gui.QGuiApplication_Exec()
}
