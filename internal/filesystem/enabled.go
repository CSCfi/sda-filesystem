package filesystem

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/billziss-gh/cgofuse/fuse"
)

// Open opens a file.
func (fs *Fuse) Open(path string, flags int) (errc int, fh uint64) {
	defer fs.synchronize()()
	logs.Debug("Opening file ", filepath.FromSlash(path))

	if !isValidOpen() {
		return -fuse.ECANCELED, ^uint64(0)
	}

	errc, fh = fs.openNode(path, false)
	if errc != 0 {
		return
	}

	if n := fs.openmap[fh]; n.path[0] == api.SDConnect && !n.node.decryptionChecked {
		newSize := n.node.stat.Size
		if err := api.UpdateAttributes(n.path, path, &newSize); err != nil {
			var re *api.RequestError
			if errors.As(err, &re) && re.StatusCode == 451 {
				logs.Errorf("You do not have permission to access file %s: %w", path, err)
				n.node.denied = true
				n.node.decryptionChecked = true
				return -fuse.EACCES, ^uint64(0)
			} else {
				logs.Errorf("Encryption status and segmented object size of object %s could not be determined: %w", path, err)
				return -fuse.EIO, ^uint64(0)
			}
		} else if n.node.stat.Size != newSize {
			fs.updateNodeSizesAlongPath(path, n.node.stat.Size-newSize, fuse.Now())
		}
		n.node.decryptionChecked = true
	}
	return
}

var isValidOpen = func() bool {
	switch runtime.GOOS {
	case "darwin":
		grep := exec.Command("pgrep", "-f", "QuickLook")
		if res, err := grep.Output(); err == nil {
			pids := strings.Split(string(res), "\n")
			_, _, pid := fuse.Getcontext()

			for i := range pids {
				pidInt, err := strconv.Atoi(pids[i])
				if err == nil && pidInt == pid {
					logs.Debug("Finder trying to create thumbnails")
					return false
				}
			}
		}
	case "windows":
		_, _, pid := fuse.Getcontext()
		filter := fmt.Sprintf("PID eq %d", pid)
		task := exec.Command("tasklist", "/FI", filter, "/fo", "table", "/nh")
		if res, err := task.Output(); err == nil {
			parts := strings.Fields(string(res))
			if parts[0] == "explorer.exe" {
				logs.Debug("Explorer trying to create thumbnails")
				return false
			}
		}
	}
	return true
}

// Opendir opens a directory.
func (fs *Fuse) Opendir(path string) (errc int, fh uint64) {
	defer fs.synchronize()()
	logs.Debug("Opening directory ", filepath.FromSlash(path))
	return fs.openNode(path, true)
}

// Release closes a file.
func (fs *Fuse) Release(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	logs.Debug("Closing file ", filepath.FromSlash(path))
	return fs.closeNode(fh)
}

// Releasedir closes a directory.
func (fs *Fuse) Releasedir(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	logs.Debug("Closing directory ", filepath.FromSlash(path))
	return fs.closeNode(fh)
}

// Getattr returns file properties in stat structure.
func (fs *Fuse) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer fs.synchronize()()
	node := fs.getNode(path, fh).node
	if node == nil {
		return -fuse.ENOENT
	}
	*stat = node.stat
	return 0
}

// Read returns bytes from a file
func (fs *Fuse) Read(path string, buff []byte, ofst int64, fh uint64) int {
	defer fs.synchronize()()
	logs.Debug("Reading ", filepath.FromSlash(path))

	n := fs.getNode(path, fh)
	if n.node == nil {
		logs.Errorf("File %s not found", path)
		return -fuse.ENOENT
	}

	if n.node.denied {
		return -fuse.EACCES
	}

	// Get file end coordinate
	endofst := ofst + int64(len(buff))
	if endofst > n.node.stat.Size {
		endofst = n.node.stat.Size
	}
	if endofst <= ofst {
		return 0
	}

	// Download data from file
	data, err := api.DownloadData(n.path, path, ofst, endofst, n.node.stat.Size)
	if err != nil {
		logs.Error(err)
		return -fuse.EIO
	}

	// Update file accession timestamp
	n.node.stat.Atim = fuse.Now()
	return copy(buff, data)
}

// Readdir reads the contents of a directory.
func (fs *Fuse) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64, fh uint64) (errc int) {
	defer fs.synchronize()()
	node := fs.getNode(path, fh).node
	if node == nil {
		return -fuse.ENOENT
	}
	fill(".", &node.stat, 0)
	fill("..", nil, 0)
	for name, chld := range node.chld {
		if !fill(name, &chld.stat, 0) {
			break
		}
	}
	return 0
}
