package main

import (
	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/filesystem"

	"github.com/therecipe/qt/core"
)

const (
	ProjectName = int(core.Qt__UserRole) + 1<<iota
	LoadedContainers
	AllContainers
	NoStorage
)

// For some reason (bug?) the signal apiToProject() does not recognize []api.Metadata as a list
type metadataList []api.Metadata

type ProjectModel struct {
	core.QAbstractTableModel

	_ func() `constructor:"init"`

	_ func(metadataList)    `signal:"apiToProject,auto"`
	_ func(map[string]bool) `signal:"updateNoStorage,auto"`
	_ func(count int)       `signal:"noStorageWarning"`

	_ int `property:"loadedProjects"`

	roles       map[int]*core.QByteArray
	projects    []*project
	nameToIndex map[string]int
}

type project struct {
	projectName      string
	loadedContainers int
	allContainers    int
	noStorage        bool
}

func (pm *ProjectModel) init() {
	pm.roles = map[int]*core.QByteArray{
		ProjectName:      core.NewQByteArray2("projectName", -1),
		LoadedContainers: core.NewQByteArray2("loadedContainers", -1),
		AllContainers:    core.NewQByteArray2("allContainers", -1),
		NoStorage:        core.NewQByteArray2("noStorage", -1),
	}
	pm.SetLoadedProjects(0)

	pm.ConnectData(pm.data)
	pm.ConnectRowCount(pm.rowCount)
	pm.ConnectColumnCount(pm.columnCount)
	pm.ConnectRoleNames(pm.roleNames)
}

func (pm *ProjectModel) data(index *core.QModelIndex, role int) *core.QVariant {
	if !index.IsValid() {
		return core.NewQVariant()
	}

	if index.Row() < 0 || index.Row() >= len(pm.projects) {
		return core.NewQVariant()
	}

	var p = pm.projects[index.Row()]

	switch role {
	case ProjectName:
		{
			return core.NewQVariant1(p.projectName)
		}

	case LoadedContainers:
		{
			return core.NewQVariant1(p.loadedContainers)
		}

	case AllContainers:
		{
			return core.NewQVariant1(p.allContainers)
		}

	case NoStorage:
		{
			return core.NewQVariant1(p.noStorage)
		}

	default:
		{
			return core.NewQVariant()
		}
	}
}

func (pm *ProjectModel) rowCount(parent *core.QModelIndex) int {
	return len(pm.projects)
}

func (pm *ProjectModel) columnCount(parent *core.QModelIndex) int {
	return 2
}

func (pm *ProjectModel) roleNames() map[int]*core.QByteArray {
	return pm.roles
}

func (pm *ProjectModel) apiToProject(projectsAPI metadataList) {
	pm.projects = make([]*project, len(projectsAPI))
	pm.nameToIndex = make(map[string]int)

	for i := range projectsAPI {
		pm.projects[i] = &project{projectName: projectsAPI[i].Name, loadedContainers: 0,
			allContainers: -1, noStorage: false}
		pm.nameToIndex[projectsAPI[i].Name] = i
	}
}

func (pm *ProjectModel) waitForInfo(ch <-chan filesystem.LoadProjectInfo) {
	for info := range ch {
		row := pm.nameToIndex[info.Project]
		var pr = pm.projects[row]
		if pr.allContainers != -1 {
			pr.loadedContainers += info.Count
			var index = pm.Index(row, 1, core.NewQModelIndex())
			pm.DataChanged(index, index, []int{LoadedContainers})
			if pr.loadedContainers == pr.allContainers {
				pm.SetLoadedProjects(pm.LoadedProjects() + 1)
			}
		} else {
			pr.allContainers = info.Count
			var index = pm.Index(row, 1, core.NewQModelIndex())
			pm.DataChanged(index, index, []int{AllContainers})
			if pr.allContainers == 0 {
				pm.SetLoadedProjects(pm.LoadedProjects() + 1)
			}
		}
	}
}

func (pm *ProjectModel) updateNoStorage(projectsWithStrorage map[string]bool) {
	count := 0

	for i := range pm.projects {
		var pr = pm.projects[i]
		if _, ok := projectsWithStrorage[pr.projectName]; !ok {
			pr.noStorage = true
			pm.DataChanged(pm.Index(i, 0, core.NewQModelIndex()),
				pm.Index(i, 1, core.NewQModelIndex()),
				[]int{NoStorage})
			count++
		}
	}

	if count > 0 {
		pm.NoStorageWarning(count)
	}
}
