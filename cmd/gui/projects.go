package main

import (
	"context"
	"sync"

	"sda-filesystem/internal/api"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Project struct {
	Name       string   `json:"name"`
	Repository api.Repo `json:"repository"`
}

type progress struct {
	current int
	max     int
}

type ProjectHandler struct {
	ctx  context.Context
	prog map[Project]progress
	lock sync.Mutex
}

func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{}
}

func (ph *ProjectHandler) SetContext(ctx context.Context) {
	ph.ctx = ctx
}

func (ph *ProjectHandler) trackProjectProgress(rep api.Repo, name string, count int) {
	ph.lock.Lock()
	defer ph.lock.Unlock()

	if rep == "" {
		wailsruntime.EventsEmit(ph.ctx, "showProgress")

		return
	}

	project := Project{Name: name, Repository: rep}
	prog, ok := ph.prog[project]
	if !ok {
		ph.prog[project] = progress{current: 0, max: count}
		wailsruntime.EventsEmit(ph.ctx, "updateGlobalProgress", 0, -count)
		wailsruntime.EventsEmit(ph.ctx, "updateProjectProgress", project.Name, project.Repository, 0)

		return
	}

	ph.prog[project] = progress{prog.current + count, prog.max}
	progress := 100
	if ph.prog[project].max != 0 && ph.prog[project].current != ph.prog[project].max {
		progress = int(float64(ph.prog[project].current) / float64(ph.prog[project].max) * 100)
	}
	wailsruntime.EventsEmit(ph.ctx, "updateGlobalProgress", count, 0)
	wailsruntime.EventsEmit(ph.ctx, "updateProjectProgress", project.Name, project.Repository, progress)
}

func (ph *ProjectHandler) DeleteProjects() {
	ph.prog = make(map[Project]progress)
}
