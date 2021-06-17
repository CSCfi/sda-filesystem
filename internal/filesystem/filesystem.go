package filesystem

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"
	log "github.com/sirupsen/logrus"
)

const sRDONLY = 00555

// Connectfs stores the filesystem structure
type Connectfs struct {
	fuse.FileSystemBase
	lock    sync.Mutex
	ino     uint64
	root    *node
	openmap map[uint64]*node
	origin  string
}

type node struct {
	stat    fuse.Stat_t
	chld    map[string]*node
	data    []byte
	opencnt int
}

// CreateFileSystem initialises the in-memory filesystem database and mounts the root folder
func CreateFileSystem() *Connectfs {
	log.Debug("Creating in-memory filesystem database")
	timestamp := fuse.Now()
	c := Connectfs{}
	defer c.synchronize()()
	c.ino++
	c.openmap = map[uint64]*node{}
	c.root = newNode(0, c.ino, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)
	c.origin = "/Users/emrehn/Documents/work/examples" // Remove this!
	c.populateDirectory("", timestamp)
	log.Debug("Filesystem database completed")
	return &c
}

// populateDirectory creates the nodes (files and directories) of the filesystem
func (fs *Connectfs) populateDirectory(dir string, timestamp fuse.Timespec) {
	// Remove characters which may interfere with filesystem structure
	//dir = strings.Replace(dir, "/", "_", -1)
	//dir = strings.Replace(dir, ":", ".", -1) // For Windows

	// Create a dataset directory
	log.Debugf("Creating directory %v", dir)
	fs.makeNode(dir, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)

	/*files, err := doa.GetDatasetFiles(datasets[i])
	if err != nil {
		log.Error(err)
	}*/
	dirs, files, err := fs.dirChildren(dir)

	if err != nil {
		log.Error(err)
		return
	}

	// Create file handles
	for j := range files {
		// Create a file handle
		log.Debugf("Creating file handler for file %s", files[j].Name())
		p := filepath.FromSlash(dir + "/" + files[j].Name())
		log.Debug(p)
		fs.makeNode(p, fuse.S_IFREG|sRDONLY, 0, files[j].Size(), timestamp)
	}

	for j := range dirs {
		p := filepath.FromSlash(dir + "/" + dirs[j])
		fs.populateDirectory(p, timestamp)
	}
}

// dirChildren will be replaced with a function that gets this info from api
func (fs *Connectfs) dirChildren(dir string) (dirs []string, files []os.FileInfo, err error) {
	ch, err := ioutil.ReadDir(filepath.FromSlash(fs.origin + "/" + dir))

	if err != nil {
		log.Errorf("Couldn't read directory %v. Directory left empty.", dir)
		return
	}

	for _, entry := range ch {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		} else {
			files = append(files, entry)
		}
	}
	return
}

// lookupNode finds the names and inodes of self and parents all the way to root directory
func (fs *Connectfs) lookupNode(path string, ancestor *node) (prnt *node, name string, node *node) {
	prnt, node = fs.root, fs.root
	name = ""
	for _, c := range strings.Split(filepath.ToSlash(path), "/") {
		if c != "" {
			if len(c) > 255 {
				panic(fuse.Error(-fuse.ENAMETOOLONG))
			}
			prnt, name = node, c
			if node == nil {
				return
			}
			node = node.chld[c]
			if ancestor != nil && node == ancestor {
				name = "" // special case loop condition
				return
			}
		}
	}
	return
}

func (fs *Connectfs) makeNode(path string, mode uint32, dev uint64, size int64, timestamp fuse.Timespec) int {
	prnt, name, node := fs.lookupNode(path, nil)
	if prnt == nil {
		return -fuse.ENOENT
	}
	if node != nil {
		return -fuse.EEXIST
	}
	fs.ino++
	node = newNode(dev, fs.ino, mode, 0, 0, timestamp)
	node.stat.Size = size
	prnt.chld[name] = node
	prnt.stat.Ctim = node.stat.Ctim
	prnt.stat.Mtim = node.stat.Ctim
	return 0
}

func newNode(dev uint64, ino uint64, mode uint32, uid uint32, gid uint32, tmsp fuse.Timespec) *node {
	self := node{
		fuse.Stat_t{
			Dev:      dev,
			Ino:      ino,
			Mode:     mode,
			Nlink:    1,
			Uid:      uid,
			Gid:      gid,
			Atim:     tmsp,
			Mtim:     tmsp,
			Ctim:     tmsp,
			Birthtim: tmsp,
			Flags:    0,
		},
		nil,
		nil,
		0}
	if fuse.S_IFDIR == self.stat.Mode&fuse.S_IFMT {
		self.chld = map[string]*node{}
	}
	return &self
}

func (fs *Connectfs) openNode(path string, dir bool) (int, uint64) {
	_, _, node := fs.lookupNode(path, nil)
	if node == nil {
		return -fuse.ENOENT, ^uint64(0)
	}
	if !dir && fuse.S_IFDIR == node.stat.Mode&fuse.S_IFMT {
		return -fuse.EISDIR, ^uint64(0)
	}
	if dir && fuse.S_IFDIR != node.stat.Mode&fuse.S_IFMT {
		return -fuse.ENOTDIR, ^uint64(0)
	}
	node.opencnt++
	if node.opencnt == 1 {
		fs.openmap[node.stat.Ino] = node
	}
	return 0, node.stat.Ino
}

func (fs *Connectfs) closeNode(fh uint64) int {
	node := fs.openmap[fh]
	node.opencnt--
	if node.opencnt == 0 {
		delete(fs.openmap, node.stat.Ino)
	}
	return 0
}

func (fs *Connectfs) getNode(path string, fh uint64) *node {
	if fh == ^uint64(0) {
		_, _, node := fs.lookupNode(path, nil)
		return node
	}
	return fs.openmap[fh]
}

func (fs *Connectfs) synchronize() func() {
	fs.lock.Lock()
	return func() {
		fs.lock.Unlock()
	}
}
