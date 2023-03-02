package main

import (
	"context"
	"sda-filesystem/internal/filesystem"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ProjectHandler struct {
	ctx      context.Context
	projects []filesystem.Project
	progress map[filesystem.Project][]int
}

// NewApp creates a new App application struct
func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{projects: make([]filesystem.Project, 0), progress: make(map[filesystem.Project][]int)}
}

func (ph *ProjectHandler) SetContext(ctx context.Context) {
	ph.ctx = ctx
}

func (ph *ProjectHandler) AddProject(pr filesystem.Project) {
	ph.projects = append(ph.projects, pr)
	ph.progress[pr] = []int{0, -1}
}

func (ph *ProjectHandler) sendProjects() {
	wailsruntime.EventsEmit(ph.ctx, "sendProjects", ph.projects)
}

func (ph *ProjectHandler) trackContainers(rep, pr string, count int) {
	if rep == "" {
		wailsruntime.EventsEmit(ph.ctx, "showProgress")

		return
	}

	project := filesystem.Project{Name: pr, Repository: rep}
	if ph.progress[project][1] == -1 {
		ph.progress[project][1] = count

		if count == 0 {
			wailsruntime.EventsEmit(ph.ctx, "updateGlobalProgress", 1, -1)
			wailsruntime.EventsEmit(ph.ctx, "updateProjectProgress", project, 100)

			return
		}

		wailsruntime.EventsEmit(ph.ctx, "updateGlobalProgress", 0, -count)

		return
	}

	ph.progress[project][0] += count
	progress := 100
	if ph.progress[project][1] != 0 && ph.progress[project][0] != ph.progress[project][1] {
		progress = int(float64(ph.progress[project][0]) / float64(ph.progress[project][1]) * 100)
	}
	wailsruntime.EventsEmit(ph.ctx, "updateGlobalProgress", count, 0)
	wailsruntime.EventsEmit(ph.ctx, "updateProjectProgress", project, progress)
}

func (ph *ProjectHandler) deleteProjects() {
	ph.projects = []filesystem.Project{}
	ph.progress = make(map[filesystem.Project][]int)
}
