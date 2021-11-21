package logs

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

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

	origInfof := Infof
	Warningf = func(format string, args ...interface{}) {}
	defer func() { Warningf = origInfof }()

	for _, tt := range tests {
		testname := strings.ToUpper(tt.input)
		t.Run(testname, func(t *testing.T) {
			SetLevel(tt.input)
			if logrus.GetLevel() != tt.level {
				t.Errorf("%s test failed. Got %q, expected %q", testname, logrus.GetLevel().String(), tt.level.String())
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
