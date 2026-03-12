package main

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/api"
)

func TestExportSetup(t *testing.T) {
	var tests = []struct {
		testname, args, prefix, email string
		selection                     []string
		override, findata             bool
		metadata                      map[string]string
	}{
		{
			"OK_1",
			"-override test-bucket test-file",
			"test-bucket", "",
			[]string{"test-file"},
			true, false, make(map[string]string),
		},
		{
			"OK_2",
			"test-bucket test-file --override",
			"test-bucket", "",
			[]string{"test-file"},
			true, false, make(map[string]string),
		},
		{
			"OK_3",
			"test-bucket-2 test-file test-dir",
			"test-bucket-2", "",
			[]string{"test-file", "test-dir"},
			false, false, make(map[string]string),
		},
		{
			"OK_3",
			"test-bucket-2 test-file-2 --override=false",
			"test-bucket-2", "",
			[]string{"test-file-2"},
			false, false, make(map[string]string),
		},
		{
			"OK_4",
			"--email= test-bucket-2 test-file test-file-2",
			"test-bucket-2", "",
			[]string{"test-file", "test-file-2"},
			false, false, make(map[string]string),
		},
		{
			"OK_5",
			"-email=matti.meikalainen@gmail.com --journal-number=123 test-bucket-3 test-file-3 --override",
			"test-bucket-3", "",
			[]string{"test-file-3"},
			true, true, map[string]string{"author_email": "matti.meikalainen@gmail.com", "journal_number": "123"},
		},
		{
			"OK_6",
			"--email= -journal-number 123 test-bucket-3 test-file --email matti.meikalainen@gmail.com",
			"test-bucket-3", "maija.meikalainen@gmail.com",
			[]string{"test-file"},
			false, true, map[string]string{"author_email": "matti.meikalainen@gmail.com", "journal_number": "123"},
		},
		{
			"OK_7",
			"test-bucket-4 test-file test-file-2 test-file-3 --journal-number=123",
			"test-bucket-4", "maija.meikalainen@gmail.com",
			[]string{"test-file", "test-file-2", "test-file-3"},
			false, true, map[string]string{"author_email": "maija.meikalainen@gmail.com", "journal_number": "123"},
		},
	}

	origExportPossible := airlock.ExportPossible
	origFindataUpload := api.FindataUpload
	origGetUserEmail := api.GetUserEmail
	defer func() {
		airlock.ExportPossible = origExportPossible
		api.FindataUpload = origFindataUpload
		api.GetUserEmail = origGetUserEmail
	}()

	airlock.ExportPossible = func() bool {
		return true
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			t.Cleanup(func() {
				exportPrefix, selection = "", []string{}
				override = false
				metadata = make(map[string]string)
			})

			api.FindataUpload = func() bool {
				return tt.findata
			}
			api.GetUserEmail = func() string {
				return tt.email
			}

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
			case tt.prefix != exportPrefix:
				t.Errorf("Received incorrect prefix. Expected=%s, received=%s", tt.prefix, exportPrefix)
			case !reflect.DeepEqual(tt.selection, selection):
				t.Errorf("Received incorrect selection. Expected=%v, received=%v", tt.selection, selection)
			case tt.override != override:
				t.Errorf("Received incorrect override value. Expected=%t, received=%t", tt.override, override)
			case !reflect.DeepEqual(tt.metadata, metadata):
				t.Errorf("Received incorrect metadata\nExpected=%v\nReceived=%v", tt.metadata, metadata)
			}
		})
	}
}

func TestExportSetup_Error(t *testing.T) {
	var tests = []struct {
		testname, args, errStr string
		code                   int
		export, findata        bool
	}{
		{
			"FAIL_BAD_ARG_1",
			"-overrid test-bucket test-file", "",
			2, true, false,
		},
		{
			"FAIL_BAD_ARG_2",
			"test-bucket --override", "",
			2, true, false,
		},
		{
			"FAIL_BAD_ARG_3",
			"test-bucket test-file", "invalid email argument \"\": mail: no address",
			2, true, true,
		},
		{
			"FAIL_BAD_ARG_4",
			"test-bucket test-file -email=matti.meikalainen@csc.fi --email", "invalid email argument \"\": mail: no address",
			2, true, true,
		},
		{
			"FAIL_BAD_ARG_5",
			"test-bucket test-file -email=matti.meikalainen@csc.fi", "missing journal number argument",
			2, true, true,
		},
		{
			"FAIL_EXPORT",
			"test-bucket test-file test-folder", "you are not allowed to export files",
			0, false, false,
		},
	}

	origExportPossible := airlock.ExportPossible
	origFindataUpload := api.FindataUpload
	defer func() {
		airlock.ExportPossible = origExportPossible
		api.FindataUpload = origFindataUpload
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			airlock.ExportPossible = func() bool {
				return tt.export
			}
			api.FindataUpload = func() bool {
				return tt.findata
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
