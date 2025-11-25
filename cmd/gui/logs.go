package main

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"

	"sda-filesystem/internal/logs"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Log struct {
	Level     string   `json:"loglevel"`
	Timestamp string   `json:"timestamp"`
	Message   []string `json:"message"`
}

type LogHandler struct {
	ctx context.Context
}

func NewLogHandler() *LogHandler {
	return &LogHandler{}
}

func (lh *LogHandler) SetContext(ctx context.Context) {
	lh.ctx = ctx
	logs.SetLevel("info")
	logs.SetSignal(lh.AddLog)
}

func (lh *LogHandler) AddLog(level string, message []string) {
	lg := Log{Level: level, Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"), Message: message}
	wailsruntime.EventsEmit(lh.ctx, "newLogEntry", lg)
}

func (lh *LogHandler) SaveLogs(logsToSave []Log) {
	home, _ := os.UserHomeDir()
	options := wailsruntime.SaveDialogOptions{DefaultDirectory: home, DefaultFilename: "gateway.log"}
	file, err := wailsruntime.SaveFileDialog(lh.ctx, options)
	if err != nil {
		logs.Errorf("Could not select file name: %w", err)

		return
	}
	if file == "" { // Cancelled
		return
	}

	f, err := os.Create(file)
	if err != nil {
		logs.Errorf("Could not create file %s: %w", file, err)

		return
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	newline := "\n"

	for i := range logsToSave {
		lg := logsToSave[i]
		str := strings.ToUpper(lg.Level)[:4] + "[" +
			strings.ReplaceAll(strings.Split(lg.Timestamp, ".")[0], " ", "T") + "] " +
			strings.Join(lg.Message, ": ")

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
