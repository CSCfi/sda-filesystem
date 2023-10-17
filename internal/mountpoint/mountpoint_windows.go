package mountpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

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

func WaitForUpdateSignal(ch chan<- []string) {
}

func BytesAvailable(dir string) (freeBytes uint64, err error) {
	lazy := windows.NewLazySystemDLL("kernel32.dll")
	if err = lazy.Load(); err != nil {
		return
	}
	c := lazy.NewProc("GetDiskFreeSpaceExW")
	if err = c.Find(); err != nil {
		return
	}

	var num1, num2 uint64 // Not used for anything

	_, _, err = c.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(dir))),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&num1)),
		uintptr(unsafe.Pointer(&num2)))

	errno, ok := err.(windows.Errno)
	if ok && errno == windows.ERROR_SUCCESS {
		err = nil

		return
	}

	return
}
