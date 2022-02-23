package mountpoint

import (
	"github.com/hectane/go-acl"
	"io/ioutil"
	"os"
	"testing"
)

func TestCheckMountPoint(t *testing.T) {
	node, err := ioutil.TempDir("", "dir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(node)

	subnode := node + string(os.PathSeparator) + "subdir"
	if err = os.Mkdir(subnode, 0755); err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}

	if err = CheckMountPoint(node); err == nil {
		t.Fatal("Function did not return error when folder was not empty")
	}

	if err = CheckMountPoint(subnode); err != nil {
		t.Fatalf("Function returned error for empty folder: %s", err.Error())
	}

	if err = CheckMountPoint(node); err != nil {
		t.Fatalf("Function returned error when folder did not exist: %s", err.Error())
	}

	if err = CheckMountPoint(subnode); err != nil {
		t.Fatalf("Function returned error when path to folder did not exist: %s", err.Error())
	}
}

func TestCheckMountPoint_Fail_MkdirAll(t *testing.T) {
	node, err := ioutil.TempDir("", "dir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(node)

	if err = acl.Chmod(node, 0444); err != nil {
		t.Errorf("Changing permission bits failed: %s", err.Error())
	} else if err = CheckMountPoint(node + filepath.FromSlash("/child/grandchild")); err == nil {
		t.Error("Function should have returned non-nil error")
	}
}
