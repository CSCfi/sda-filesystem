package filesystem

import (
	"fmt"

	"github.com/billziss-gh/cgofuse/fuse"
	log "github.com/sirupsen/logrus"
)

// Destroy ...
func (fs *Connectfs) Destroy() {
	defer fs.synchronize()()
	fmt.Println("I am destroyed :(")
}

// Init ...
func (fs *Connectfs) Init() {
	defer fs.synchronize()()
	fmt.Println("I am initialized :)")
}

// Open ...
func (fs *Connectfs) Open(path string, flags int) (errc int, fh uint64) {
	defer fs.synchronize()()
	log.Debugf("Open %s", path)
	return fs.openNode(path, false)
}

// Opendir ...
func (fs *Connectfs) Opendir(path string) (errc int, fh uint64) {
	defer fs.synchronize()()
	log.Debugf("Opendir %s", path)
	return fs.openNode(path, true)
}

// Release ...
func (fs *Connectfs) Release(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	log.Debugf("Release %s", path)
	return fs.closeNode(fh)
}

// Releasedir ...
func (fs *Connectfs) Releasedir(path string, fh uint64) (errc int) {
	defer fs.synchronize()()
	log.Debugf("Releasedir %s", path)
	return fs.closeNode(fh)
}

// Getattr ...
func (fs *Connectfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer fs.synchronize()()
	node := fs.getNode(path, fh)
	if nil == node {
		return -fuse.ENOENT
	}
	*stat = node.stat
	return 0
}

// Read ...
func (fs *Connectfs) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer fs.synchronize()()
	log.Debugf("Read %s", path)
	node := fs.getNode(path, fh)
	if nil == node {
		return -fuse.ENOENT
	}
	endofst := ofst + int64(len(buff))
	if endofst > node.stat.Size {
		endofst = node.stat.Size
	}
	if endofst < ofst {
		return 0
	}
	n = 0 //copy(buff, node.data[ofst:endofst])
	node.stat.Atim = fuse.Now()
	return
}

// Readdir ...
func (fs *Connectfs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64, fh uint64) (errc int) {
	log.Debugf("Readdir %s", path)
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

//func (*FileSystemHost) Unmount
