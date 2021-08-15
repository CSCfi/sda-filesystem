package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/qml"
	"github.com/therecipe/qt/quickcontrols2"
	"golang.org/x/sys/unix"

	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/filesystem"
	"sd-connect-fuse/internal/logs"
)

var projectModel = NewProjectModel(nil)
var logModel = NewLogModel(nil)

// QmlBridge is the link between QML and Go
type QmlBridge struct {
	core.QObject

	_ func()                          `constructor:"init"`
	_ func(username, password string) `slot:"sendLoginRequest"`
	_ func()                          `slot:"loadFuse"`
	_ func()                          `slot:"openFuse"`
	_ func(mount string) string       `slot:"changeMountPoint"`
	_ func()                          `slot:"shutdown"`
	_ func(message, err string)       `signal:"loginResult"`
	_ func(err error)                 `signal:"envError"`
	_ func(err error)                 `signal:"mountError"`
	_ func()                          `signal:"fuseReady"`

	_ *fuse.FileSystemHost `property:"fileSystemHost"`
	_ string               `property:"mountPoint"`
	_ gui.QFont            `property:"fixedFont"`
	_ bool                 `property:"isWindows"`
}

func (qb *QmlBridge) init() {
	qb.SetMountPoint(mountPoint())
	qb.SetFixedFont(gui.QFontDatabase_SystemFont(gui.QFontDatabase__FixedFont))

	qb.ConnectSendLoginRequest(qb.sendLoginRequest)
	qb.ConnectLoadFuse(qb.loadFuse)
	qb.ConnectOpenFuse(qb.openFuse)
	qb.ConnectChangeMountPoint(qb.changeMountPoint)
	qb.ConnectShutdown(qb.shutdown)
}

func (qb *QmlBridge) sendLoginRequest(username, password string) {
	go func() {
		api.CreateToken(username, password)
		err := api.InitializeClient()
		if err != nil {
			logs.Error(err)
			qb.LoginResult("Initializing HTTP client failed", logs.StructureError(err))
			return
		}

		err = api.GetUToken()
		if err != nil {
			logs.Error(err)

			var re *api.RequestError
			if errors.As(err, &re) && re.StatusCode == 401 {
				qb.LoginResult("Incorrect username or password", "")
				return
			}

			qb.LoginResult("Login failed. Could not retrieve unscoped token", logs.StructureError(err))
			return
		}

		projects, err := api.GetProjects(false)
		if err != nil {
			logs.Error(err)
			qb.LoginResult("Failed to retrieve projects", logs.StructureError(err))
			return
		}
		if len(projects) == 0 {
			qb.LoginResult("No project permissions found", "")
			return
		}
		projectModel.ApiToProject(projects)

		err = api.GetSTokens(projects)
		if err != nil {
			logs.Error(err)
			qb.LoginResult("Failed to retrieve scoped tokens", logs.StructureError(err))
			return
		}

		qb.LoginResult("", "")
	}()
}

func (qb *QmlBridge) loadFuse() {
	sendToModel := make(chan filesystem.LoadProjectInfo)
	go projectModel.waitForInfo(sendToModel)
	go func() {
		connectfs := filesystem.CreateFileSystem(sendToModel)
		host := fuse.NewFileSystemHost(connectfs)
		qb.SetFileSystemHost(host)
		options := []string{}
		if runtime.GOOS == "darwin" {
			options = append(options, "-o", "defer_permissions")
			options = append(options, "-o", "volname="+path.Base(qb.MountPoint()))
		}

		qb.FuseReady()
		host.Mount(qb.MountPoint(), options)
	}()
}

func (qb *QmlBridge) openFuse() {
	cmd := exec.Command("open", qb.MountPoint())
	err := cmd.Run()
	if err != nil {
		logs.Error(err)
	}
}

func (qb *QmlBridge) shutdown() {
	logs.Info("Shutting down SD-Connect FUSE")
	// Sending interrupt signal to unmount fuse
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (qb *QmlBridge) changeMountPoint(url string) string {
	mount := core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())

	//TODO: check permissions?
	fmt.Println(unix.Access(mount, unix.W_OK), os.Getuid())

	dir, err := os.Open(mount)
	if err != nil {
		logs.Errorf("Could not open mount point %s: %w", mount, err)
		return "Could not open mount point " + mount + ". Check logs for further details."
	}
	dir.Close()

	logs.Debug("Filesystem will be mounted at ", mount)
	qb.SetMountPoint(mount)
	return ""

	/*// Verify mount point directory
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			logs.Debugf("Mount point %s does not exist, so it will be created", mount)
			if err = os.Mkdir(mount, 0777); err != nil {
				str := fmt.Sprintf("Could not create directory %s: ", mount)
				logs.Errorf("%s: %w", str, err)
				return
			}
		}
	} else {
		// Mount directory must not already exist in Windows
		if runtime.GOOS == "windows" {
			logs.Errorf("Mount point %s already exists, remove the directory or use another mount point", mount)
		}
		// Check that the mount point is empty if it already exists
		dir, err := os.Open(mount)
		if err != nil {
			logs.Errorf("Could not open mount point %s", mount)
		}
		defer dir.Close()
		// Verify dir is empty
		if _, err = dir.Readdir(1); err != io.EOF {
			str := fmt.Sprintf("Mount point %s must be empty", mount)
			if err == nil {
				logs.Errorf("%s", str)
			} else {
				logs.Errorf("%s: %w", str, err)
			}
			return str
		}
	}*/
}

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logs.Errorf("Could not find user home directory: %w", err)
		return ""
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Projects")
	return p
}

func init() {
	logs.SetSignal(logModel.AddLog)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	core.QCoreApplication_SetApplicationName("SD-Connect FUSE")
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

	app.AddImportPath("qrc:/qml/")
	app.Load(core.NewQUrl3("qrc:/qml/main/login.qml", 0))

	//fmt.Println(core.QThread_CurrentThread().Pointer())

	err := api.GetEnvs()
	if err != nil {
		logs.Error(err)
		qmlBridge.EnvError(err)
	}

	gui.QGuiApplication_Exec()
}
