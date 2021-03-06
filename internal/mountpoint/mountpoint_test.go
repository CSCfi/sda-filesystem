package mountpoint

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var homeEnv string

func TestMain(m *testing.M) {
	homeEnv = "HOME"
	if runtime.GOOS == "windows" {
		homeEnv = "USERPROFILE"
	}

	origHomeDir := os.Getenv(homeEnv)
	code := m.Run()
	os.Setenv(homeEnv, origHomeDir)
	os.Exit(code)
}

func TestDefaultMountPoint(t *testing.T) {
	origCheckMountPoint := CheckMountPoint
	defer func() { CheckMountPoint = origCheckMountPoint }()

	os.Setenv(homeEnv, filepath.FromSlash("/spirited/away"))
	CheckMountPoint = func(mount string) error { return nil }

	ret, err := DefaultMountPoint()
	if err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	}
	if ret != filepath.FromSlash("/spirited/away/Projects") {
		t.Errorf("Incorrect default mount point\nExpected=%s\nReceived=%s", filepath.FromSlash("/spirited/away/Projects"), ret)
	}
}

func TestDefaultMountPoint_Fail_OS(t *testing.T) {
	origCheckMountPoint := CheckMountPoint
	defer func() { CheckMountPoint = origCheckMountPoint }()

	os.Unsetenv(homeEnv)
	CheckMountPoint = func(mount string) error { return nil }

	ret, err := DefaultMountPoint()
	if err == nil {
		t.Errorf("Function should have returned error")
	}
	if ret != "" {
		t.Errorf("Function should have returned empty mount point")
	}
}

func TestDefaultMountPoint_Fail_Check(t *testing.T) {
	origCheckMountPoint := CheckMountPoint
	defer func() { CheckMountPoint = origCheckMountPoint }()

	os.Setenv(homeEnv, filepath.FromSlash("/the/matrix"))
	checkErr := errors.New("Checking mount point failed")
	CheckMountPoint = func(mount string) error {
		return checkErr
	}

	ret, err := DefaultMountPoint()
	if err == nil {
		t.Errorf("Function should have returned error")
	} else if !errors.Is(err, checkErr) {
		t.Errorf("Function returned incorrect error: %s", err.Error())
	}
	if ret != "" {
		t.Errorf("Function should have returned empty mount point")
	}
}
