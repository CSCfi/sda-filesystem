package mountpoint

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestCheckMountPoint(t *testing.T) {
	node, err := ioutil.TempDir("", "dir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}

	if err = CheckMountPoint(node); err == nil {
		t.Fatal("Function did not return error when folder existed")
	}

	os.RemoveAll(node)

	if err = CheckMountPoint(node); err != nil {
		t.Fatalf("Function returned error when folder did not exist: %s", err.Error())
	}
}
