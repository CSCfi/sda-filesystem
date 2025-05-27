//go:build linux || darwin

package mountpoint

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"sda-filesystem/internal/logs"

	"golang.org/x/sys/unix"
)

// CheckMountPoint verifies that the filesystem can be created in directory 'mount'
var CheckMountPoint = func(mount string) error {
	_ = Unmount(mount) // In case previous run did not unmount the directory

	// Verify mount point exists
	info, err := os.Stat(mount)
	if os.IsNotExist(err) {
		logs.Debugf("Mount point %s does not exist, so it will be created", mount)
		if err = os.MkdirAll(mount, 0755); err != nil {
			return fmt.Errorf("could not create directory %s", mount)
		}

		return nil
	} else if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", mount)
	}

	if unix.Access(mount, unix.W_OK) != nil {
		return fmt.Errorf("you do not have permission to write to folder %s", mount)
	}

	dir, err := os.Open(mount)
	if err != nil {
		return fmt.Errorf("could not open mount point %s", mount)
	}
	defer dir.Close()

	// Verify dir is empty
	if _, err = dir.ReadDir(1); err != io.EOF {
		if err != nil {
			return fmt.Errorf("error occurred when trying to read from directory %s: %w", mount, err)
		}

		return fmt.Errorf("mount point %s must be empty", mount)
	}

	logs.Debugf("Directory %s is a valid mount point", mount)

	return nil
}

func WaitForUpdateSignal(ch chan<- []string) {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGUSR2)
	for {
		<-s
		ch <- []string{"update"}
	}
}

var Unmount = func(mount string) error {
	logs.Debugf("Starting to unmount %s", mount)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("fusermount3", "-u", mount)
	case "darwin":
		cmd = exec.Command("diskutil", "unmount", mount)
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("unmounting filesystem failed: %w", err)
	}
	logs.Debug("Filesystem unmounted")

	return nil
}

/*
func BytesAvailable(dir string) (uint64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)

	return stat.Bavail * uint64(stat.Bsize), err
}
*/
