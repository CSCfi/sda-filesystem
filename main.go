package main

import (
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

	_ func()                               `constructor:"init"`
	_ func(username, password string) bool `slot:"sendLoginRequest"`
}

func (qb *QmlBridge) init() {
	log.Info("qb init")
	qb.ConnectSendLoginRequest(func(username, password string) bool {
		api.CreateToken(username, password)
		err := api.InitializeClient()
		if err != nil {
			log.Fatal(err)
		}
		log.Info("Retrieving projects in order to test login")
		_, err = api.GetProjects()
		if err != nil {
			log.Error(err)
			return false
		}
		return true
	})
}

func init() {
	log.Info("main init")
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
	app.AddImportPath("qrc:/qml/") // Do I need three slashes?
	app.Load(core.NewQUrl3("qrc:/qml/main/login.qml", 0))

	var qmlBridge = NewQmlBridge(nil)
	app.RootContext().SetContextProperty("qmlBridge", qmlBridge)

	gui.QGuiApplication_Exec()
}
