package filesystem

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/billziss-gh/cgofuse/fuse"

	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/logs"
)

// Open opens a file.
func (fs *Connectfs) Open(path string, flags int) (errc int, fh uint64) {
	defer fs.synchronize()()
	logs.Debug("Opening file ", path)

	node, errc, fh := fs.openNode(path, false)

	if !node.checkDecryption {
		if node.stat.Size >= 152 {
			path = strings.TrimPrefix(path, "/")
			if decrypted, err := api.IsDecrypted(path); err != nil {
				logs.Error(fmt.Errorf("Encryption status of object %s could not be determined: %w", path, err))
				return -fuse.EAGAIN, ^uint64(0)
			} else if decrypted {
				logs.Infof("Object %s is automatically decrypted", path)
				node.stat.Size = calculateDecryptedSize(node.stat.Size)
				node.stat.Ctim = fuse.Now()
			} else {
				logs.Debugf("Object %s is not decrypted", path)
			}
		}
		node.checkDecryption = true
	}

	return
}

// Opendir opens a directory.
func (fs *Connectfs) Opendir(path string) (errc int, fh uint64) {
	defer fs.synchronize()()
	logs.Debug("Opening directory ", path)
	_, errc, fh = fs.openNode(path, true)
	return
}

// Release closes a file.
func (fs *Connectfs) Release(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	logs.Debug("Closing file ", path)
	return fs.closeNode(fh)
}

// Releasedir closes a directory.
func (fs *Connectfs) Releasedir(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	logs.Debug("Closing directory ", path)
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
func (fs *Connectfs) Read(path string, buff []byte, ofst int64, fh uint64) int {
	defer fs.synchronize()()
	logs.Debugf("Reading %s", path)
	node := fs.getNode(path, fh)
	if nil == node {
		logs.Errorf("Read %s, inode does't exist", path)
		return -fuse.ENOENT
	}

	// Check whether this file has had its name changed
	path = strings.TrimPrefix(path, "/")
	if origName, ok := fs.renamed[path]; ok {
		path = origName
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
		return -fuse.EIO
	}
	n := copy(buff, data)

	// Update file accession timestamp
	node.stat.Atim = fuse.Now()
	return n
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
