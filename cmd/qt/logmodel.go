package main

import (
	"time"

	"github.com/therecipe/qt/core"
)

const (
	Level = int(core.Qt__UserRole) + 1<<iota
	Timestamp
	Message
)

type LogModel struct {
	core.QAbstractListModel

	_ func() `constructor:"init"`

	_ map[int]*core.QByteArray `property:"roles"`
	_ []*Log                   `property:"logs"`

	_ func(string, string) `slot:"addLog"`
}

type Log struct {
	core.QObject

	_ string `property:"level"`
	_ string `property:"timestamp"`
	_ string `property:"message"`
}

func init() {
	Log_QRegisterMetaType()
}

func (lm *LogModel) init() {
	lm.SetRoles(map[int]*core.QByteArray{
		Level:     core.NewQByteArray2("level", -1),
		Timestamp: core.NewQByteArray2("timestamp", -1),
		Message:   core.NewQByteArray2("message", -1),
	})

	lm.ConnectData(lm.data)
	lm.ConnectRowCount(lm.rowCount)
	lm.ConnectColumnCount(lm.columnCount)
	lm.ConnectRoleNames(lm.roleNames)
	lm.ConnectAddLog(lm.addLog)
}

func (lm *LogModel) data(index *core.QModelIndex, role int) *core.QVariant {
	if !index.IsValid() {
		return core.NewQVariant()
	}

	if index.Row() >= len(lm.Logs()) {
		return core.NewQVariant()
	}

	var l = lm.Logs()[index.Row()]

	switch role {
	case Level:
		{
			return core.NewQVariant1(l.Level())
		}

	case Timestamp:
		{
			return core.NewQVariant1(l.Timestamp())
		}

	case Message:
		{
			return core.NewQVariant1(l.Message())
		}

	default:
		{
			return core.NewQVariant()
		}
	}
}

func (lm *LogModel) rowCount(parent *core.QModelIndex) int {
	return len(lm.Logs())
}

func (lm *LogModel) columnCount(parent *core.QModelIndex) int {
	return 1
}

func (lm *LogModel) roleNames() map[int]*core.QByteArray {
	return lm.Roles()
}

func (lm *LogModel) addLog(level, message string) {
	var log = NewLog(nil)
	log.SetLevel(level)
	log.SetTimestamp(time.Now().String())
	log.SetMessage(message)

	lm.BeginInsertRows(core.NewQModelIndex(), len(lm.Logs()), len(lm.Logs()))
	lm.SetLogs(append(lm.Logs(), log))
	lm.EndInsertRows()
}
