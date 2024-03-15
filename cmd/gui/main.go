package main

import (
	"context"
	"sda-filesystem/build"
	"sda-filesystem/frontend"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

func main() {
	// Create an instance of the app structure
	projectHandler := NewProjectHandler()
	logHandler := NewLogHandler()
	app := NewApp(projectHandler, logHandler)

	// Create application with options
	err := wails.Run(&options.App{
		Title: "Data Gateway",
		AssetServer: &assetserver.Options{
			Assets: frontend.Assets,
		},
		OnStartup: func(ctx context.Context) {
			logHandler.SetContext(ctx)
			projectHandler.SetContext(ctx)
			app.startup(ctx)
		},
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
			logHandler,
			projectHandler,
		},
		MinWidth: 800,
		Width:    800,
		Height:   575,
		Linux: &linux.Options{
			Icon:             build.Icon,
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
