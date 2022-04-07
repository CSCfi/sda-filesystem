package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"sda-filesystem/internal/logs"

	"github.com/sirupsen/logrus"
	"github.com/therecipe/qt/core"
)

const (
	Level = int(core.Qt__UserRole) + 1<<iota
	Timestamp
	Message
)

// Couldn't find a way to use actual enums so this will have to do
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

// Contains the model filtered based on log levels (chosen by the user in UI)
type LogModelFiltered struct {
	core.QSortFilterProxyModel

	_ func() `constructor:"init"`

	chosenLevel int
}

func (lf *LogModelFiltered) init() {
	lf.ConnectFilterAcceptsRow(lf.filterAcceptsRow)
	lf.chosenLevel = -1
}

func (lf *LogModelFiltered) filterAcceptsRow(sourceRow int, sourceParent *core.QModelIndex) bool {
	var ok bool
	index := lf.SourceModel().Index(sourceRow, 0, sourceParent)
	level := lf.SourceModel().Data(index, Level).ToInt(&ok)
	return ok && (lf.chosenLevel == -1 || lf.chosenLevel == level)
}

// The actual model which contains all logs
type LogModel struct {
	core.QAbstractListModel

	_ func() `constructor:"init"`

	_ func(int)           `slot:"changeFilteredLevel,auto"`
	_ func(int) string    `slot:"getLevelStr,auto"`
	_ func(string)        `slot:"saveLogs,auto"`
	_ func(int, []string) `signal:"addLog,auto"`

	_ *LogModelFiltered `property:"proxy"`
	_ bool              `property:"includeDebug"`

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
	lm.ConnectGetLevelStr(lm.getLevelStr)

	proxy := NewLogModelFiltered(nil)
	proxy.SetSourceModel(lm)
	lm.SetProxy(proxy)

	lm.logs = []logRow{}
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

func (lm *LogModel) changeFilteredLevel(level int) {
	lm.Proxy().chosenLevel = level
	lm.Proxy().InvalidateFilter()
}

func (lm *LogModel) getLevelStr(level int) string {
	switch level {
	case int(logrus.ErrorLevel):
		return "Error"
	case int(logrus.WarnLevel):
		return "Warning"
	case int(logrus.InfoLevel):
		return "Info"
	case int(logrus.DebugLevel):
		return "Debug"
	case -1:
		return "All"
	}
	return ""
}

func (lm *LogModel) addLog(level int, message []string) {
	lg := logRow{level: int(level),
		timestamp: core.QDateTime_CurrentDateTime().ToString("yyyy-MM-dd hh:mm:ss"), message: message}
	length := len(lm.logs)

	lm.BeginInsertRows(core.NewQModelIndex(), length, length)
	lm.logs = append(lm.logs, lg)
	lm.EndInsertRows()
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

	newline := "\n"
	if runtime.GOOS == "windows" {
		newline = "\r\n"
	}

	for i := range lm.logs {
		lg := lm.logs[i]
		str := fmt.Sprintf(strings.ToUpper(logrus.Level(lg.level).String())[:4] + "[" +
			strings.ReplaceAll(lg.timestamp, " ", "T") + "] " +
			strings.Join(lg.message, ": "))

		if _, err = writer.WriteString(str + newline); err != nil {
			logs.Errorf("Something went wrong when writing to file %s: %w", file, err)
			return
		}
	}

	err = writer.Flush()
	if err != nil {
		logs.Errorf("Could not flush file %s: %w", file, err)
	}

	logs.Infof("Logs written successfully to file %s", file)
}
