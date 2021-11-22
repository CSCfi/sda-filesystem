package mountpoint

import (
	"os"
	"path/filepath"
	"sda-filesystem/internal/logs"
)

// DefaultMountPoint constructs a path to the user's home directory for mounting FUSE
var DefaultMountPoint = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logs.Fatalf("Could not find user home directory: %s", err.Error())
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Projects")
	return p
}
