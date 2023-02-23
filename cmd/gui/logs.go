package main

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	"sda-filesystem/internal/logs"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Log struct {
	level     string
	timestamp string
	message   []string
}

type LogHandler struct {
	ctx  context.Context
	logs []Log
}

// NewApp creates a new App application struct
func NewLogHandler() *LogHandler {
	return &LogHandler{}
}

func (lh *LogHandler) SetContext(ctx context.Context) {
	lh.ctx = ctx
	logs.SetLevel("info")
	logs.SetSignal(lh.AddLog)
}

func (lh *LogHandler) AddLog(level string, message []string) {
	lg := Log{level: level, timestamp: time.Now().Format("2006-01-02 15:04:05.000000"), message: message}
	lh.logs = append(lh.logs, lg)
	wailsruntime.EventsEmit(lh.ctx, "newLogEntry", lg.level, lg.timestamp, lg.message[0])
}

func (lh *LogHandler) SaveLogs() {
	home, _ := os.UserHomeDir()
	options := wailsruntime.SaveDialogOptions{DefaultDirectory: home, DefaultFilename: "gateway.log"}
	file, err := wailsruntime.SaveFileDialog(lh.ctx, options)
	if err != nil {
		logs.Errorf("Could not select file name: %w", err)

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
	if runtime.GOOS == "windows" {
		newline = "\r\n"
	}

	for i := range lh.logs {
		lg := lh.logs[i]
		str := strings.ToUpper(lg.level)[:4] + "[" +
			strings.ReplaceAll(strings.Split(lg.timestamp, ".")[0], " ", "T") + "] " +
			strings.Join(lg.message, ": ")

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
