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

	Warningf = func(format string, args ...interface{}) {}

	for _, tt := range tests {
		testname := strings.ToUpper(tt.input)
		t.Run(testname, func(t *testing.T) {
			SetLevel(tt.input)
			if log.GetLevel() != tt.level {
				t.Errorf("%s test failed. Got %q, expected %q", testname, log.GetLevel().String(), tt.level.String())
			}
		})
	}
}

func TestWrapper(t *testing.T) {
	errs := []string{"Original problem", "Fix me", "Whaaat???", "Another error", "Error 1"}
	delim := ": "
	fullError := fmt.Errorf("%s", errs[0])

	for _, e := range errs[1:] {
		fullError = fmt.Errorf("%s%s%w", e, delim, fullError)
	}

	var eStr string
	for i := range errs {
		eStr, fullError = Wrapper(fullError)
		if eStr != errs[len(errs)-1-i] {
			t.Fatalf("Wrapper test failed. Got %q, expected %q", eStr, errs[len(errs)-1-i])
		}
	}

	if fullError != nil {
		t.Fatal("Wrapper did not return nil error in the end")
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
		t.Errorf("StructureError test failed.\nGot %q\nExpected %q", errs2, errs)
	}
}

func TestError(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "This is an error"
	StructureError = func(err error) []string {
		return []string{message}
	}

	Error(errors.New(message))

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.ErrorLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.ErrorLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message.\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	Error(errors.New(message))

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.ErrorLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.ErrorLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
	}
}

func TestErrorf(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "This is an unexpected error: Where am I?"
	StructureError = func(err error) []string {
		return []string{message}
	}

	Errorf("This is an %s error: %w", "unexpected", errors.New("Where am I?"))

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.ErrorLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.ErrorLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}
	Errorf("This is an %s error: %w", "unexpected", errors.New("Where am I?"))

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.ErrorLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.ErrorLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
	}
}

func TestWarning(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "Tomorrow snow will fall"
	StructureError = func(err error) []string {
		return []string{message}
	}

	Warning(errors.New(message))

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.WarnLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.WarnLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}
	Warning(errors.New(message))

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.WarnLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.WarnLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
	}
}

func TestWarningf(t *testing.T) {
	origStructureError := StructureError
	defer func() {
		StructureError = origStructureError
		testHook.Reset()
	}()

	signal = nil
	message := "Tomorrow the sun will shine: Remember sunscreen"
	StructureError = func(err error) []string {
		return []string{message}
	}

	Warningf("%s the sun will shine: %w", "Tomorrow", errors.New("Remember sunscreen"))

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.WarnLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.WarnLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}
	Warningf("%s the sun will shine: %w", "Tomorrow", errors.New("Remember sunscreen"))

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.WarnLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.WarnLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
	}
}

func TestInfo(t *testing.T) {
	signal = nil
	message := "I am grand, and you?"
	Info("I am ", "grand,", " and you?")
	defer testHook.Reset()

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.InfoLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.InfoLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}
	Info("I am ", "grand,", " and you?")

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.InfoLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.InfoLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
	}
}

func TestInfof(t *testing.T) {
	signal = nil
	message := "100 students barged in the classroom"
	Infof("%d students barged in the %s", 100, "classroom")
	defer testHook.Reset()

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.InfoLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.InfoLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}
	Infof("%d students barged in the %s", 100, "classroom")

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.InfoLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.InfoLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
	}
}

func TestDebug(t *testing.T) {
	signal = nil
	message := "Why did a thing happen? I don't know"
	Debug("Why did a ", "thing happen? ", "I don't know")
	defer testHook.Reset()

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.DebugLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.DebugLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	log.SetLevel(logrus.InfoLevel)
	Debug("Why did a ", "thing happen? ", "I don't know")

	if level != 0 || strs != nil {
		t.Fatal("With loglevel info, signal should not have been called")
	}

	log.SetLevel(logrus.DebugLevel)
	Debug("Why did a ", "thing happen? ", "I don't know")

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.DebugLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.DebugLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
	}
}

func TestDebugf(t *testing.T) {
	signal = nil
	message := "10 ducks crossed the road. And?"
	Debugf("%d ducks crossed the road. %s", 10, "And?")
	defer testHook.Reset()

	if len(testHook.Entries) != 1 {
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.DebugLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.DebugLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}

	var level int = 0
	var strs []string = nil
	signal = func(i int, s []string) {
		level, strs = i, s
	}

	log.SetLevel(logrus.InfoLevel)
	Debugf("%d ducks crossed the road. %s", 10, "And?")

	if level != 0 || strs != nil {
		t.Fatal("With loglevel info, signal should not have been called")
	}

	log.SetLevel(logrus.DebugLevel)
	Debugf("%d ducks crossed the road. %s", 10, "And?")

	if len(testHook.Entries) != 1 {
		t.Fatal("Logger with signal should not have logged to stdout")
	}
	if level != int(logrus.DebugLevel) {
		t.Fatalf("Logger with signal logged at incorrect level. Expected value %d, got %d", int(logrus.DebugLevel), level)
	}
	if !reflect.DeepEqual(strs, []string{message}) {
		t.Fatalf("Logger with signal gave incorrect message\nExpected: %v\nGot: %v", []string{message}, strs)
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
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.FatalLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.FatalLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
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
		t.Fatalf("Logger did not make the correct amount of entries. Expected 1, got %d", len(testHook.Entries))
	}
	if testHook.LastEntry().Level != logrus.FatalLevel {
		t.Fatalf("Logger logged at incorrect level. Expected %q, got %q",
			logrus.FatalLevel.String(), testHook.LastEntry().Level.String())
	}
	if testHook.LastEntry().Message != message {
		t.Fatalf("Logger displayed incorrect message\nExpected: %q\nGot: %q", message, testHook.LastEntry().Message)
	}
}
