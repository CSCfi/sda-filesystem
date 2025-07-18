package main

import (
	"os"
	"sda-filesystem/internal/airlock"
	"strings"
	"testing"
)

func TestExportSetup(t *testing.T) {
	var tests = []struct {
		testname, args, bucket, filename string
		override                         bool
	}{
		{"OK_1", "-override test-bucket test-file", "test-bucket", "test-file", true},
		{"OK_2", "test-bucket test-file --override", "test-bucket", "test-file", true},
		{"OK_3", "test-bucket-2 test-file", "test-bucket-2", "test-file", false},
		{"OK_3", "test-bucket-2 test-file-2 --override=false", "test-bucket-2", "test-file-2", false},
	}

	origExportPossible := airlock.ExportPossible
	defer func() {
		airlock.ExportPossible = origExportPossible
	}()

	airlock.ExportPossible = func() bool {
		return true
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			bucket, filename = "", ""
			override = false

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			code, err := exportSetup(strings.Split(tt.args, " "))

			os.Stdout = sout
			null.Close()

			switch {
			case err != nil:
				t.Errorf("Returned unexpected error: %s", err.Error())
			case code != 0:
				t.Errorf("Received incorrect status code. Expected=0, received=%d", code)
			case tt.bucket != bucket:
				t.Errorf("Received incorrect bucket. Expected=%s, received=%s", tt.bucket, bucket)
			case tt.filename != filename:
				t.Errorf("Received incorrect file. Expected=%s, received=%s", tt.filename, filename)
			case tt.override != override:
				t.Errorf("Received incorrect override value. Expected=%t, received=%t", tt.override, override)
			}
		})
	}
}

func TestExportSetup_Error(t *testing.T) {
	var tests = []struct {
		testname, args, errStr string
		code                   int
		export                 bool
	}{
		{"FAIL_BAD_ARG_1", "-overrid test-bucket test-file", "", 2, true},
		{"FAIL_BAD_ARG_2", "test-bucket --override", "", 2, true},
		{"FAIL_EXPORT", "test-bucket test-file", "you are not allowed to export files", 0, false},
	}

	origExportPossible := airlock.ExportPossible
	defer func() {
		airlock.ExportPossible = origExportPossible
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			airlock.ExportPossible = func() bool {
				return tt.export
			}

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			code, err := exportSetup(strings.Split(tt.args, " "))

			os.Stdout = sout
			null.Close()

			if code != tt.code {
				t.Errorf("Received incorrect status code. Expected=%d, received=%d", tt.code, code)
			}

			switch {
			case tt.errStr == "":
				if err != nil {
					t.Errorf("Returned unexpected err: %s", err.Error())
				}
			case err == nil:
				t.Error("Function should have returned error")
			case err.Error() != tt.errStr:
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}
