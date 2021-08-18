package filesystem

import (
	"path/filepath"

	"github.com/billziss-gh/cgofuse/fuse"

	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/logs"
)

// Open opens a file.
func (fs *Connectfs) Open(path string, flags int) (errc int, fh uint64) {
	defer fs.synchronize()()
	return fs.openNode(path, false)
}

// Opendir opens a directory.
func (fs *Connectfs) Opendir(path string) (errc int, fh uint64) {
	defer fs.synchronize()()
	return fs.openNode(path, true)
}

// Release closes a file.
func (fs *Connectfs) Release(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	return fs.closeNode(fh)
}

// Releasedir closes a directory.
func (fs *Connectfs) Releasedir(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	return fs.closeNode(fh)
}

// Getattr returns file properties in stat structure.
func (fs *Connectfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer fs.synchronize()()
	node := fs.getNode(path, fh)
	if nil == node {
		return -fuse.ENOENT
	}
	*stat = node.stat
	return 0
}

// Read returns bytes from a file
func (fs *Connectfs) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer fs.synchronize()()
	logs.Debugf("Read %s", path)
	node := fs.getNode(path, fh)
	if nil == node {
		logs.Errorf("Read %s, inode does't exist", path)
		return -fuse.ENOENT
	}
	path = filepath.ToSlash(path)

	// Get file end coordinate
	endofst := ofst + int64(len(buff))
	if endofst > node.stat.Size {
		endofst = node.stat.Size
	}
	if endofst < ofst {
		return 0
	}

	// Download data from file
	data, err := api.DownloadData(path, ofst, endofst)
	if err != nil {
		logs.Error(err)
		return
	}
	n = copy(buff, data)

	// Update file accession timestamp
	node.stat.Atim = fuse.Now()
	logs.Debugf("File %s has been accessed/read", path)
	return
}

// Readdir reads the contents of a directory.
func (fs *Connectfs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64, fh uint64) (errc int) {
	defer fs.synchronize()()
	node := fs.openmap[fh]
	fill(".", &node.stat, 0)
	fill("..", nil, 0)
	for name, chld := range node.chld {
		if !fill(name, &chld.stat, 0) {
			break
		}
	}
	return 0
}
