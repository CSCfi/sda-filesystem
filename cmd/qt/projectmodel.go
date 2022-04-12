package main

import (
	"sort"

	"github.com/therecipe/qt/core"
)

const (
	ProjectName = int(core.Qt__UserRole) + 1<<iota
	RepositoryName
	LoadedContainers
	AllContainers
)

type ProjectModel struct {
	core.QAbstractListModel

	_ func() `constructor:"init"`

	_ func(string, string)      `signal:"addProject,auto"`
	_ func(string, string, int) `signal:"addToCount,auto"`
	_ func()                    `signal:"prepareForRefresh,auto"`
	_ func()                    `signal:"deleteExtraProjects,auto"`

	_ int `property:"loadedProjects"`

	roles       map[int]*core.QByteArray
	projects    []project
	nameToIndex map[string]int
	deletedIdxs map[int]bool
}

type project struct {
	projectName      string
	repositoryName   string
	loadedContainers int
	allContainers    int
}

func (pm *ProjectModel) init() {
	pm.roles = map[int]*core.QByteArray{
		ProjectName:      core.NewQByteArray2("projectName", -1),
		RepositoryName:   core.NewQByteArray2("repositoryName", -1),
		LoadedContainers: core.NewQByteArray2("loadedContainers", -1),
		AllContainers:    core.NewQByteArray2("allContainers", -1),
	}
	pm.SetLoadedProjects(0)

	pm.ConnectData(pm.data)
	pm.ConnectRowCount(pm.rowCount)
	pm.ConnectRoleNames(pm.roleNames)

	pm.projects = []project{}
	pm.nameToIndex = make(map[string]int)
	pm.deletedIdxs = make(map[int]bool)
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
		return core.NewQVariant1(p.projectName)
	case RepositoryName:
		return core.NewQVariant1(p.repositoryName)
	case LoadedContainers:
		return core.NewQVariant1(p.loadedContainers)
	case AllContainers:
		return core.NewQVariant1(p.allContainers)
	default:
		return core.NewQVariant()
	}
}

func (pm *ProjectModel) rowCount(parent *core.QModelIndex) int {
	return len(pm.projects)
}

func (pm *ProjectModel) roleNames() map[int]*core.QByteArray {
	return pm.roles
}

func (pm *ProjectModel) addProject(rep, pr string) {
	if idx, ok := pm.nameToIndex[rep+"/"+pr]; ok {
		delete(pm.deletedIdxs, idx)
		return
	}

	length := len(pm.projects)
	pm.nameToIndex[rep+"/"+pr] = length
	pm.BeginInsertRows(core.NewQModelIndex(), length, length)
	pm.projects = append(pm.projects, project{repositoryName: rep, projectName: pr, allContainers: -1})
	pm.EndInsertRows()
}

func (pm *ProjectModel) addToCount(rep, pr string, count int) {
	row := pm.nameToIndex[rep+"/"+pr]
	var project = &pm.projects[row]
	var index = pm.Index(row, 0, core.NewQModelIndex())

	if project.allContainers != -1 {
		project.loadedContainers += count
		pm.DataChanged(index, index, []int{LoadedContainers})
	} else {
		project.allContainers = count
		pm.DataChanged(index, index, []int{AllContainers})
	}

	if project.loadedContainers == project.allContainers {
		pm.SetLoadedProjects(pm.LoadedProjects() + 1)
	}
}

func (pm *ProjectModel) prepareForRefresh() {
	pm.deletedIdxs = make(map[int]bool)
	for i := range pm.projects {
		pm.deletedIdxs[i] = true
		pm.projects[i].loadedContainers = 0
		pm.projects[i].allContainers = -1
	}
	pm.SetLoadedProjects(0)
	pm.DataChanged(pm.Index(0, 0, core.NewQModelIndex()),
		pm.Index(len(pm.projects)-1, 0, core.NewQModelIndex()), []int{LoadedContainers})
	pm.DataChanged(pm.Index(0, 0, core.NewQModelIndex()),
		pm.Index(len(pm.projects)-1, 0, core.NewQModelIndex()), []int{AllContainers})
}

func (pm *ProjectModel) deleteExtraProjects() {
	deletedSlice := []int{}
	for i := range pm.deletedIdxs {
		deletedSlice = append(deletedSlice, i)
	}
	sort.Ints(deletedSlice)

	for i, deleted := range deletedSlice {
		pm.BeginRemoveRows(core.NewQModelIndex(), deleted-i, deleted-i)
		pm.projects = append(pm.projects[:deleted-i], pm.projects[deleted-i+1:]...)
		pm.EndRemoveRows()
	}
}
