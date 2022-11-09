//go:build linux || darwin

package mountpoint

import (
	"fmt"
	"io"
	"os"

	"sda-filesystem/internal/logs"

	"golang.org/x/sys/unix"
)

// CheckMountPoint verifies that the filesystem can be created in directory 'mount'
var CheckMountPoint = func(mount string) error {
	// Verify mount point exists
	info, err := os.Stat(mount)
	if os.IsNotExist(err) {
		logs.Debugf("Mount point %s does not exist, so it will be created", mount)
		if err = os.MkdirAll(mount, 0755); err != nil {
			return fmt.Errorf("Could not create directory %s", mount)
		}
		return nil
	} else if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", mount)
	}

	if unix.Access(mount, unix.W_OK) != nil {
		return fmt.Errorf("You do not have permission to write to folder %s", mount)
	}

	dir, err := os.Open(mount)
	if err != nil {
		return fmt.Errorf("Could not open mount point %s", mount)
	}
	defer dir.Close()

	// Verify dir is empty
	if _, err = dir.ReadDir(1); err != io.EOF {
		if err != nil {
			return fmt.Errorf("Error occurred when trying to read from directory %s: %w", mount, err)
		}
		return fmt.Errorf("Mount point %s must be empty", mount)
	}

	logs.Debugf("Directory %s is a valid mount point", mount)
	return nil
}
