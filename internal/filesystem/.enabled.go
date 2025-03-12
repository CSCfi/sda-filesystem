package filesystem

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
)

// Open opens a file.
func (fs *Fuse) Open(path string, _ int) (errc int, fh uint64) {
	logs.Debug("Opening file ", filepath.FromSlash(path))

	if !isValidOpen() {
		return -ECANCELED, ^uint64(0)
	}

	errc, fh = fs.openNode(path, false)
	if errc != 0 {
		return
	}

	return
}

var isValidOpen = func() bool {
	switch runtime.GOOS {
	case "darwin":
		grep := exec.Command("pgrep", "-f", "QuickLook")
		if res, err := grep.Output(); err == nil {
			pids := strings.Split(string(res), "\n")
			_, _, pid := Getcontext()

			for i := range pids {
				pidInt, err := strconv.Atoi(pids[i])
				if err == nil && pidInt == pid {
					logs.Debug("Finder trying to create thumbnails")

					return false
				}
			}
		}
	case "windows":
		_, _, pid := Getcontext()
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
	logs.Debug("Opening directory ", filepath.FromSlash(path))

	return fs.openNode(path, true)
}

// Release closes a file.
func (fs *Fuse) Release(path string, fh uint64) (errc int) {
	logs.Debug("Closing file ", filepath.FromSlash(path))

	return fs.closeNode(fh)
}

// Releasedir closes a directory.
func (fs *Fuse) Releasedir(path string, fh uint64) (errc int) {
	logs.Debug("Closing directory ", filepath.FromSlash(path))

	return fs.closeNode(fh)
}

// Getattr returns file properties in stat structure.
func (fs *Fuse) Getattr(path string, stat *Stat_t, fh uint64) (errc int) {
	node := fs.getNode(path, fh).node
	if node == nil {
		return -ENOENT
	}
	*stat = node.stat

	return 0
}

// Read returns bytes from a file
func (fs *Fuse) Read(path string, buff []byte, ofst int64, fh uint64) int {
	logs.Debug("Reading ", filepath.FromSlash(path))

	n := fs.getNode(path, fh)
	if n.node == nil {
		logs.Errorf("File %s not found", path)

		return -ENOENT
	}

	if n.node.denied {
		return -EACCES
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

		return -EIO
	}

	// Update file accession timestamp
	n.node.stat.Atim = Now()

	return copy(buff, data)
}

// Readdir reads the contents of a directory.
func (fs *Fuse) Readdir(path string, fill func(name string, stat *Stat_t, ofst int64) bool,
	_ int64, fh uint64) (errc int) {
	node := fs.getNode(path, fh).node
	if node == nil {
		return -ENOENT
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
