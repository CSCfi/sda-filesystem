package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"

	"github.com/billziss-gh/cgofuse/fuse"
	log "github.com/sirupsen/logrus"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/qml"
	"github.com/therecipe/qt/quickcontrols2"

	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/filesystem"
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
	_ func(err error)                        `signal:"envError"`
	_ func()                                 `signal:"fuseReady"`

	_ *fuse.FileSystemHost `property:"fileSystemHost"`
	_ string               `property:"mountPoint"`
}

func (qb *QmlBridge) init() {
	qb.SetMountPoint(mountPoint())

	qb.ConnectSendLoginRequest(qb.sendLoginRequest)
	qb.ConnectLoadFuse(qb.loadFuse)
	qb.ConnectOpenFuse(qb.openFuse)
	qb.ConnectChangeMountPoint(qb.changeMountPoint)
}

func (qb *QmlBridge) sendLoginRequest(username, password string) string {
	api.CreateToken(username, password)
	text, err := api.InitializeClient()
	if err != nil {
		log.Error(err)
		return text
	}

	log.Info("Retrieving projects in order to test login")
	projects, err := api.GetProjects()
	if err != nil {
		log.Error(err)

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
	go func() {
		connectfs := filesystem.CreateFileSystem()
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
		log.Error(err)
	}
}

func (qb *QmlBridge) changeMountPoint(mount string) {
	// Verify mount point directory
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			log.Debugf("Mount point %s does not exist, so it will be created", mount)
			if err = os.Mkdir(mount, 0777); err != nil {
				log.Fatalf("Could not create directory %s", mount)
			}
		}
	} else {
		// Mount directory must not already exist in Windows
		if runtime.GOOS == "windows" {
			log.Fatalf("Mount point %s already exists, remove the directory or use another mount point", mount)
		}
		// Check that the mount point is empty if it already exists
		dir, err := os.Open(mount)
		if err != nil {
			log.Fatalf("Could not open mount point %s", mount)
		}
		defer dir.Close()
		// Verify dir is empty
		if _, err = dir.Readdir(1); err != io.EOF {
			log.Fatalf("Mount point %s must be empty", mount)
		}
	}

	log.Debugf("Filesystem will be mounted at %s", mount)
	qb.SetMountPoint(mount)
}

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Could not find user home directory", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Projects")
	return p
}

func init() {
	// Configure Log Text Formatter
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	core.QCoreApplication_SetApplicationName("SD-Connect FUSE")
	core.QCoreApplication_SetAttribute(core.Qt__AA_EnableHighDpiScaling, true)

	gui.NewQGuiApplication(len(os.Args), os.Args)

	// Inbuild styles are:
	// Default, Material, Fusion, Imagine, Universal
	quickcontrols2.QQuickStyle_SetStyle("Material")

	var app = qml.NewQQmlApplicationEngine(nil)

	var qmlBridge = NewQmlBridge(nil)
	app.RootContext().SetContextProperty("QmlBridge", qmlBridge)
	app.RootContext().SetContextProperty("ProjectModel", projectModel)

	app.AddImportPath("qrc:/qml/")
	app.Load(core.NewQUrl3("qrc:/qml/main/login.qml", 0))

	err := api.GetEnvs()
	if err != nil {
		qmlBridge.EnvError(err)
	}

	gui.QGuiApplication_Exec()
}
