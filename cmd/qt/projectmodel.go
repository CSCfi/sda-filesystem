package main

import (
	"sd-connect-fuse/internal/api"

	"github.com/therecipe/qt/core"
)

const (
	ProjectName = int(core.Qt__UserRole) + 1<<iota
	ContainerCount
)

// For some reason (bug?) the slot apiToProject() did not recognize []api.APIData as a list
type APIDataList []api.APIData

type ProjectModel struct {
	core.QAbstractListModel

	_ func() `constructor:"init"`

	_ map[int]*core.QByteArray `property:"roles"`
	_ []*Project               `property:"projects"`

	_ func(APIDataList) `slot:"apiToProject"`
}

type Project struct {
	core.QObject

	_ string `property:"projectName"`
	_ int    `property:"containerCount"`
}

func init() {
	Project_QRegisterMetaType()
}

func (pm *ProjectModel) init() {
	pm.SetRoles(map[int]*core.QByteArray{
		ProjectName:    core.NewQByteArray2("projectName", -1),
		ContainerCount: core.NewQByteArray2("containerCount", -1),
	})

	pm.ConnectData(pm.data)
	pm.ConnectRowCount(pm.rowCount)
	pm.ConnectColumnCount(pm.columnCount)
	pm.ConnectRoleNames(pm.roleNames)
	pm.ConnectApiToProject(pm.apiToProject)
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

	case ContainerCount:
		{
			return core.NewQVariant1(p.ContainerCount())
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
	return 1
}

func (pm *ProjectModel) roleNames() map[int]*core.QByteArray {
	return pm.Roles()
}

func (pm *ProjectModel) apiToProject(projectsAPI APIDataList) {
	projects := make([]*Project, len(projectsAPI))

	for i := range projectsAPI {
		var pr = NewProject(nil)
		pr.SetProjectName(projectsAPI[i].Name)
		pr.SetContainerCount(0)
		projects[i] = pr
	}

	pm.SetProjects(projects)
}

/*func (m *ProjectModel) editProject(row int, firstName string, lastName string) {
	var p = m.Projects()[row]

	if firstName != "" {
		p.SetFirstName(firstName)
	}

	if lastName != "" {
		p.SetLastName(lastName)
	}

	var pIndex = m.Index(row, 0, core.NewQModelIndex())
	m.DataChanged(pIndex, pIndex, []int{FirstName, LastName})
}*/
