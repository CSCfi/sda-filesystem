package main

import (
	"context"
	"sda-filesystem/frontend"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	// Create an instance of the app structure
	app := NewApp()
	logHandler := NewLogHandler()

	// Create application with options
	err := wails.Run(&options.App{
		Title: "Data Gateway",
		AssetServer: &assetserver.Options{
			Assets: frontend.Assets,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			logHandler.SetContext(ctx)
		},
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
			logHandler,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}