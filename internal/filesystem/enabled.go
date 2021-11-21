package filesystem

import (
	"path/filepath"
	"strings"

	"github.com/billziss-gh/cgofuse/fuse"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
)

// Open opens a file.
func (fs *Fuse) Open(path string, flags int) (errc int, fh uint64) {
	defer fs.synchronize()()
	logs.Debug("Opening file ", path)

	errc, fh = fs.openNode(path, false)
	if errc != 0 {
		return
	}

	if on := fs.openmap[fh]; on.path[0] == api.SDConnect && !on.node.checkDecryption {
		path = filepath.ToSlash(path)
		path = strings.TrimPrefix(path, "/")
		newSize := on.node.stat.Size
		api.UpdateAttributes(on.path, path, &newSize)
		if newSize == -1 {
			return -fuse.EIO, ^uint64(0)
		}
		if on.node.stat.Size != newSize {
			on.node.stat.Size = newSize
			on.node.stat.Ctim = fuse.Now()
		}
		on.node.checkDecryption = true
	}
	return
}

// Opendir opens a directory.
func (fs *Fuse) Opendir(path string) (errc int, fh uint64) {
	defer fs.synchronize()()
	logs.Debug("Opening directory ", path)
	errc, fh = fs.openNode(path, true)
	return
}

// Release closes a file.
func (fs *Fuse) Release(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	logs.Debug("Closing file ", path)
	return fs.closeNode(fh)
}

// Releasedir closes a directory.
func (fs *Fuse) Releasedir(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	logs.Debug("Closing directory ", path)
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
	logs.Debug("Reading ", path)

	on := fs.getNode(path, fh)
	if on.node == nil {
		logs.Errorf("File %q not found", path)
		return -fuse.ENOENT
	}

	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "/")

	// Get file end coordinate
	endofst := ofst + int64(len(buff))
	if endofst > on.node.stat.Size {
		endofst = on.node.stat.Size
	}
	if endofst <= ofst {
		return 0
	}

	// Download data from file
	data, err := api.DownloadData(on.path, path, ofst, endofst, on.node.stat.Size)
	if err != nil {
		logs.Error(err)
		return -fuse.EIO
	}

	// Update file accession timestamp
	on.node.stat.Atim = fuse.Now()
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
		if fs.hidden[node.stat.Ino] {
			for key := range chld.chld {
				if !fill(key, &chld.chld[key].stat, 0) {
					return 0
				}
			}
		} else if !fill(name, &chld.stat, 0) {
			break
		}
	}
	return 0
}
