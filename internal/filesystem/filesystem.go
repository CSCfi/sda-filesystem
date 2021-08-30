package filesystem

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"

	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/logs"
)

const sRDONLY = 00444
const numRoutines = 4

var signalBridge func() = nil

// Connectfs stores the filesystem structure
type Connectfs struct {
	fuse.FileSystemBase
	lock    sync.Mutex
	inoLock sync.RWMutex
	ino     uint64
	root    *node
	openmap map[uint64]*node
	renamed map[string]string
}

// node represents one file or directory
type node struct {
	stat    fuse.Stat_t
	chld    map[string]*node
	opencnt int
}

// containerInfo is a packet of information sent through a channel to createObjects()
type containerInfo struct {
	project   string
	container string
	timestamp fuse.Timespec
	fs        *Connectfs
}

// LoadProjectInfo is used to carry information through a channel to projectmodel
type LoadProjectInfo struct {
	Project string
	Count   int
}

// SetSignalBridge initializes the signal which informs QML that program has paniced
func SetSignalBridge(fn func()) {
	signalBridge = fn
}

// CreateFileSystem initialises the in-memory filesystem database and mounts the root folder
func CreateFileSystem(send ...chan<- LoadProjectInfo) *Connectfs {
	logs.Info("Creating in-memory filesystem database")
	timestamp := fuse.Now()
	c := Connectfs{}
	defer c.synchronize()()
	c.ino++
	c.openmap = map[uint64]*node{}
	c.renamed = map[string]string{}
	c.root = newNode(0, c.ino, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)
	c.populateFilesystem(timestamp, send...)
	logs.Info("Filesystem database completed")
	return &c
}

// populateFilesystem creates the nodes (files and directories) of the filesystem
func (fs *Connectfs) populateFilesystem(timestamp fuse.Timespec, send ...chan<- LoadProjectInfo) {
	projects, err := api.GetProjects()
	if err != nil {
		logs.Error(err)
		return
	}

	if len(projects) == 0 {
		logs.Errorf("No project permissions found")
		return
	}

	logs.Debug("Beginning filling in filesystem")

	// Calculating root size
	rootSize := int64(0)
	for i := range projects {
		rootSize += projects[i].Bytes
	}
	fs.root.stat.Size = rootSize

	var wg sync.WaitGroup
	forChannel := make(map[string][]api.Metadata)
	numJobs := 0
	mapLock := sync.RWMutex{}

	for i := range projects {
		wg.Add(1)

		// Remove characters which may interfere with filesystem structure
		projectSafe := removeInvalidChars(projects[i].Name)

		// Create a project directory
		logs.Debugf("Creating project %s", projectSafe)
		fs.makeNode(projectSafe, fuse.S_IFDIR|sRDONLY, 0, projects[i].Bytes, timestamp)

		go func(project string) {
			defer wg.Done()
			defer func() {
				// recover from panic if one occured.
				if err := recover(); err != nil && signalBridge != nil {
					logs.Error(fmt.Errorf("Something went wrong when creating filesystem: %w",
						fmt.Errorf("%v\n\n%s", err, string(debug.Stack()))))
					// Send alert
					signalBridge()
				}
			}()

			logs.Debugf("Fetching containers for project %s", project)
			containers, err := api.GetContainers(project)

			if err != nil {
				logs.Error(err)
				return
			}

			if len(send) > 0 {
				send[0] <- LoadProjectInfo{Project: project, Count: len(containers)}
			}

			mapLock.Lock()
			forChannel[project] = containers // LOCK
			numJobs += len(containers)       // LOCK
			mapLock.Unlock()

			for _, c := range containers {
				containerSafe := removeInvalidChars(c.Name)
				containerPath := projectSafe + "/" + containerSafe

				// Create a container directory
				logs.Debugf("Creating container %s", containerPath)
				p := filepath.FromSlash(containerPath)
				fs.makeNode(p, fuse.S_IFDIR|sRDONLY, 0, c.Bytes, timestamp)
			}
		}(projects[i].Name)
	}

	wg.Wait()
	jobs := make(chan containerInfo, numJobs)

	for w := 1; w <= numRoutines; w++ {
		wg.Add(1)
		if len(send) > 0 {
			go createObjects(w, jobs, &wg, send[0])
		} else {
			go createObjects(w, jobs, &wg)
		}
	}

	for key, value := range forChannel {
		for i := range value {
			jobs <- containerInfo{project: key, container: value[i].Name, timestamp: timestamp, fs: fs}
		}
	}
	close(jobs)

	wg.Wait()
	if len(send) > 0 {
		close(send[0])
	}
}

func createObjects(id int, jobs <-chan containerInfo, wg *sync.WaitGroup, send ...chan<- LoadProjectInfo) {
	defer wg.Done()
	defer func() {
		// recover from panic if one occured.
		if err := recover(); err != nil && signalBridge != nil {
			logs.Error(fmt.Errorf("Something went wrong when creating filesystem: %w",
				fmt.Errorf("%v\n\n%s", err, string(debug.Stack()))))
			// Send alert
			signalBridge()
		}
	}()

	for j := range jobs {
		project := j.project
		container := j.container
		fs := j.fs
		timestamp := j.timestamp

		projectSafe := removeInvalidChars(project)
		containerSafe := removeInvalidChars(container)
		containerPath := projectSafe + "/" + containerSafe

		logs.Debugf("Fetching objects for container %s", containerPath)
		objects, err := api.GetObjects(project, container)
		if err != nil {
			logs.Error(err)
			continue
		}

		level := 1

		// Object names contain their path from container
		// Creating both subdirectories and the files
		for len(objects) > 0 {
			// uniqueDirs contains the unique directory paths for this particular level
			uniqueDirs := make(map[string]int64)
			// remove contains the indexes that need to be removed from 'objects'
			remove := make([]int, 0, len(objects))

			for i, obj := range objects {
				parts := strings.SplitN(obj.Name, "/", level+1)

				// If true, create the final object file
				if len(parts) < level+1 {
					objectPath := containerPath + "/" + removeInvalidChars(obj.Name, "/")
					p := filepath.FromSlash(objectPath)
					logs.Debugf("Creating object %s", objectPath)
					fs.makeNode(p, fuse.S_IFREG|sRDONLY, 0, obj.Bytes, timestamp)
					remove = append(remove, i)
					continue
				}

				dir := strings.Join(parts[:len(parts)-1], "/")
				uniqueDirs[dir] += obj.Bytes
			}

			// remove is in increasing order. In order to index objects correctly,
			// remove has to be iterated in decreasing order.
			for i := len(remove) - 1; i >= 0; i-- {
				idx := remove[i]
				objects[idx] = objects[len(objects)-len(remove)+i]
			}
			objects = objects[:len(objects)-len(remove)]

			// Create all unique subdirectories at this level
			for key, value := range uniqueDirs {
				p := filepath.FromSlash(containerPath + "/" + removeInvalidChars(key, "/"))
				fs.makeNode(p, fuse.S_IFDIR|sRDONLY, 0, value, timestamp)
			}

			level++
		}

		if len(send) > 0 {
			send[0] <- LoadProjectInfo{Project: project, Count: 1}
		}
	}
}

// split deconstructs a filepath string into an array of strings
func split(path string) []string {
	return strings.Split(path, "/")
}

func removeInvalidChars(str string, ignore ...string) string {
	forReplacer := []string{"/", "_", "#", "_", "%", "_", "$", "_", "+",
		"_", "|", "_", "@", "_", ":", ".", "&", ".", "!", ".", "?", ".",
		"<", ".", ">", ".", "'", ".", "\"", "."}

	if len(ignore) == 1 {
		for i := 0; i < len(forReplacer)-1; i += 2 {
			if forReplacer[i] == ignore[0] {
				forReplacer[i] = forReplacer[len(forReplacer)-2]
				forReplacer[i+1] = forReplacer[len(forReplacer)-1]
				forReplacer = forReplacer[:len(forReplacer)-2]
				break
			}
		}
	}

	r := strings.NewReplacer(forReplacer...)
	return r.Replace(str)
}

// lookupNode finds the names and inodes of self and parents all the way to root directory
func (fs *Connectfs) lookupNode(path string) (prnt *node, name string, node *node, dir bool) {
	prnt, node = fs.root, fs.root
	name = ""
	dir = true
	for _, c := range split(filepath.ToSlash(path)) {
		if c != "" {
			prnt, name = node, c
			if node == nil {
				return
			}
			node = node.chld[c]
			if node != nil {
				dir = (fuse.S_IFDIR == node.stat.Mode&fuse.S_IFMT)
			}
		}
	}
	return
}

// makeNode adds a node into the fuse
func (fs *Connectfs) makeNode(path string, mode uint32, dev uint64, size int64, timestamp fuse.Timespec) int {
	prnt, name, node, isDir := fs.lookupNode(path)
	if prnt == nil {
		// No such file or directory
		return -fuse.ENOENT
	}
	if node != nil && (isDir == (fuse.S_IFDIR == mode&fuse.S_IFMT)) {
		// File/directory exists
		return -fuse.EEXIST
	}

	// A folder or a file with the same name already exists
	if node != nil {
		// Create a unique prefix for file
		i := 1
		for {
			newName := fmt.Sprintf("FILE_%d_%s", i, name)
			if _, ok := prnt.chld[newName]; !ok {
				location := strings.TrimSuffix(path, name)
				fs.renamed[location+newName] = path
				// Change name of node (whichever is a file)
				if !isDir {
					prnt.chld[newName] = node
				} else {
					name = newName
				}
				logs.Warningf("File %s has its name changed to %s due to a folder with the same name", path, newName)
				break
			}
			i++
		}
	}

	fs.inoLock.Lock()
	fs.ino++                                           // LOCK
	node = newNode(dev, fs.ino, mode, 0, 0, timestamp) // LOCK
	fs.inoLock.Unlock()

	node.stat.Size = size
	prnt.chld[name] = node
	prnt.stat.Ctim = node.stat.Ctim
	prnt.stat.Mtim = node.stat.Ctim
	return 0
}

// newNode initializes a node struct
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
		0}
	// Initialize map of children if node is a directory
	if fuse.S_IFDIR == self.stat.Mode&fuse.S_IFMT {
		self.chld = map[string]*node{}
	}
	return &self
}

func (fs *Connectfs) openNode(path string, dir bool) (int, uint64) {
	_, _, node, _ := fs.lookupNode(path)
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
		_, _, node, _ := fs.lookupNode(path)
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
