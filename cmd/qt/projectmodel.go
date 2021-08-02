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
)

// For some reason (bug?) the slot apiToProject() did not recognize []api.Metadata as a list
type metadataList []api.Metadata

type ProjectModel struct {
	core.QAbstractTableModel

	_ func() `constructor:"init"`

	_ map[int]*core.QByteArray `property:"roles"`
	_ []*Project               `property:"projects"`
	_ map[string]int           `property:"nameToIndex"`

	_ func(metadataList) `slot:"apiToProject"`
}

type Project struct {
	core.QObject

	_ string `property:"projectName"`
	_ int    `property:"loadedContainers"`
	_ int    `property:"allContainers"`
}

func init() {
	Project_QRegisterMetaType()
}

func (pm *ProjectModel) init() {
	pm.SetRoles(map[int]*core.QByteArray{
		ProjectName:      core.NewQByteArray2("projectName", -1),
		LoadedContainers: core.NewQByteArray2("loadedContainers", -1),
		AllContainers:    core.NewQByteArray2("allContainers", -1),
	})

	pm.ConnectData(pm.data)
	pm.ConnectRowCount(pm.rowCount)
	pm.ConnectColumnCount(pm.columnCount)
	pm.ConnectRoleNames(pm.roleNames)
	pm.ConnectApiToProject(pm.apiToProject)
	pm.ConnectProjectsChanged(pm.projectsChanged)
}

func (pm *ProjectModel) data(index *core.QModelIndex, role int) *core.QVariant {
	if !index.IsValid() {
		return core.NewQVariant()
	}

	if index.Row() >= len(pm.Projects()) {
		return core.NewQVariant()
	}

	var p = pm.Projects()[index.Row()]

	switch role {
	case ProjectName:
		{
			return core.NewQVariant1(p.ProjectName())
		}

	case LoadedContainers:
		{
			return core.NewQVariant1(p.LoadedContainers())
		}

	case AllContainers:
		{
			return core.NewQVariant1(p.AllContainers())
		}

	default:
		{
			return core.NewQVariant()
		}
	}
}

func (pm *ProjectModel) rowCount(parent *core.QModelIndex) int {
	return len(pm.Projects())
}

func (pm *ProjectModel) columnCount(parent *core.QModelIndex) int {
	return 2
}

func (pm *ProjectModel) roleNames() map[int]*core.QByteArray {
	return pm.Roles()
}

func (pm *ProjectModel) apiToProject(projectsAPI metadataList) {
	projects := make([]*Project, len(projectsAPI))

	for i := range projectsAPI {
		var pr = NewProject(nil)
		pr.SetProjectName(projectsAPI[i].Name)
		pr.SetLoadedContainers(0)
		pr.SetAllContainers(-1)
		projects[i] = pr
	}

	pm.SetProjects(projects)
}

func (pm *ProjectModel) waitForInfo(ch <-chan filesystem.LoadProjectInfo) {
	for info := range ch {
		row := pm.NameToIndex()[info.Project]
		var pr = pm.Projects()[row]
		if pr.AllContainers() != -1 {
			pr.SetLoadedContainers(pr.LoadedContainers() + info.Count)
			pm.DataChanged(pm.Index(row, 1, core.NewQModelIndex()),
				pm.Index(row, 1, core.NewQModelIndex()),
				[]int{LoadedContainers})
		} else {
			pr.SetAllContainers(info.Count)
			pm.DataChanged(pm.Index(row, 1, core.NewQModelIndex()),
				pm.Index(row, 1, core.NewQModelIndex()),
				[]int{AllContainers})
		}
	}
}

func (pm *ProjectModel) projectsChanged(projects []*Project) {
	toIndex := make(map[string]int)

	for i := range projects {
		toIndex[projects[i].ProjectName()] = i
	}

	pm.SetNameToIndex(toIndex)
}
