package mountpoint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hectane/go-acl"
)

func TestCheckMountPoint(t *testing.T) {
	node := t.TempDir()

	subnode := node + string(os.PathSeparator) + "subdir"
	if err := os.Mkdir(subnode, 0755); err != nil {
		t.Errorf("Failed to create folder: %s", err.Error())
	}

	if err := CheckMountPoint(node); err == nil {
		t.Error("Function did not return error when folder was not empty")
	}

	if err := CheckMountPoint(subnode); err != nil {
		t.Errorf("Function returned error for empty folder: %s", err.Error())
	}

	if err := CheckMountPoint(node); err != nil {
		t.Errorf("Function returned error when folder did not exist: %s", err.Error())
	}

	if err := CheckMountPoint(subnode); err != nil {
		t.Errorf("Function returned error when path to folder did not exist: %s", err.Error())
	}
}

func TestCheckMountPoint_Fail_MkdirAll(t *testing.T) {
	node := t.TempDir()

	if err := acl.Chmod(node, 0600); err != nil {
		t.Errorf("Changing permission bits failed: %s", err.Error())
	} else if err := CheckMountPoint(node + filepath.FromSlash("/child/grandchild")); err == nil {
		t.Error("Function should have returned error")
	}
}
