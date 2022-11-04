package logs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// signal sends a new log to the LogModel
var signal func(int, []string) = nil

var log *logrus.Logger

var levelMap = map[string]logrus.Level{
	"debug":   logrus.DebugLevel,
	"info":    logrus.InfoLevel,
	"warning": logrus.WarnLevel,
	"error":   logrus.ErrorLevel,
}

// SetSignal initializes 'signal', which sends logs to LogModel
func SetSignal(fn func(int, []string)) {
	signal = fn
}

// SetLevel sets the logging level
var SetLevel = func(level string) {
	if logrusLevel, ok := levelMap[strings.ToLower(level)]; ok {
		log.SetLevel(logrusLevel)
		return
	}

	Warningf("-loglevel=%s is not supported, possible values are {debug,info,warning,error}, setting fallback loglevel to 'info'", level)
	log.SetLevel(logrus.InfoLevel)
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
var StructureError = func(err error) []string {
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
		signal(int(logrus.ErrorLevel), StructureError(err))
	} else {
		log.Error(err)
	}
}

// Errorf logs a message at level "Error" either on the standard logger or in the GUI
func Errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	if signal != nil {
		signal(int(logrus.ErrorLevel), StructureError(err))
	} else {
		log.Error(err)
	}
}

// Warning logs a message at level "Warning" either on the standard logger or in the GUI
func Warning(err error) {
	if signal != nil {
		signal(int(logrus.WarnLevel), StructureError(err))
	} else {
		log.Warning(err.Error())
	}
}

// Warningf logs a message at level "Warning" either on the standard logger or in the GUI
var Warningf = func(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	if signal != nil {
		signal(int(logrus.WarnLevel), StructureError(err))
	} else {
		log.Warningf(err.Error())
	}
}

// Info logs a message at level "Info" either on the standard logger or in the GUI
func Info(args ...any) {
	if signal != nil {
		signal(int(logrus.InfoLevel), []string{fmt.Sprint(args...)})
	} else {
		log.Info(args...)
	}
}

// Infof logs a message at level "Info" either on the standard logger or in the GUI
func Infof(format string, args ...any) {
	if signal != nil {
		signal(int(logrus.InfoLevel), []string{fmt.Sprintf(format, args...)})
	} else {
		log.Infof(format, args...)
	}
}

// Debug logs a message at level "Debug" either on the standard logger or in the GUI
func Debug(args ...any) {
	if signal != nil {
		if log.IsLevelEnabled(logrus.DebugLevel) {
			signal(int(logrus.DebugLevel), []string{fmt.Sprint(args...)})
		}
	} else {
		log.Debug(args...)
	}
}

// Debugf logs a message at level "Debug" either on the standard logger or in the GUI
func Debugf(format string, args ...any) {
	if signal != nil {
		if log.IsLevelEnabled(logrus.DebugLevel) {
			signal(int(logrus.DebugLevel), []string{fmt.Sprintf(format, args...)})
		}
	} else {
		log.Debugf(format, args...)
	}
}

// Fatal logs a message at level "Fatal" on the standard logger
func Fatal(args ...any) {
	log.Fatal(args...)
}

// Fatalf logs a message at level "Fatal" on the standard logger
func Fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}

func init() {
	log = logrus.New()
	// Configure Log Text Formatter
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})
}
