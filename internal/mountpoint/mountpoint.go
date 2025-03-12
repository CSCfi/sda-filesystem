package mountpoint

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultMountPoint constructs a path to the user's home directory for mounting FUSE
var DefaultMountPoint = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find user's home directory: %w", err)
	}
	p := filepath.Join(home, "Projects")
	if err = CheckMountPoint(p); err != nil {
		return "", fmt.Errorf("cannot create Data Gateway in default directory: %w", err)
	}

	return p, nil
}
