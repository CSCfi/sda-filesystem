package mountpoint

import (
	"fmt"
	"os"
)

var CheckMountPoint = func(mount string) error {
	if _, err := os.Stat(mount); !os.IsNotExist(err) {
		return fmt.Errorf("Mount point %q already exists, remove the directory or use another mount point", mount)
	}
	return nil
}
