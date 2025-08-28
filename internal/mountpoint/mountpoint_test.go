package mountpoint

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/winfsp/cgofuse/fuse"
)

var homeEnv string

func TestMain(m *testing.M) {
	homeEnv = "HOME"
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

func TestCheckMountPoint(t *testing.T) {
	node := t.TempDir()
	if err := CheckMountPoint(node); err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	}
}

func TestCheckMountPoint_Permissions(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping fuse test for ci docker")
	}

	var tests = []struct {
		testname, name string
		mode           uint32
	}{
		{"NO_READ_PERM", "folder", 0333},
		{"NO_WRITE_PERM", "node", 0555},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			node := t.TempDir()

			if err := os.Chmod(node, os.FileMode(tt.mode)); err != nil {
				t.Errorf("Changing permission bits failed: %s", err.Error())
			} else if err = CheckMountPoint(node); err == nil {
				t.Error("Function should have returned error")
			}
		})
	}

}

func TestCheckMountPoint_Not_Dir(t *testing.T) {
	node := t.TempDir()
	filename := node + "/file.txt"
	err := os.WriteFile(filename, []byte("hello world"), 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}

	if err = CheckMountPoint(filename); err == nil {
		t.Error("Function should have returned error")
	}
}

func TestCheckMountPoint_Fail_Stat(t *testing.T) {
	node := t.TempDir()
	filename := node + "/parent-file"
	err := os.WriteFile(filename, []byte("hello world"), 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}

	if err = CheckMountPoint(filename + "/folder"); err == nil {
		t.Error("Function should have returned error")
	}
}

func TestCheckMountPoint_Fail_MkdirAll(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping fuse test for ci docker")
	}

	node := t.TempDir()
	if err := os.Chmod(node, os.FileMode(0555)); err != nil {
		t.Errorf("Changing permission bits failed: %s", err.Error())
	} else if err = CheckMountPoint(node + "/child"); err == nil {
		t.Error("Function should have returned error")
	}
}

func TestCheckMountPoint_Not_Exist(t *testing.T) {
	node := t.TempDir()
	os.RemoveAll(node) // make sure folder does not exist

	if err := CheckMountPoint(node); err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	} else if _, err := os.Stat(node); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

func TestCheckMountPoint_Not_Empty(t *testing.T) {
	node := t.TempDir()
	filename := node + "/file.txt"
	if err := os.WriteFile(filename, []byte("hello world"), 0600); err != nil {
		t.Errorf("Failed to create file %s: %s", filename, err.Error())
	} else if err = CheckMountPoint(node); err == nil {
		t.Error("Function should have returned error")
	}
}

type Testfs struct {
	fuse.FileSystemBase
}

func (t *Testfs) Getattr(_ string, stat *fuse.Stat_t, _ uint64) (errc int) {
	stat.Mode = fuse.S_IFDIR | 0755

	return 0
}

func (t *Testfs) Readdir(_ string,
	_ func(_ string, _ *fuse.Stat_t, _ int64) bool,
	_ int64, _ uint64) (errc int) {
	return -fuse.EIO
}

func TestCheckMountPoint_Fail_Read(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping fuse test for ci docker")
	}

	node := t.TempDir()

	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
	}

	origUnmount := Unmount
	Unmount = func(mount string) error {
		return nil
	}

	testfs := &Testfs{}
	host := fuse.NewFileSystemHost(testfs)
	go host.Mount(node, options)

	time.Sleep(2 * time.Second)

	if err := CheckMountPoint(node); err == nil {
		t.Error("Function should have returned error")
	}

	Unmount = origUnmount
	_ = Unmount(node)
}
