package logs

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

var testHook *test.Hook

func TestMain(m *testing.M) {
	origLog := log
	log, testHook = test.NewNullLogger()
	log.SetLevel(logrus.DebugLevel)

	code := m.Run()

	log = origLog
	signal = nil

	os.Exit(code)
}

func TestSetSignal(t *testing.T) {
	defer func() { signal = nil }()

	called := false
	testSignal := func(i int, s []string) {
		called = true
	}

	SetSignal(testSignal)

	if called {
		t.Error("SetSignal() should not have called signal")
	}

	signal(0, []string{})

	if !called {
		t.Error("Signal was not assigned correctly")
	}
}

func TestSetLevel(t *testing.T) {
	var tests = []struct {
		input string
		level logrus.Level
	}{
		{"error", logrus.ErrorLevel},
		{"warning", logrus.WarnLevel},
		{"info", logrus.InfoLevel},
		{"debug", logrus.DebugLevel},
		{"test", logrus.InfoLevel},
		{"warn", logrus.InfoLevel},
	}

	origLoggingLevel := log.GetLevel()
	origWarningf := Warningf

	defer func() {
		log.SetLevel(origLoggingLevel)
		Warningf = origWarningf
	}()

	Warningf = func(format string, args ...any) {}

	for _, tt := range tests {
		testname := strings.ToUpper(tt.input)
		t.Run(testname, func(t *testing.T) {
			SetLevel(tt.input)
			if log.GetLevel() != tt.level {
				t.Errorf("%s test failed. Expected=%s, received=%s", testname, tt.level.String(), log.GetLevel().String())
			}
		})
	}
}

func TestWrapper(t *testing.T) {
	errs := []string{"Original problem", "Fix me", "Whaaat???", "Another error", "Error 1"}
	fullError := fmt.Errorf("%s", errs[0])

	for _, e := range errs[1:] {
		fullError = fmt.Errorf("%s: %w", e, fullError)
	}

	var eStr string
	for i := range errs {
		eStr, fullError = Wrapper(fullError)
		if eStr != errs[len(errs)-1-i] {
			t.Fatalf("Wrapper test failed. Expected=%s, received=%s", errs[len(errs)-1-i], eStr)
		}
	}

	if fullError != nil {
		t.Fatal("Wrapper did not return nil in the end")
	}
}

func TestStructureError(t *testing.T) {
	errs := []string{"Work and no play", "Tomorrow is Saturday", "How are you?", "knock knock", "Errorrrrr"}
	delim := ": "
	fullError := fmt.Errorf("%s", errs[0])

	for _, e := range errs[1:] {
		fullError = fmt.Errorf("%s%s%w", e, delim, fullError)
	}

	// Reversing errs
	for i, j := 0, len(errs)-1; i < j; i, j = i+1, j-1 {
		errs[i], errs[j] = errs[j], errs[i]
	}

	if errs2 := StructureError(fullError); !reflect.DeepEqual(errs, errs2) {
		t.Errorf("StructureError test failed\nExpected=%s\nReceived=%s", errs, errs2)
	}
}

func TestError(t *testing.T) {
	origStructureError := StructureError
	StructureError = func(err error) []string { return nil }
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "This is an error"

	Error(errors.New(message))

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.ErrorLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.ErrorLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestError_Signal(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	message := "This is another error"
	StructureError = func(err error) []string {
		return []string{err.Error()}
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	Error(errors.New(message))

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.ErrorLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.ErrorLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestErrorf(t *testing.T) {
	origStructureError := StructureError
	StructureError = func(err error) []string { return nil }
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "This is an unexpected error: Where am I?"

	Errorf("This is an %s error: %w", "unexpected", errors.New("Where am I?"))

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.ErrorLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.ErrorLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestErrorf_Signal(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	message := "This is an unexpected error: Who are you?"
	StructureError = func(err error) []string {
		return []string{err.Error()}
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	Errorf("This is an %s error: %w", "unexpected", errors.New("Who are you?"))

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.ErrorLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.ErrorLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestWarning(t *testing.T) {
	origStructureError := StructureError
	StructureError = func(err error) []string { return nil }
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "Tomorrow snow will fall"

	Warning(errors.New(message))

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.WarnLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.WarnLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestWarning_Signal(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	message := "Tomorrow snow will fall"
	StructureError = func(err error) []string {
		return []string{err.Error()}
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	Warning(errors.New(message))

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.WarnLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.WarnLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestWarningf(t *testing.T) {
	origStructureError := StructureError
	StructureError = func(err error) []string { return nil }
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "Tomorrow the sun will shine: Remember sunscreen"

	Warningf("%s the sun will shine: %w", "Tomorrow", errors.New("Remember sunscreen"))

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.WarnLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.WarnLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestWarningf_Signal(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	message := "Tomorrow the sun will not shine: It is the end of days"
	StructureError = func(err error) []string {
		return []string{err.Error()}
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	Warningf("%s the sun will not shine: %w", "Tomorrow", errors.New("It is the end of days"))

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.WarnLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.WarnLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestInfo(t *testing.T) {
	defer testHook.Reset()
	signal = nil
	message := "I am grand, and you?"

	Info("I am ", "grand,", " and you?")

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.InfoLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.InfoLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestInfo_Signal(t *testing.T) {
	defer testHook.Reset()
	message := "I am grand, and you?"

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	Info("I am ", "grand,", " and you?")

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.InfoLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.InfoLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestInfof(t *testing.T) {
	defer testHook.Reset()
	signal = nil
	message := "100 students barged in the classroom"

	Infof("%d students barged in the %s", 100, "classroom")

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.InfoLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.InfoLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestInfof_Signal(t *testing.T) {
	defer testHook.Reset()
	message := "99 students barged in the classroom"

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	Infof("%d students barged in the %s", 99, "classroom")

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.InfoLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.InfoLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestDebug(t *testing.T) {
	defer testHook.Reset()
	signal = nil
	message := "Why did a thing happen? I don't know"

	Debug("Why did a ", "thing happen? ", "I don't know")

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.DebugLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.DebugLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestDebug_Signal(t *testing.T) {
	defer testHook.Reset()
	message := "When did this happen? I don't know"

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	log.SetLevel(logrus.InfoLevel)
	Debug("When did ", "this happen? ", "I don't know")

	if level != 0 || strs != nil {
		t.Error("With loglevel=info, signal() should not have been called")
		return
	}

	log.SetLevel(logrus.DebugLevel)
	Debug("When did ", "this happen? ", "I don't know")

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.DebugLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.DebugLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestDebugf(t *testing.T) {
	defer testHook.Reset()
	signal = nil
	message := "10 ducks crossed the road. And?"

	Debugf("%d ducks crossed the road. %s", 10, "And?")

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.DebugLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.DebugLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestDebugf_Signal(t *testing.T) {
	defer testHook.Reset()
	message := "8 dogs crossed the road. And?"

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	log.SetLevel(logrus.InfoLevel)
	Debugf("%d dogs crossed the road. %s", 8, "And?")

	if level != 0 || strs != nil {
		t.Error("With loglevel=info, signal() should not have been called")
		return
	}

	log.SetLevel(logrus.DebugLevel)
	Debugf("%d dogs crossed the road. %s", 8, "And?")

	if len(testHook.Entries) != 0 {
		t.Error("Logger with signal should not have logged to stdout")
	} else if level != int(logrus.DebugLevel) {
		t.Errorf("Logger with signal logged at incorrect level. Expected=%d, received=%d", int(logrus.DebugLevel), level)
	} else if !reflect.DeepEqual(strs, []string{message}) {
		t.Errorf("Logger with signal gave incorrect message\nExpected=%v\nReceived=%v", []string{message}, strs)
	}
}

func TestFatal(t *testing.T) {
	log.ExitFunc = func(int) {}
	defer func() {
		log.ExitFunc = nil
		testHook.Reset()
	}()

	message := "Too late. All 5 programs are dead"
	Fatal("Too late. ", "All ", 5, " programs are dead")

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.FatalLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.FatalLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}

func TestFatalf(t *testing.T) {
	log.ExitFunc = func(int) {}
	defer func() {
		log.ExitFunc = nil
		testHook.Reset()
	}()

	message := "No! This cannot be happening! Football is cancelled"
	Fatalf("No! This cannot be happening! %s is cancelled", "Football")

	if len(testHook.Entries) != 1 {
		t.Errorf("Logger did not make the correct amount of entries. Expected=1, received=%d", len(testHook.Entries))
	} else if testHook.LastEntry().Level != logrus.FatalLevel {
		t.Errorf("Logger logged at incorrect level. Expected=%s, received=%s",
			logrus.FatalLevel.String(), testHook.LastEntry().Level.String())
	} else if testHook.LastEntry().Message != message {
		t.Errorf("Logger displayed incorrect message\nExpected=%s\nReceived=%s", message, testHook.LastEntry().Message)
	}
}
