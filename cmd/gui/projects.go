package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Project struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
}

type ProjectHandler struct {
	ctx      context.Context
	progress map[Project][]int
}

func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{}
}

func (ph *ProjectHandler) SetContext(ctx context.Context) {
	ph.ctx = ctx
}

func (ph *ProjectHandler) trackProjectProgress(rep, name string, count int) {
	if rep == "" {
		wailsruntime.EventsEmit(ph.ctx, "showProgress")

		return
	}

	project := Project{Name: name, Repository: rep}
	_, ok := ph.progress[project]
	if !ok {
		ph.progress[project] = []int{0, count}
		wailsruntime.EventsEmit(ph.ctx, "updateGlobalProgress", 0, -count)
		wailsruntime.EventsEmit(ph.ctx, "updateProjectProgress", project.Name, project.Repository, 0)

		return
	}

	ph.progress[project][0] += count
	progress := 100
	if ph.progress[project][1] != 0 && ph.progress[project][0] != ph.progress[project][1] {
		progress = int(float64(ph.progress[project][0]) / float64(ph.progress[project][1]) * 100)
	}
	wailsruntime.EventsEmit(ph.ctx, "updateGlobalProgress", count, 0)
	wailsruntime.EventsEmit(ph.ctx, "updateProjectProgress", project.Name, project.Repository, progress)
}

func (ph *ProjectHandler) DeleteProjects() {
	ph.progress = make(map[Project][]int)
}
