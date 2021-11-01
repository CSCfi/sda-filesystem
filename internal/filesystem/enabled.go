package filesystem

import (
	"fmt"
	"os"
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

	if node != nil && !node.checkDecryption {
		// In case file has been renamed
		origPath := path
		path = strings.TrimPrefix(path, string(os.PathSeparator))
		if origName, ok := fs.renamed[path]; ok {
			path = origName
		}

		decrypted, segSize, err := api.GetSpecialHeaders(path)
		if err != nil {
			logs.Error(fmt.Errorf("Encryption status and segmented object size of object %s could not be determined: %w", origPath, err))
			return -fuse.EAGAIN, ^uint64(0)
		}
		if segSize != -1 {
			logs.Infof("Object %s is a segmented object with size %d", origPath, segSize)
			node.stat.Size = segSize
			node.stat.Ctim = fuse.Now()
		}
		if decrypted {
			dSize := calculateDecryptedSize(node.stat.Size)
			if dSize != -1 {
				logs.Infof("Object %s is automatically decrypted", origPath)
				node.stat.Size = dSize
				node.stat.Ctim = fuse.Now()
			} else {
				logs.Warningf("API returned header X-Decrypted even though size of object %s is too small", origPath)
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
	path = strings.TrimPrefix(path, string(os.PathSeparator))
	if origName, ok := fs.renamed[path]; ok {
		path = origName
	}
	path = filepath.ToSlash(path)

	// Get file end coordinate
	endofst := ofst + int64(len(buff))
	if endofst > node.stat.Size {
		endofst = node.stat.Size
	}
	if endofst <= ofst {
		return 0
	}

	// Download data from file
	data, err := api.DownloadData(path, ofst, endofst, node.stat.Size)
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
