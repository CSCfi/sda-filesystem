package main

import (
	"context"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sda-filesystem/internal/logs"
)

type Log struct {
	Level     string `json:"loglevel"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

type LogHandler struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewLogHandler() *LogHandler {
	return &LogHandler{}
}

func (lh *LogHandler) SetContext(ctx context.Context) {
	lh.ctx = ctx
	logs.SetSignal(lh.AddLog)
}

func (lh *LogHandler) AddLog(level string, message []string) {
	lg := Log{Level: level, Timestamp: time.Now().Format("2006-01-02 15:04:05"), Message: message[0]}
	wailsruntime.EventsEmit(lh.ctx, "newLogEntry", lg)
}

func (lh *LogHandler) SaveLogs(url string, logs []Log) {
	/*file := url //core.QDir_ToNativeSeparators(core.NewQUrl3(url, 0).ToLocalFile())

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

	for i := range lh.logs {
		lg := lh.logs[i]
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

	logs.Infof("Logs written successfully to file %s", file)*/
}
