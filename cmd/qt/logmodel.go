package main

import (
	"bufio"
	"fmt"
	"os"
	"sd-connect-fuse/internal/logs"
	"strings"

	"github.com/therecipe/qt/core"
)

const (
	Level = int(core.Qt__UserRole) + 1<<iota
	Timestamp
	Message
)

type LogModel struct {
	core.QAbstractTableModel

	_ func()               `constructor:"init"`
	_ func(string, string) `slot:"addLog"`
	_ func()               `slot:"removeDummy"`
	_ func(url string)     `slot:"saveLogs"`

	_ map[int]*core.QByteArray `property:"roles"`
	_ []*Log                   `property:"logs"`
	_ bool                     `property:"dummyPresent"`
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
	lm.ConnectRemoveDummy(lm.removeDummy)
	lm.ConnectSaveLogs(lm.saveLogs)

	lm.addDummy()
}

func (lm *LogModel) data(index *core.QModelIndex, role int) *core.QVariant {
	if !index.IsValid() {
		return core.NewQVariant()
	}

	if index.Row() < 0 || index.Row() >= len(lm.Logs()) {
		return core.NewQVariant()
	}

	var l = lm.Logs()[len(lm.Logs())-index.Row()-1]

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
	return 3
}

func (lm *LogModel) roleNames() map[int]*core.QByteArray {
	return lm.Roles()
}

func (lm *LogModel) addDummy() {
	var lg = NewLog(nil)
	lg.SetLevel("WARNING")
	lg.SetTimestamp("0000-00-00 00:00:00")
	lg.SetMessage("")
	lm.SetLogs([]*Log{lg})
	lm.SetDummyPresent(true)
}

func (lm *LogModel) removeDummy() {
	lm.BeginRemoveRows(core.NewQModelIndex(), len(lm.Logs())-1, len(lm.Logs())-1)
	lm.SetLogs(lm.Logs()[:len(lm.Logs())-1])
	lm.EndRemoveRows()
	lm.SetDummyPresent(false)
}

func (lm *LogModel) addLog(level, message string) {
	var lg = NewLog(nil)
	lg.SetLevel(level)
	lg.SetTimestamp(core.QDateTime_CurrentDateTime().ToString("yyyy-MM-dd hh:mm:ss"))
	lg.SetMessage(message)

	length := len(lm.Logs())
	if lm.IsDummyPresent() {
		lm.BeginInsertRows(core.NewQModelIndex(), length-1, length-1)
		lm.SetLogs(append(lm.Logs()[:length-1], lg, lm.Logs()[length-1]))
	} else {
		lm.BeginInsertRows(core.NewQModelIndex(), length, length)
		lm.SetLogs(append(lm.Logs(), lg))
	}
	lm.EndInsertRows()
}

func (lm *LogModel) saveLogs(url string) {
	file := core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())

	len := len(lm.Logs())
	f, err := os.Create(file)
	if err != nil {
		logs.Errorf("Could not create file %s: %w", file, err)
		return
	}
	defer f.Close()

	writer := bufio.NewWriter(f)

	for i := range lm.Logs() {
		lg := lm.Logs()[len-i-1]
		str := fmt.Sprintf(strings.ToUpper(lg.Level()) + "[" +
			strings.ReplaceAll(lg.Timestamp(), " ", "T") + "] " +
			strings.ReplaceAll(lg.Message(), "\n", " "))

		if _, err = writer.WriteString(str + "\n"); err != nil {
			logs.Errorf("Something went wrong when writing to file %s: %w", file, err)
			return
		}
	}

	err = writer.Flush()
	if err != nil {
		logs.Errorf("Could not flush file %s: %w", file, err)
	}

	logs.Info("Logs written successfully to file ", file)
}
