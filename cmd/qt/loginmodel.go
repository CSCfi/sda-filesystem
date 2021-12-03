package main

import (
	"sda-filesystem/internal/api"
	"sort"

	"github.com/therecipe/qt/core"
)

const (
	Repository = int(core.Qt__UserRole) + 1<<iota
	Method
)

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

	_ func(int, bool)   `signal:"setChecked,auto"`
	_ func(int, string) `signal:"setUsername,auto"`
	_ func(int, string) `signal:"setPassword,auto"`

	roles  map[int]*core.QByteArray
	logins []loginRow
}

type loginRow struct {
	repository string
	method     int
	checked    bool
	username   string
	password   string
}

func (lm *LoginModel) init() {
	lm.roles = map[int]*core.QByteArray{
		Repository: core.NewQByteArray2("repository", -1),
		Method:     core.NewQByteArray2("method", -1),
	}

	lm.ConnectData(lm.data)
	lm.ConnectRowCount(lm.rowCount)
	lm.ConnectRoleNames(lm.roleNames)

	reps := api.GetAllPossibleRepositories()
	sort.Strings(reps)
	for i := len(reps) - 1; i >= 0; i-- {
		lm.logins = append(lm.logins, loginRow{repository: reps[i], method: int(api.GetLoginMethod(reps[i])),
			username: "", password: ""})
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

func (lm *LoginModel) setChecked(idx int, checked bool) {
	lm.logins[idx].checked = checked
}

func (lm *LoginModel) setUsername(idx int, username string) {
	lm.logins[idx].username = username
}

func (lm *LoginModel) setPassword(idx int, password string) {
	lm.logins[idx].password = password
}

func (lm *LoginModel) getAuth() map[string][]string {
	ret := make(map[string][]string)
	for i := range lm.logins {
		lg := lm.logins[i]
		if lg.checked {
			if lg.method == int(api.Password) {
				ret[lg.repository] = []string{lg.username, lg.password}
			} else {
				ret[lg.repository] = []string{}
			}
		}
	}
	return ret
}
