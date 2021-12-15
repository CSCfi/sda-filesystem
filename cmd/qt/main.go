package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"syscall"
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

	_ func()                    `slot:"login,auto"`
	_ func()                    `slot:"loadFuse,auto"`
	_ func()                    `slot:"openFuse,auto"`
	_ func(string) string       `slot:"changeMountPoint,auto"`
	_ func()                    `slot:"shutdown,auto"`
	_ func(message, err string) `signal:"loginResult"`
	_ func()                    `signal:"fuseReady"`
	_ func()                    `signal:"panic"`

	_ string    `property:"mountPoint"`
	_ gui.QFont `property:"fixedFont"`

	fs *filesystem.Fuse
}

func (qb *QmlBridge) init() {
	mount, err := mountpoint.DefaultMountPoint()
	qb.SetMountPoint(mount)
	if err != nil {
		logs.Warning(err)
	}

	qb.SetFixedFont(gui.QFontDatabase_SystemFont(gui.QFontDatabase__FixedFont))
	filesystem.SetSignalBridge(qb.Panic)
}

func (qb *QmlBridge) login() {
	go func() {
		api.RemoveAll()
		auth := loginModel.getAuth()
		for rep := range auth {
			api.AddRepository(rep)
		}

		err := api.GetEnvs()
		if err != nil {
			logs.Error(err)
			qb.LoginResult("Environment variables not valid", strings.Join(logs.StructureError(err), "\n"))
			return
		}

		err = api.InitializeCache()
		if err != nil {
			logs.Error(err)
			outer, inner := logs.Wrapper(err)
			qb.LoginResult(outer, strings.Join(logs.StructureError(inner), "\n"))
			return
		}

		err = api.InitializeClient()
		if err != nil {
			logs.Error(err)
			qb.LoginResult("Initializing HTTP client failed", strings.Join(logs.StructureError(err), "\n"))
			return
		}

		for rep := range auth {
			if err := api.ValidateLogin(rep, auth[rep]...); err != nil {
				logs.Error(err)

				var re *api.RequestError
				if errors.As(err, &re) && (re.StatusCode == 401 || re.StatusCode == 404) {
					qb.LoginResult(fmt.Sprintf("Incorrect %s username, password or token", rep), "")
					return
				}

				qb.LoginResult(fmt.Sprintf("%s authentication failed", rep), strings.Join(logs.StructureError(err), "\n"))
				return
			}
		}

		sendToModel := make(chan filesystem.LoadProjectInfo)
		go func() {
			projectModel.addProjects(sendToModel)

			if len(projectModel.projects) == 0 {
				qb.LoginResult("No permissions found", "")
				return
			}

			logs.Info("Login successful")
			qb.LoginResult("", "")
		}()
		qb.fs = filesystem.InitializeFileSystem(sendToModel)
	}()
}

func (qb *QmlBridge) loadFuse() {
	sendToModel := make(chan filesystem.LoadProjectInfo)
	go projectModel.waitForInfo(sendToModel)
	go func() {
		defer filesystem.CheckPanic()
		qb.fs.PopulateFilesystem(sendToModel)

		go func() {
			time.Sleep(time.Second)
			qb.FuseReady()
		}()

		filesystem.MountFilesystem(qb.fs, qb.MountPoint())

		// In case program is terminated with a ctrl+c
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
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
	// this is to address gosec204, we clean to return the shortest path name
	// that returns string which seems to satisfy the gosec204
	userPath := path.Clean(qb.MountPoint())

	cmd := exec.Command(command, userPath)
	err = cmd.Run()
	if err != nil {
		logs.Errorf("Could not open directory %s: %w", userPath, err)
	}
}

func (qb *QmlBridge) shutdown() {
	logs.Info("Shutting down SDA Filesystem")
	// Sending interrupt signal to unmount fuse
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (qb *QmlBridge) changeMountPoint(url string) string {
	mount := core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())
	logs.Debug("Trying to change mount point to ", mount)

	if err := mountpoint.CheckMountPoint(mount); err != nil {
		logs.Error(err)
		return err.Error()
	}

	logs.Debug("Filesystem will be mounted at ", mount)
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
	app.Load(core.NewQUrl3("qrc:/qml/main/login.qml", 0))

	//fmt.Println(core.QThread_CurrentThread().Pointer())

	gui.QGuiApplication_Exec()
}
