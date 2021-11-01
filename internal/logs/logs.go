package logs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

// signal sends a new log to the LogModel
var signal func(int, []string) = nil

var levelMap = map[string]log.Level{
	"debug":   log.DebugLevel,
	"info":    log.InfoLevel,
	"warning": log.WarnLevel,
	"error":   log.ErrorLevel,
}

// SetSignal initializes 'signal', which sends logs to LogModel
func SetSignal(fn func(int, []string)) {
	signal = fn
}

// SetLevel sets the logging level
func SetLevel(level string) {
	if logrusLevel, ok := levelMap[strings.ToLower(level)]; ok {
		log.SetLevel(logrusLevel)
		return
	}

	Infof("-loglevel=%s is not supported, possible values are {debug,info,warning,error}, setting fallback loglevel to 'info'", level)
	log.SetLevel(log.InfoLevel)
}

// Wrapper returns the outermost error in err as a string along with the wrapped error
func Wrapper(err error) (string, error) {
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		return strings.TrimRight(strings.TrimSuffix(err.Error(), unwrapped.Error()), ": "), unwrapped
	}
	return err.Error(), nil
}

// StructureError divides err into a list of strings where each string represents one wrapped error
func StructureError(err error) []string {
	fullError := []string{}
	for i := 0; err != nil; i++ {
		var str string
		str, err = Wrapper(err)
		fullError = append(fullError, str)
	}
	return fullError
}

// Error logs a message at level "Error" either on the standard logger or in the GUI
func Error(err error) {
	if signal != nil {
		signal(int(log.ErrorLevel), StructureError(err))
	} else {
		log.Error(err)
	}
}

// Errorf logs a message at level "Error" either on the standard logger or in the GUI
func Errorf(format string, args ...interface{}) {
	if signal != nil {
		err := fmt.Errorf(format, args...)
		signal(int(log.ErrorLevel), StructureError(err))
	} else {
		log.Errorf(format, args...)
	}
}

// Warning logs a message at level "Warning" either on the standard logger or in the GUI
func Warning(err error) {
	if signal != nil {
		signal(int(log.WarnLevel), StructureError(err))
	} else {
		log.Warning(err.Error())
	}
}

// Warningf logs a message at level "Warning" either on the standard logger or in the GUI
func Warningf(format string, args ...interface{}) {
	if signal != nil {
		signal(int(log.WarnLevel), []string{fmt.Sprintf(format, args...)})
	} else {
		log.Warningf(format, args...)
	}
}

// Info logs a message at level "Info" either on the standard logger or in the GUI
func Info(args ...interface{}) {
	if signal != nil {
		signal(int(log.InfoLevel), []string{fmt.Sprint(args...)})
	} else {
		log.Info(args...)
	}
}

// Infof logs a message at level "Info" either on the standard logger or in the GUI
var Infof = func(format string, args ...interface{}) {
	if signal != nil {
		signal(int(log.InfoLevel), []string{fmt.Sprintf(format, args...)})
	} else {
		log.Infof(format, args...)
	}
}

// Debug logs a message at level "Debug" either on the standard logger or in the GUI
func Debug(args ...interface{}) {
	if signal != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			signal(int(log.DebugLevel), []string{fmt.Sprint(args...)})
		}
	} else {
		log.Debug(args...)
	}
}

// Debugf logs a message at level "Debug" either on the standard logger or in the GUI
func Debugf(format string, args ...interface{}) {
	if signal != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			signal(int(log.DebugLevel), []string{fmt.Sprintf(format, args...)})
		}
	} else {
		log.Debugf(format, args...)
	}
}

// Fatal logs a message at level "Fatal" on the standard logger
func Fatal(args ...interface{}) {
	log.Fatal(args...)
}

// Fatalf logs a message at level "Fatal" on the standard logger
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
