package filesystem

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"
	log "github.com/sirupsen/logrus"

	"github.com/cscfi/sd-connect-fuse/internal/api"
)

const sRDONLY = 00444

// Connectfs stores the filesystem structure
type Connectfs struct {
	fuse.FileSystemBase
	lock    sync.Mutex
	ino     uint64
	root    *node
	openmap map[uint64]*node
}

type node struct {
	stat    fuse.Stat_t
	chld    map[string]*node
	data    []byte
	opencnt int
}

// CreateFileSystem initialises the in-memory filesystem database and mounts the root folder
func CreateFileSystem() *Connectfs {
	log.Info("Creating in-memory filesystem database")
	timestamp := fuse.Now()
	c := Connectfs{}
	defer c.synchronize()()
	c.ino++
	c.openmap = map[uint64]*node{}
	c.root = newNode(0, c.ino, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)
	c.populateFilesystem(timestamp)
	log.Info("Filesystem database completed")
	return &c
}

// populateDirectory creates the nodes (files and directories) of the filesystem
func (fs *Connectfs) populateFilesystem(timestamp fuse.Timespec) {
	projects, err := api.GetProjects()
	if err != nil {
		log.Error(err)
	}
	if len(projects) == 0 {
		log.Fatal("No project permissions found")
	}
	log.Infof("Receiving %d projects", len(projects))

	// Create dataset directories
	for i := range projects {
		// Remove characters which may interfere with filesystem structure
		project := removeInvalidChars(projects[i])

		// Create a project directory
		log.Debugf("Creating project %s", project)
		fs.makeNode(project, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)

		containers, err := api.GetContainers(projects[i])
		if err != nil {
			log.Error(err)
			continue
		}

		for j := range containers {
			// Remove characters which may interfere with filesystem structure
			container := removeInvalidChars(containers[j].Name)
			containerPath := project + "/" + container

			// Create a project directory
			log.Debugf("Creating container %s", containerPath)
			p := filepath.FromSlash(containerPath)
			fs.makeNode(p, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)

			// TODO: Temporary
			if containers[j].Count > 200000 {
				log.Errorf("Container %s too large (%d)", containers[j].Name, containers[j].Count)
				continue
			}

			objects, err := api.GetObjects(project, container)
			if err != nil {
				log.Error(err)
				continue
			}

			for _, obj := range objects {
				nodes := split(obj.Name)
				objectPath := containerPath

				for n := range nodes {
					objectPath = objectPath + "/" + removeInvalidChars(nodes[n])
					p := filepath.FromSlash(objectPath)

					if n == len(nodes)-1 {
						fs.makeNode(p, fuse.S_IFREG|sRDONLY, 0, obj.Bytes, timestamp)
					} else {
						fs.makeNode(p, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)
					}
				}
			}
		}
	}
}

// split deconstructs a filepath string into an array of strings
func split(path string) []string {
	return strings.Split(path, "/")
}

func removeInvalidChars(str string) string {
	return strings.Replace(strings.Replace(str, "/", "_", -1), ":", ".", -1)
}

// lookupNode finds the names and inodes of self and parents all the way to root directory
func (fs *Connectfs) lookupNode(path string, ancestor *node) (prnt *node, name string, node *node) {
	prnt, node = fs.root, fs.root
	name = ""
	for _, c := range split(filepath.ToSlash(path)) {
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
