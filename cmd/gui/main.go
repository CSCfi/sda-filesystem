package main

import (
	"context"
	wailsbuild "sda-filesystem/build"
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
		OnBeforeClose: app.beforeClose,
		Bind: []any{
			app,
			logHandler,
			projectHandler,
		},
		MinWidth: 800,
		Width:    800,
		Height:   575,
		Linux: &linux.Options{
			Icon:             wailsbuild.Icon,
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
