// +build linux darwin

package mountpoint

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestCheckMountPoint(t *testing.T) {
	var tests = []struct {
		testname, name string
		dir            bool
		mode           int
	}{
		{
			"OK", "dir", true, 0755,
		},
		{
			"NOT_DIR", "file", false, 0755,
		},
		{
			"NO_READ_PERM", "folder", true, 0333,
		},
		{
			"NO_WRITE_PERM", "folder", true, 0555,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			var node string
			var err error
			if tt.dir {
				node, err = ioutil.TempDir("", tt.name)
			} else {
				var file *os.File
				file, err = ioutil.TempFile("", tt.name)
				node = file.Name()
			}

			if err != nil {
				t.Fatalf("Failed to create file/folder: %s", err.Error())
			}

			if err = os.Chmod(node, os.FileMode(tt.mode)); err != nil {
				t.Fatalf("Changing permission bits failed: %s", err.Error())
			}

			err = CheckMountPoint(node)

			if tt.testname == "OK" {
				if err != nil {
					t.Errorf("Function returned error: %s", err.Error())
				}
			} else if err == nil {
				t.Error("Function should have returned non-nil error")
			}

			os.RemoveAll(node)
		})
	}
}

func TestCheckMountPoint_Not_Empty(t *testing.T) {
	node, err := ioutil.TempDir("", "dir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(node)

	file, err := ioutil.TempFile(node, "file")
	if err != nil {
		t.Fatalf("Failed to create file %q: %s", file.Name(), err.Error())
	}
	if err = CheckMountPoint(node); err == nil {
		t.Error("Function should have returned non-nil error")
	}
}

func TestCheckMountPoint_Not_Exist(t *testing.T) {
	node, err := ioutil.TempDir("", "dir") // get unique folder name
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	os.RemoveAll(node)       // make sure folder does not exist
	defer os.RemoveAll(node) // if folder was created in function

	if err = CheckMountPoint(node); err != nil {
		t.Fatalf("Function should returned error: %s", err.Error())
	}
	if _, err := os.Stat(node); os.IsNotExist(err) {
		t.Fatalf("Directory was not created")
	}
}
