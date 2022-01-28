package main

import (
	"bufio"
	"fmt"
	"os"
	"sda-filesystem/internal/logs"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/therecipe/qt/core"
)

const (
	Level = int(core.Qt__UserRole) + 1<<iota
	Timestamp
	Message
)

type LogLevel struct {
	core.QObject

	_ func() `constructor:"init"`

	_ int `property:"Error"`
	_ int `property:"Warning"`
	_ int `property:"Info"`
	_ int `property:"Debug"`
}

func (ll *LogLevel) init() {
	ll.SetError(int(logrus.ErrorLevel))
	ll.SetWarning(int(logrus.WarnLevel))
	ll.SetInfo(int(logrus.InfoLevel))
	ll.SetDebug(int(logrus.DebugLevel))
}

type LogModel struct {
	core.QAbstractListModel

	_ func()              `constructor:"init"`
	_ func(int, []string) `signal:"addLog,auto"`
	_ func(string)        `signal:"saveLogs,auto"`

	_ int `property:"count"`

	roles map[int]*core.QByteArray
	logs  []logRow
}

type logRow struct {
	level     int
	timestamp string
	message   []string
}

func (lm *LogModel) init() {
	lm.roles = map[int]*core.QByteArray{
		Level:     core.NewQByteArray2("level", -1),
		Timestamp: core.NewQByteArray2("timestamp", -1),
		Message:   core.NewQByteArray2("message", -1),
	}

	lm.ConnectData(lm.data)
	lm.ConnectRowCount(lm.rowCount)
	lm.ConnectRoleNames(lm.roleNames)
	lm.SetCount(0)
}

func (lm *LogModel) data(index *core.QModelIndex, role int) *core.QVariant {
	if !index.IsValid() {
		return core.NewQVariant()
	}

	if index.Row() < 0 || index.Row() >= len(lm.logs) {
		return core.NewQVariant()
	}

	var l = lm.logs[index.Row()]

	switch role {
	case Level:
		return core.NewQVariant1(l.level)
	case Timestamp:
		return core.NewQVariant1(l.timestamp)
	case Message:
		return core.NewQVariant1(l.message)
	default:
		return core.NewQVariant()
	}
}

func (lm *LogModel) rowCount(parent *core.QModelIndex) int {
	return len(lm.logs)
}

func (lm *LogModel) roleNames() map[int]*core.QByteArray {
	return lm.roles
}

func (lm *LogModel) addLog(level int, message []string) {
	lg := logRow{level: int(level),
		timestamp: core.QDateTime_CurrentDateTime().ToString("yyyy-MM-dd hh:mm:ss"), message: message}
	length := len(lm.logs)

	lm.BeginInsertRows(core.NewQModelIndex(), length, length)
	lm.logs = append(lm.logs, lg)
	lm.EndInsertRows()

	lm.SetCount(length + 1)
}

func (lm *LogModel) saveLogs(url string) {
	file := core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())

	f, err := os.Create(file)
	if err != nil {
		logs.Errorf("Could not create file %s: %w", file, err)
		return
	}
	defer f.Close()

	writer := bufio.NewWriter(f)

	for i := range lm.logs {
		lg := lm.logs[i]
		str := fmt.Sprintf(strings.ToUpper(logrus.Level(lg.level).String())[:4] + "[" +
			strings.ReplaceAll(lg.timestamp, " ", "T") + "] " +
			strings.Join(lg.message, ": "))

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
