package mountpoint

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestMounPoint(t *testing.T) {
	dir := "/spirited/away"

	origHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", dir)

	defer func() {
		logrus.StandardLogger().ExitFunc = nil
		os.Setenv("HOME", origHomeDir)
	}()

	ret, err := DefaultMountPoint()
	if err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
	if ret != dir+"/Projects" {
		t.Fatalf("Incorrect mount point. Expected %q, got %q", dir+"/Projects", ret)
	}
}

func TestMounPoint_Fail(t *testing.T) {
	origHomeDir := os.Getenv("HOME")
	os.Unsetenv("HOME")

	defer func() {
		logrus.StandardLogger().ExitFunc = nil
		os.Setenv("HOME", origHomeDir)
	}()

	ret, err := DefaultMountPoint()
	if err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
	if ret != "" {
		t.Fatalf("Function should have returned empty mount point, got %q", ret)
	}
}
