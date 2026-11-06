package main

import (
	"testing"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/mountpoint"
)

func TestImportSetup(t *testing.T) {
	defaultMount := "default_dir"

	var tests = []struct {
		testname, mount string
	}{
		{"OK_1", "/hello"},
		{"OK_2", "/goodbye"},
		{"OK_3", "/hi/hello"},
		{"OK_4", ""},
	}

	origDefaultMountPoint := mountpoint.DefaultMountPoint
	origCheckMountPoint := mountpoint.CheckMountPoint
	origSDConnectEnabled := api.SDConnectEnabled

	defer func() {
		mountpoint.DefaultMountPoint = origDefaultMountPoint
		mountpoint.CheckMountPoint = origCheckMountPoint
		api.SDConnectEnabled = origSDConnectEnabled
	}()

	var testMount string

	mountpoint.DefaultMountPoint = func() (string, error) {
		return defaultMount, nil
	}
	mountpoint.CheckMountPoint = func(mount string) error {
		testMount = mount

		return nil
	}
	api.SDConnectEnabled = func() bool {
		return true
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			mount, testMount = "", ""

			code, err := importSetup([]string{"-mount=" + tt.mount})

			switch {
			case err != nil:
				t.Errorf("Returned unexpected error: %s", err.Error())
			case code != 0:
				t.Errorf("Received incorrect status code. Expected=0, received=%d", code)
			case tt.mount == "" && mount != defaultMount:
				t.Errorf("Expected default mount point %s, received=%s", defaultMount, mount)
			case tt.mount != testMount:
				t.Errorf("CheckMountPoint() received incorrect mount point. Expected=%s, received=%s", tt.mount, testMount)
			}
		})
	}
}

func TestImportSetup_Error(t *testing.T) {
	var tests = []struct {
		testname, arg, errStr              string
		enabled                            bool
		code                               int
		checkMountError, defaultMountError error
	}{
		{
			"FAIL_DEFAULT_MOUNT", "-mount=", errExpected.Error(),
			true, 0, nil, errExpected,
		},
		{
			"FAIL_CHECK_MOUNT", "-mount=/bad/directory", errExpected.Error(),
			true, 0, errExpected, nil,
		},
		{
			"FAIL_BAD_ARG", "-money=euro", "",
			true, 2, nil, nil,
		},
	}

	origDefaultMountPoint := mountpoint.DefaultMountPoint
	origCheckMountPoint := mountpoint.CheckMountPoint
	origSDConnectEnabled := api.SDConnectEnabled

	defer func() {
		mountpoint.DefaultMountPoint = origDefaultMountPoint
		mountpoint.CheckMountPoint = origCheckMountPoint
		api.SDConnectEnabled = origSDConnectEnabled
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			mountpoint.DefaultMountPoint = func() (string, error) {
				return "", tt.defaultMountError
			}
			mountpoint.CheckMountPoint = func(mount string) error {
				return tt.checkMountError
			}
			api.SDConnectEnabled = func() bool {
				return tt.enabled
			}

			code, err := importSetup([]string{tt.arg})
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
