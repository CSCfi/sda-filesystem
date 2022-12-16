package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Project struct {
	Name             string `json:"name"`
	Repository       string `json:"repository"`
	AllContainers    string `json:"allContainers"`
	LoadedContainers string `json:"loadedContainers"`
}

type ProjectHandler struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{}
}

func (ph *ProjectHandler) SetContext(ctx context.Context) {
	ph.ctx = ctx
}

func (ph *ProjectHandler) addProject(rep, pr string) {
	wailsruntime.EventsEmit(ph.ctx, "newLogEntry", Project{Name: pr, Repository: rep})
}

func (ph *ProjectHandler) trackContainers(rep, pr string, count int) {
	if rep == "" {
		wailsruntime.EventsEmit(ph.ctx, "startProgress")

		return
	}

	wailsruntime.EventsEmit(ph.ctx, "updateProgress", Project{Name: pr, Repository: rep}, count)
}
