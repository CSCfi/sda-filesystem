//go:build linux || darwin
// +build linux darwin

package mountpoint

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/billziss-gh/cgofuse/fuse"
)

func TestCheckMountPoint(t *testing.T) {
	node, err := os.MkdirTemp("", "dir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(node)

	if err = CheckMountPoint(node); err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	}
}

func TestCheckMountPoint_Permissions(t *testing.T) {
	var tests = []struct {
		testname, name string
		mode           int
	}{
		{"NO_READ_PERM", "folder", 0333},
		{"NO_WRITE_PERM", "node", 0555},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			node, err := os.MkdirTemp("", tt.name)

			if err != nil {
				t.Errorf("Failed to create folder: %s", err.Error())
			} else if err = os.Chmod(node, os.FileMode(tt.mode)); err != nil {
				t.Errorf("Changing permission bits failed: %s", err.Error())
			} else if err = CheckMountPoint(node); err == nil {
				t.Error("Function should have returned error")
			}

			os.RemoveAll(node)
		})
	}
}

func TestCheckMountPoint_Not_Dir(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	if err = CheckMountPoint(file.Name()); err == nil {
		t.Error("Function should have returned error")
	}
}

func TestCheckMountPoint_Fail_Stat(t *testing.T) {
	file, err := os.CreateTemp("", "file_parent")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	if err = CheckMountPoint(file.Name() + "/folder"); err == nil {
		t.Error("Function should have returned error")
	}
}

func TestCheckMountPoint_Fail_MkdirAll(t *testing.T) {
	node, err := os.MkdirTemp("", "dir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(node)

	if err = os.Chmod(node, os.FileMode(0555)); err != nil {
		t.Errorf("Changing permission bits failed: %s", err.Error())
	} else if err = CheckMountPoint(node + "/child"); err == nil {
		t.Error("Function should have returned error")
	}
}

func TestCheckMountPoint_Not_Exist(t *testing.T) {
	node, err := os.MkdirTemp("", "dir") // get unique folder name
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	os.RemoveAll(node)       // make sure folder does not exist
	defer os.RemoveAll(node) // if folder was created in function

	if err = CheckMountPoint(node); err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	} else if _, err := os.Stat(node); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

func TestCheckMountPoint_Not_Empty(t *testing.T) {
	node, err := os.MkdirTemp("", "dir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(node)

	if file, err := os.CreateTemp(node, "file"); err != nil {
		t.Errorf("Failed to create file %s: %s", file.Name(), err.Error())
	} else if err = CheckMountPoint(node); err == nil {
		t.Error("Function should have returned error")
	}
}

type Testfs struct {
	fuse.FileSystemBase
}

func (t *Testfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	stat.Mode = fuse.S_IFDIR | 0755
	return 0
}

func (t *Testfs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64, fh uint64) (errc int) {
	return -fuse.EIO
}

func TestCheckMountPoint_Fail_Read(t *testing.T) {
	basepath, err := os.Getwd()
	if err != nil {
		t.Fatalf("Could not retrieve working directory: %s", err.Error())
	}
	node, err := os.MkdirTemp(basepath, "filesystem")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(node)

	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
	}

	testfs := &Testfs{}
	host := fuse.NewFileSystemHost(testfs)
	go host.Mount(node, options)
	defer host.Unmount()

	time.Sleep(2 * time.Second)

	if err = CheckMountPoint(node); err == nil {
		t.Error("Function should have returned error")
	}
}
