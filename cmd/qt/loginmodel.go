package main

import (
	"sort"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/therecipe/qt/core"
)

const (
	Repository = int(core.Qt__UserRole) + 1<<iota
	Method
	LoggedIn
	EnvsMissing
)

// LoginMethod is used to transfer the api.LoginMethod enums to qml
type LoginMethod struct {
	core.QObject

	_ func() `constructor:"init"`

	_ int `property:"Password"`
	_ int `property:"Token"`
}

func (ll *LoginMethod) init() {
	ll.SetPassword(int(api.Password))
	ll.SetToken(int(api.Token))
}

type LoginModel struct {
	core.QAbstractListModel

	_ func() `constructor:"init"`

	_ bool `property:"loggedInToSDConnect"`
	_ int  `property:"connectIdx"`

	roles  map[int]*core.QByteArray
	logins []loginRow
}

type loginRow struct {
	repository  string
	method      int
	loggedIn    bool
	envsMissing bool
}

func (lm *LoginModel) init() {
	lm.roles = map[int]*core.QByteArray{
		Repository:  core.NewQByteArray2("repository", -1),
		Method:      core.NewQByteArray2("method", -1),
		LoggedIn:    core.NewQByteArray2("loggedIn", -1),
		EnvsMissing: core.NewQByteArray2("envsMissing", -1),
	}

	lm.SetLoggedInToSDConnect(false)

	lm.ConnectData(lm.data)
	lm.ConnectRowCount(lm.rowCount)
	lm.ConnectRoleNames(lm.roleNames)

	reps := api.GetAllPossibleRepositories()
	sort.Strings(reps)
	for i := len(reps) - 1; i >= 0; i-- {
		lm.logins = append(lm.logins, loginRow{repository: reps[i], method: int(api.GetLoginMethod(reps[i]))})
		if reps[i] == api.SDConnect {
			lm.SetConnectIdx(len(lm.logins) - 1)
		}
	}
}

func (lm *LoginModel) data(index *core.QModelIndex, role int) *core.QVariant {
	if !index.IsValid() {
		return core.NewQVariant()
	}

	if index.Row() < 0 || index.Row() >= len(lm.logins) {
		return core.NewQVariant()
	}

	var l = lm.logins[index.Row()]

	switch role {
	case Repository:
		return core.NewQVariant1(l.repository)
	case Method:
		return core.NewQVariant1(l.method)
	case LoggedIn:
		return core.NewQVariant1(l.loggedIn)
	case EnvsMissing:
		return core.NewQVariant1(l.envsMissing)
	default:
		return core.NewQVariant()
	}
}

func (lm *LoginModel) rowCount(parent *core.QModelIndex) int {
	return len(lm.logins)
}

func (lm *LoginModel) roleNames() map[int]*core.QByteArray {
	return lm.roles
}

func (lm *LoginModel) getRepository(idx int) string {
	return lm.logins[idx].repository
}

func (lm *LoginModel) setLoggedIn(idx int, value bool) {
	lm.logins[idx].loggedIn = value
	var index = lm.Index(idx, 0, core.NewQModelIndex())
	lm.DataChanged(index, index, []int{LoggedIn})

	if lm.getRepository(idx) == api.SDConnect {
		lm.SetLoggedInToSDConnect(true)
	}
}

func (lm *LoginModel) checkEnvs() bool {
	noneAvailable := true

	for i := range lm.logins {
		if err := api.GetEnvs(lm.logins[i].repository); err != nil {
			logs.Error(err)
			lm.logins[i].envsMissing = true
			var index = lm.Index(i, 0, core.NewQModelIndex())
			lm.DataChanged(index, index, []int{EnvsMissing})
		} else {
			noneAvailable = false
		}
	}

	return noneAvailable
}
