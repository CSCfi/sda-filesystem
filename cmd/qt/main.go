package main

import (
	"errors"
	"io"
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

	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/filesystem"
	"sd-connect-fuse/internal/logs"
)

var projectModel = NewProjectModel(nil)

// QmlBridge is the link between QML and Go
type QmlBridge struct {
	core.QObject

	_ func()                                 `constructor:"init"`
	_ func(username, password string) string `slot:"sendLoginRequest"`
	_ func()                                 `slot:"loadFuse"`
	_ func()                                 `slot:"openFuse"`
	_ func(mount string)                     `slot:"changeMountPoint"`
	_ func()                                 `slot:"shutdown"`
	_ func(err error)                        `signal:"envError"`
	_ func()                                 `signal:"fuseReady"`

	_ *fuse.FileSystemHost `property:"fileSystemHost"`
	_ string               `property:"mountPoint"`
	_ gui.QFont            `property:"fixedFont"`
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

func (qb *QmlBridge) sendLoginRequest(username, password string) string {
	api.CreateToken(username, password)
	err := api.InitializeClient()
	if err != nil {
		logs.Error(err)
		text, _ := logs.Wrapper(err)
		return text
	}

	logs.Info("Retrieving projects in order to test login")
	projects, err := api.GetProjects()
	if err != nil {
		logs.Error(err)

		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			return "Incorrect username or password"
		}

		return "Failed to get a response from metadata API"
	}

	projectModel.ApiToProject(projects)
	return ""
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

func (qb *QmlBridge) changeMountPoint(mount string) {
	// Verify mount point directory
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			logs.Debug("Mount point", mount, "does not exist, so it will be created")
			if err = os.Mkdir(mount, 0777); err != nil {
				logs.Errorf("Could not create directory %s", mount)
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
			logs.Errorf("Mount point %s must be empty", mount)
		}
	}

	logs.Debug("Filesystem will be mounted at", mount)
	qb.SetMountPoint(mount)
}

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		//logs.Fatal("Could not find user home directory", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Projects")
	return p
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
	var logModel = NewLogModel(nil)
	app.RootContext().SetContextProperty("QmlBridge", qmlBridge)
	app.RootContext().SetContextProperty("ProjectModel", projectModel)
	app.RootContext().SetContextProperty("LogModel", logModel)

	app.AddImportPath("qrc:/qml/")
	app.Load(core.NewQUrl3("qrc:/qml/main/login.qml", 0))

	//fmt.Println(core.QThread_CurrentThread().Pointer())

	logs.SetSignal(logModel.AddLog)

	err := api.GetEnvs()
	if err != nil {
		logs.Error(err)
		qmlBridge.EnvError(err)
	}

	gui.QGuiApplication_Exec()
}
