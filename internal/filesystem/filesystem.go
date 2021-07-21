package filesystem

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"
	log "github.com/sirupsen/logrus"

	"sd-connect-fuse/internal/api"
)

const sRDONLY = 00444
const numRoutines = 4

// Connectfs stores the filesystem structure
type Connectfs struct {
	fuse.FileSystemBase
	lock    sync.Mutex
	inoLock sync.RWMutex
	ino     uint64
	root    *node
	openmap map[uint64]*node
}

type node struct {
	stat    fuse.Stat_t
	chld    map[string]*node
	opencnt int
}

type containerInfo struct {
	project   string
	container string
	timestamp fuse.Timespec
	fs        *Connectfs
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

	var wg sync.WaitGroup
	forChannel := make(map[string][]api.Container)
	numJobs := 0
	mapLock := sync.RWMutex{}
	//start := time.Now()

	for i := range projects {
		wg.Add(1)

		// Remove characters which may interfere with filesystem structure
		projectSafe := removeInvalidChars(projects[i])

		// Create a project directory
		log.Debugf("Creating project %s", projectSafe)
		fs.makeNode(projectSafe, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)

		go func(project string) {
			defer wg.Done()

			log.Debugf("Fetching containers for project %s", project)
			containers, err := api.GetContainers(project)

			if err != nil {
				log.Error(err)
				return
			}

			mapLock.Lock()
			forChannel[project] = containers // LOCK
			numJobs += len(containers)       // LOCK
			mapLock.Unlock()

			for _, c := range containers {
				containerSafe := removeInvalidChars(c.Name)
				containerPath := projectSafe + "/" + containerSafe

				// Create a container directory
				log.Debugf("Creating container %s", containerPath)
				p := filepath.FromSlash(containerPath)
				fs.makeNode(p, fuse.S_IFDIR|sRDONLY, 0, c.Bytes, timestamp)
			}
		}(projects[i])
	}

	wg.Wait()

	//fmt.Println(runtime.NumGoroutine(), runtime.GOMAXPROCS(-1))
	//elapsed := time.Since(start)
	//fmt.Printf("Time: %s\n", elapsed)

	jobs := make(chan containerInfo, numJobs)

	//fmt.Println(numJobs)
	//start = time.Now()

	for w := 1; w <= numRoutines; w++ {
		wg.Add(1)
		go createObjects(w, jobs, &wg)
	}

	for key, value := range forChannel {
		for i := range value {
			jobs <- containerInfo{project: key, container: value[i].Name, timestamp: timestamp, fs: fs}
		}
	}
	close(jobs)

	wg.Wait()
	//elapsed = time.Since(start)
	//fmt.Printf("Time: %s\n", elapsed)
}

func createObjects(id int, jobs <-chan containerInfo, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		project := j.project
		container := j.container
		fs := j.fs
		timestamp := j.timestamp

		projectSafe := removeInvalidChars(project)
		containerSafe := removeInvalidChars(container)
		containerPath := projectSafe + "/" + containerSafe

		objects, err := api.GetObjects(project, container)
		if err != nil {
			log.Error(err)
			continue
		}

		level := 1

		// Object names contain their path from container
		// Create both subdirectories and the files
		for len(objects) > 0 {
			// uniqueDirs contains the unique directory paths for this particular level
			uniqueDirs := make(map[string]int64)
			remove := make([]int, 0, len(objects))

			for i, obj := range objects {
				parts := strings.SplitN(obj.Name, "/", level+1)

				if len(parts) < level+1 {
					objectPath := containerPath + "/" + removeInvalidChars(obj.Name, "/")
					p := filepath.FromSlash(objectPath)
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

			for key, value := range uniqueDirs {
				p := filepath.FromSlash(containerPath + "/" + removeInvalidChars(key, "/"))
				fs.makeNode(p, fuse.S_IFDIR|sRDONLY, 0, value, timestamp)
			}

			level++
		}
	}
}

// split deconstructs a filepath string into an array of strings
func split(path string) []string {
	return strings.Split(path, "/")
}

// Characters '/' and ':' are not allowed in names of directories or files
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
func (fs *Connectfs) lookupNode(path string) (prnt *node, name string, node *node) {
	prnt, node = fs.root, fs.root
	name = ""
	for _, c := range split(filepath.ToSlash(path)) {
		if c != "" {
			if len(c) > 255 {
				log.Fatalf("Name %s in path %s is too long", c, path)
			}
			prnt, name = node, c
			if node == nil {
				return
			}
			node = node.chld[c]
		}
	}
	return
}

// makeNode adds a node into the fuse
func (fs *Connectfs) makeNode(path string, mode uint32, dev uint64, size int64, timestamp fuse.Timespec) int {
	prnt, name, node := fs.lookupNode(path)
	if prnt == nil {
		// No such file or directory
		return -fuse.ENOENT
	}
	if node != nil {
		// File exists
		return -fuse.EEXIST
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
	_, _, node := fs.lookupNode(path)
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
		_, _, node := fs.lookupNode(path)
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
