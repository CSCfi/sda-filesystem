package logs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

var signal func(string, string) = nil

var levelMap = map[string]log.Level{
	"debug": log.DebugLevel,
	"info":  log.InfoLevel,
	"error": log.ErrorLevel,
}

func SetSignal(fn func(string, string)) {
	signal = fn
}

func SetLevel(level string) {
	if logrusLevel, ok := levelMap[strings.ToLower(level)]; ok {
		log.SetLevel(logrusLevel)
		return
	}

	Infof("-loglevel=%s is not supported, possible values are {debug,info,error}, setting fallback loglevel to 'info'", level)
	log.SetLevel(log.InfoLevel)
}

func Wrapper(err error) (string, error) {
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		return strings.TrimRight(strings.TrimSuffix(err.Error(), unwrapped.Error()), ": "), unwrapped
	}
	return err.Error(), nil
}

func StructureError(err error) string {
	fullError := ""
	for i := 0; err != nil; i++ {
		var str string
		str, err = Wrapper(err)
		fullError += str + "\n"
	}
	return fullError[:len(fullError)-1]
}

func Error(err error) {
	if signal != nil {
		signal("ERROR", StructureError(err))
	} else {
		log.Error(err)
	}
}

func Errorf(format string, args ...interface{}) {
	if signal != nil {
		err := fmt.Errorf(format, args...)
		signal("ERROR", StructureError(err))
	} else {
		log.Errorf(format, args...)
	}
}

func Warning(err error) {
	if signal != nil {
		signal("WARNING", StructureError(err))
	} else {
		log.Warning(err)
	}
}

func Warningf(format string, args ...interface{}) {
	if signal != nil {
		signal("WARNING", fmt.Sprintf(format, args...))
	} else {
		log.Warningf(format, args...)
	}
}

func Info(args ...interface{}) {
	if signal != nil {
		signal("INFO", fmt.Sprint(args...))
	} else {
		log.Info(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if signal != nil {
		signal("INFO", fmt.Sprintf(format, args...))
	} else {
		log.Infof(format, args...)
	}
}

func Debug(args ...interface{}) {
	if signal != nil {
		if log.GetLevel() == log.DebugLevel {
			signal("DEBUG", fmt.Sprint(args...))
		}
	} else {
		log.Debug(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if signal != nil {
		if log.GetLevel() == log.DebugLevel {
			signal("DEBUG", fmt.Sprintf(format, args...))
		}
	} else {
		log.Debugf(format, args...)
	}
}

func Fatal(args ...interface{}) {
	log.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

func init() {
	// Configure Log Text Formatter
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
}
