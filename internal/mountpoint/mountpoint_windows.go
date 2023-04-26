package mountpoint

import (
	"fmt"
	"os"
	"path/filepath"

	"sda-filesystem/internal/logs"
)

var CheckMountPoint = func(mount string) error {
	if _, err := os.Stat(mount); !os.IsNotExist(err) {
		logs.Infof("Windows requires that mount point does not exist beforehand so directory %s will be removed", mount)
		if err = os.Remove(mount); err != nil {
			return fmt.Errorf("Removal failed: %w", err)
		}
		return nil
	}

	dir := filepath.Dir(mount)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logs.Debugf("Path to %s does not exist so it will be created", mount)
		if err = os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Could not create a path to directory %s: %w", mount, err)
		}
	}

	return nil
}

func WaitForUpdateSignal(ch chan<- bool) {
}
