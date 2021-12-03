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
		return "", fmt.Errorf("Could not find user home directory: %w", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Projects")
	if err = CheckMountPoint(p); err != nil {
		return "", fmt.Errorf("Cannot create filesystem in default directory %q: %w", p, err)
	}
	return p, nil
}
