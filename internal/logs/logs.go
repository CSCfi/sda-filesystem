package logs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

var signal func(string, string) = nil

func SetSignal(fn func(string, string)) {
	signal = fn
}

func Wrapper(err error) (string, error) {
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		return strings.TrimRight(strings.TrimSuffix(err.Error(), unwrapped.Error()), ": "), unwrapped
	}
	return err.Error(), nil
}

func structureError(err error) string {
	fullError := ""
	for i := 0; err != nil; i++ {
		var str string
		str, err = Wrapper(err)
		fullError += strings.Repeat("\t", i) + str + "\n"
	}
	return fullError
}

func Error(err error) {
	if signal != nil {
		signal("ERRO", structureError(err))
	} else {
		log.Error(err)
	}
}

func Errorf(format string, args ...interface{}) {
	if signal != nil {
		err := fmt.Errorf(format, args...)
		signal("ERRO", structureError(err))
	} else {
		log.Errorf(format, args...)
	}
}

func Warning(args ...interface{}) {
	if signal != nil {
		signal("WARN", fmt.Sprint(args...))
	} else {
		log.Warning(args...)
	}
}

func Warningf(format string, args ...interface{}) {
	if signal != nil {
		signal("WARN", fmt.Sprintf(format, args...))
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
	log.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func init() {
	// Configure Log Text Formatter
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)
}
