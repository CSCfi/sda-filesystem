package mountpoint

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestMounPoint(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	dir := "/spirited/away"

	origHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", dir)

	defer func() {
		logrus.StandardLogger().ExitFunc = nil
		os.Setenv("HOME", origHomeDir)
	}()

	ret := DefaultMountPoint()
	if fatal {
		t.Fatal("Function called Exit()")
	}
	if ret != dir+"/Projects" {
		t.Fatalf("Incorrect mount point. Expected %q, got %q", dir+"/Projects", ret)
	}
}

func TestMounPoint_Fail(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	origHomeDir := os.Getenv("HOME")
	os.Unsetenv("HOME")

	defer func() {
		logrus.StandardLogger().ExitFunc = nil
		os.Setenv("HOME", origHomeDir)
	}()

	_ = DefaultMountPoint()
	if !fatal {
		t.Fatal("Function should have called Exit()")
	}
}
