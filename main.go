package main

import (
	"errors"
	"os"
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

const dirName = "Projects"

// TODO: Think about logs. Should I use go or qt?

// QmlBridge is the link between QML and Go
type QmlBridge struct {
	core.QObject

	_ func()                                 `constructor:"init"`
	_ func(username, password string) string `slot:"sendLoginRequest"`
	_ func()                                 `slot:"loadFuse"`
	_ func()                                 `slot:"openFuse"`
	_ func(err error)                        `signal:"envError"`

	_ *fuse.FileSystemHost `property:"fileSystemHost"`
	_ string               `property:"mountPoint"`
}

func (qb *QmlBridge) init() {
	qb.SetMountPoint(mountPoint())

	qb.ConnectSendLoginRequest(func(username, password string) string {
		api.CreateToken(username, password)
		text, err := api.InitializeClient()
		if err != nil {
			log.Error(err)
			return text
		}

		log.Info("Retrieving projects in order to test login")
		_, err = api.GetProjects()
		if err != nil {
			log.Error(err)

			var re *api.RequestError
			if errors.As(err, &re) && re.StatusCode == 401 {
				return "Incorrect username or password"
			}

			return "Failed to get a response from metadata API"
		}

		return ""
	})

	qb.ConnectLoadFuse(func() {
		mount := mountPoint()
		connectfs := filesystem.CreateFileSystem()
		host := fuse.NewFileSystemHost(connectfs)
		qb.SetFileSystemHost(host)
		options := []string{}
		if runtime.GOOS == "darwin" {
			options = append(options, "-o", "defer_permissions")
			options = append(options, "-o", "volname="+dirName)
		}
		host.Mount(mount, options)
	})
}

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Could not find user home directory", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/" + dirName)
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
	app.RootContext().SetContextProperty("qmlBridge", qmlBridge)

	app.AddImportPath("qrc:/qml/") // Do I need three slashes?
	app.Load(core.NewQUrl3("qrc:/qml/main/login.qml", 0))

	err := api.GetEnvs()
	if err != nil {
		qmlBridge.EnvError(err)
	}

	gui.QGuiApplication_Exec()
}
