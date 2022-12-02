package main

import (
	"context"
	"fmt"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	filesystem.UnmountFilesystem()
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) InitializeAPI() string {
	err := api.GetCommonEnvs()
	if err != nil {
		logs.Error(err)
		return "Required environmental varibles missing"
	}

	err = api.InitializeCache()
	if err != nil {
		logs.Error(err)
		return "Initializing cache failed"
	}

	err = api.InitializeClient()
	if err != nil {
		logs.Error(err)
		return "Initializing HTTP client failed"
	}

	noneAvailable := true
	for _, rep := range api.GetAllRepositories() {
		if err := api.GetEnvs(rep); err != nil {
			logs.Error(err)
		} else {
			noneAvailable = false
		}
	}

	if noneAvailable {
		return "No services available"
	}

	return ""
}
