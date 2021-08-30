package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
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
	_ func(username, password string) `slot:"sendLoginRequest,auto"`
	_ func()                          `slot:"loadFuse,auto"`
	_ func()                          `slot:"openFuse,auto"`
	_ func(mount string) string       `slot:"changeMountPoint,auto"`
	_ func()                          `slot:"shutdown,auto"`
	_ func(message, err string)       `signal:"loginResult"`
	_ func(message, err string)       `signal:"envError"`
	_ func()                          `signal:"fuseReady"`
	_ func()                          `signal:"panic"`

	_ *fuse.FileSystemHost `property:"fileSystemHost"`
	_ string               `property:"mountPoint"`
	_ gui.QFont            `property:"fixedFont"`
}

func (qb *QmlBridge) init() {
	qb.SetMountPoint(mountPoint())
	qb.SetFixedFont(gui.QFontDatabase_SystemFont(gui.QFontDatabase__FixedFont))

	filesystem.SetSignalBridge(qb.Panic)
}

func (qb *QmlBridge) sendLoginRequest(username, password string) {
	go func() {
		api.CreateToken(username, password)
		err := api.InitializeClient()
		if err != nil {
			logs.Error(err)
			qb.LoginResult("Initializing HTTP client failed", strings.Join(logs.StructureError(err), "\n"))
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

			qb.LoginResult("Login failed", strings.Join(logs.StructureError(err), "\n"))
			return
		}

		projects, err := api.GetProjects(false)
		if err != nil {
			logs.Error(err)
			qb.LoginResult("Failed to retrieve projects", strings.Join(logs.StructureError(err), "\n"))
			return
		}
		if len(projects) == 0 {
			qb.LoginResult("No project permissions found", "")
			return
		}
		projectModel.ApiToProject(projects)

		for i := range projects {
			err = api.GetSToken(projects[i].Name)
			if err != nil {
				logs.Warning(err)
			}
		}

		logs.Info("Login successful")
		api.SetLoggedIn()
		qb.LoginResult("", "")
	}()
}

func (qb *QmlBridge) loadFuse() {
	sendToModel := make(chan filesystem.LoadProjectInfo)
	go projectModel.waitForInfo(sendToModel)
	go func() {
		defer func() {
			// recover from panic if one occured.
			if err := recover(); err != nil {
				logs.Error(fmt.Errorf("Something went wrong when creating filesystem: %w",
					fmt.Errorf("%v\n\n%s", err, string(debug.Stack()))))
				// Send alert
				qb.Panic()
			}
		}()
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
		// In case program is terminated with a ctrl+c
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()
}

func (qb *QmlBridge) openFuse() {
	var command string

	_, err := os.Stat(qb.MountPoint())
	if err != nil {
		logs.Errorf("Failed to find directory: %w", err)
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
	cmd := exec.Command(command, qb.MountPoint())
	err = cmd.Run()
	if err != nil {
		logs.Errorf("Could not open directory %s: %w", qb.MountPoint(), err)
	}
}

func (qb *QmlBridge) shutdown() {
	logs.Info("Shutting down SD-Connect Filesystem")
	// Sending interrupt signal to unmount fuse
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (qb *QmlBridge) changeMountPoint(url string) string {
	mount := core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())
	logs.Debug("Trying to change mount point to ", mount)

	// Verify mount point directory
	if dir, err := os.Stat(mount); os.IsNotExist(err) {
		err = createDir(mount)
		if err != nil {
			logs.Error(err)
			return err.Error()
		}
	} else {
		if !dir.IsDir() {
			str := fmt.Sprintf("%s is not a directory", dir.Name())
			logs.Errorf(str)
			return str
		}

		// Mount directory must not already exist in Windows
		if runtime.GOOS == "windows" { // ?
			str := fmt.Sprintf("Mount point %s already exists, remove the directory or use another mount point", mount)
			logs.Errorf(str)
			return str
		}

		if unix.Access(mount, unix.W_OK) != nil { // What about windows?
			str := fmt.Sprintf("You do not have permission to write to folder %s", mount)
			logs.Errorf(str)
			return str
		}

		// Check that the mount point is empty if it already exists
		if ok, err := isEmpty(mount); err != nil {
			logs.Error(err)
			return err.Error()
		} else if !ok {
			str := fmt.Sprintf("Mount point %s must be empty", mount)
			logs.Errorf(str)
			return str
		}
	}

	logs.Debug("Filesystem will be mounted at ", mount)
	qb.SetMountPoint(mount)
	return ""
}

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logs.Errorf("Could not find user home directory: %w", err)
		return ""
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Projects")

	if _, err := os.Stat(p); os.IsNotExist(err) {
		err = createDir(p)
		if err != nil {
			logs.Error(err)
			return ""
		}
		return p
	}

	if unix.Access(p, unix.W_OK) != nil {
		return ""
	}

	if ok, err := isEmpty(p); err != nil {
		logs.Error(err)
	} else if ok {
		return p
	}

	return ""
}

func createDir(dir string) error {
	// In other OSs except Windows, the mount point must exist and be empty
	if runtime.GOOS != "windows" { // ?
		logs.Debugf("Directory %s does not exist, so it will be created", dir)
		if err := os.Mkdir(dir, 0755); err != nil {
			return fmt.Errorf("Could not create directory %s: %w", dir, err)
		}
		logs.Debugf("Directory %s created", dir)
	}
	return nil
}

func isEmpty(mount string) (bool, error) {
	dir, err := os.Open(mount)
	if err != nil {
		return false, fmt.Errorf("Could not open mount point %s: %w", mount, err)
	}
	defer dir.Close()
	// Verify dir is empty
	if _, err = dir.Readdir(1); err != io.EOF {
		if err != nil {
			return false, fmt.Errorf("Error occurred when reading from directory %s: %w", mount, err)
		}
		return false, nil
	}
	return true, nil
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
	filesystem.SetSignalModel(projectModel.UpdateNoStorage)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	core.QCoreApplication_SetApplicationName("SD-Connect Filesystem")
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

	var logLevel = NewLogLevel(nil)
	app.RootContext().SetContextProperty("LogLevel", logLevel)

	app.AddImportPath("qrc:/qml/")
	app.Load(core.NewQUrl3("qrc:/qml/main/login.qml", 0))

	//fmt.Println(core.QThread_CurrentThread().Pointer())

	err := api.GetEnvs()
	if err != nil {
		logs.Error(err)
		qmlBridge.EnvError("Environment variables not valid", strings.Join(logs.StructureError(err), "\n"))
	}

	gui.QGuiApplication_Exec()
}
