package mountpoint

import (
	"fmt"
	"os"
	"path/filepath"

	"sda-filesystem/internal/logs"

	"golang.org/x/sys/windows"
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

func BytesAvailable(dir string) (uint64, error) {
	h, err := windows.LoadDLL("kernel32.dll")
	if err != nil {
		return 0, err
	}
	c, err := h.FindProc("GetDiskFreeSpaceExW")
	if err != nil {
		return 0, err
	}

	var freeBytes int64

	_, _, err := c.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(dir))),
		uintptr(unsafe.Pointer(&freeBytes)), nil, nil)
	if err != nil {
		return 0, err
	}
	return freeBytes, nil
}
