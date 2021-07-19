package main

import (
	"errors"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/qml"
	"github.com/therecipe/qt/quickcontrols2"

	"sd-connect-fuse/internal/api"
)

// TODO: Think about logs. Should I use go or qt?

// QmlBridge is the link between QML and Go
type QmlBridge struct {
	core.QObject

	_ func()                                 `constructor:"init"`
	_ func(username, password string) string `slot:"sendLoginRequest"`
	_ func(err error)                        `signal:"envError"`
}

func (qb *QmlBridge) init() {
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

			return "HTTP failed to get a response from metadata API"
		}

		return ""
	})
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
