package filesystem

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
)

const sRDONLY = 00444
const numRoutines = 4

var signalBridge func() = nil

// Fuse stores the filesystem structure
type Fuse struct {
	fuse.FileSystemBase
	lock    sync.Mutex
	inoLock sync.RWMutex
	ino     uint64
	root    *node
	openmap map[uint64]nodeAndPath
}

// node represents one file or directory
type node struct {
	stat            fuse.Stat_t
	chld            map[string]*node
	opencnt         int
	originalName    string // so that api calls work
	checkDecryption bool
}

// nodeAndPath contains the node itself and a list of names which are the original path to the node. Yes, a very original name
type nodeAndPath struct {
	node *node
	path []string
}

// containerInfo is a packet of information sent through a channel to createObjects()
type containerInfo struct {
	containerPath string
	timestamp     fuse.Timespec
	fs            *Fuse
}

// LoadProjectInfo is used to carry information through a channel to projectmodel
type LoadProjectInfo struct {
	Repository string
	Project    string
	Count      int
}

// SetSignalBridge initializes the signal which informs QML that program has paniced
func SetSignalBridge(fn func()) {
	signalBridge = fn
}

// CheckPanic recovers from panic if one occured. Used for GUI
func CheckPanic() {
	if signalBridge != nil {
		if err := recover(); err != nil {
			logs.Error(fmt.Errorf("Something went wrong when creating filesystem: %w",
				fmt.Errorf("%v\n\n%s", err, string(debug.Stack()))))
			// Send alert
			signalBridge()
		}
	}
}

// InitializeFileSystem initializes the in-memory filesystem database
func InitializeFileSystem(send ...chan<- LoadProjectInfo) *Fuse {
	logs.Info("Initializing in-memory filesystem database")
	timestamp := fuse.Now()
	fs := Fuse{}
	fs.ino++
	fs.openmap = map[uint64]nodeAndPath{}
	fs.root = newNode(0, fs.ino, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)

	for _, enabled := range api.GetEnabledRepositories() {
		logs.Info("Beginning filling in ", enabled)

		// Create folders for each repository
		md := api.Metadata{Name: enabled, OrigName: enabled, Bytes: -1}
		fs.makeNode(enabled, md, fuse.S_IFDIR|sRDONLY, 0, timestamp)

		// These are the folders displayed in GUI
		projects, _ := api.GetNthLevel(enabled)
		for _, project := range projects {
			removeInvalidChars(&project)

			if project.Name != project.OrigName {
				project.Name = renameDirectory(fs.root.chld[enabled], enabled, project.Name)
			}

			projectPath := enabled + "/" + project.Name

			// Create a project directory
			logs.Debugf("Creating directory %q", filepath.FromSlash(projectPath))
			fs.makeNode(projectPath, project, fuse.S_IFDIR|sRDONLY, 0, timestamp)

			if len(send) > 0 {
				send[0] <- LoadProjectInfo{Repository: enabled, Project: project.Name}
			}
		}
	}

	if len(send) > 0 {
		close(send[0])
	}
	return &fs
}

// MountFilesystem mounts filesystem 'fs' to directory 'mount'
func MountFilesystem(fs *Fuse, mount string) {
	host := fuse.NewFileSystemHost(fs)

	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
		options = append(options, "-o", "volname="+path.Base(mount))
		options = append(options, "-o", "attr_timeout=0") // This causes the fuse to call getattr between open and read
		options = append(options, "-o", "iosize=262144")  // Value not optimized
	} else if runtime.GOOS == "linux" {
		options = append(options, "-o", "attr_timeout=0") // This causes the fuse to call getattr between open and read
		options = append(options, "-o", "auto_unmount")
	} // Still needs windows options

	logs.Infof("Mounting filesystem at %q", mount)
	host.Mount(mount, options)
}

// PopulateFilesystem creates the rest of the nodes (files and directories) of the filesystem
func (fs *Fuse) PopulateFilesystem(send ...chan<- LoadProjectInfo) {
	timestamp := fuse.Now()
	fs.root.stat.Size = -1

	/*if len(c.projects) == 0 {
		return fmt.Errorf("No projects found for %s", SDConnect)
	}*/

	var wg sync.WaitGroup
	forChannel := make(map[string][]api.Metadata)
	numJobs := 0
	mapLock := sync.RWMutex{}

	for rep := range fs.root.chld {
		for pr := range fs.root.chld[rep].chld {
			wg.Add(1)

			go func(repository, project string) {
				defer wg.Done()
				defer CheckPanic()

				projectPath := repository + "/" + project
				logs.Debugf("Fetching data for %q", filepath.FromSlash(projectPath))
				containers, err := api.GetNthLevel(repository, fs.root.chld[repository].chld[project].originalName)

				if err != nil {
					logs.Error(err)
					return
				}

				for i, c := range containers {
					removeInvalidChars(&c)

					if c.Name != c.OrigName {
						c.Name = renameDirectory(fs.root.chld[repository].chld[project], project, c.Name)
					}

					containerPath := projectPath + "/" + c.Name
					containers[i].Name = containerPath

					mode := fuse.S_IFREG | sRDONLY
					if api.LevelCount(repository) > 2 {
						mode = fuse.S_IFDIR | sRDONLY
					}

					// Create a container directory
					logs.Debugf("Creating node %q", filepath.FromSlash(containerPath))
					fs.makeNode(containerPath, c, uint32(mode), 0, timestamp)
				}

				if api.LevelCount(repository) > 2 {
					mapLock.Lock()
					forChannel[project] = containers // LOCK
					numJobs += len(containers)       // LOCK
					mapLock.Unlock()

					if len(send) > 0 {
						send[0] <- LoadProjectInfo{Repository: repository, Project: project, Count: len(containers)}
					}
				} else if len(send) > 0 {
					send[0] <- LoadProjectInfo{Repository: repository, Project: project, Count: 0}
				}
			}(rep, pr)
		}
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

	for _, value := range forChannel {
		for i := range value {
			jobs <- containerInfo{containerPath: value[i].Name, timestamp: timestamp, fs: fs}
		}
	}
	close(jobs)

	wg.Wait()
	if len(send) > 0 {
		close(send[0])
	}

	// Calculate the size of higher level directories whose size currently is just -1.
	calculateFinalSize(fs.root, "")
	logs.Info("Filesystem database completed")
}

var removeInvalidChars = func(meta *api.Metadata) {
	forReplacer := []string{"/", "_", "#", "_", "%", "_", "$", "_", "+",
		"_", "|", "_", "@", "_", ":", ".", "&", ".", "!", ".", "?", ".",
		"<", ".", ">", ".", "'", ".", "\"", "."}

	// Remove characters which may interfere with filesystem structure
	r := strings.NewReplacer(forReplacer...)
	metaSafe := r.Replace(meta.Name)

	if meta.OrigName == "" {
		meta.OrigName = meta.Name
	}
	meta.Name = metaSafe
}

func renameDirectory(n *node, parent, chld string) string {
	if _, ok := n.chld[chld]; ok {
		i := 1
		for {
			newName := fmt.Sprintf("%s(%d)", chld, i)
			if _, ok := n.chld[newName]; !ok {
				path := filepath.FromSlash(parent + "/" + chld)
				logs.Warningf("Folder %q has its name changed to %s due to a folder with the same name", path, newName)
				return newName
			}
			i++
		}
	}
	return chld
}

func calculateFinalSize(n *node, path string) int64 {
	if n.stat.Size != -1 {
		return n.stat.Size
	}
	if n.chld == nil {
		logs.Warningf("Node %q has size -1 but no children! Folder sizes may be displayed incorrectly", filepath.FromSlash(path))
		return 0
	}

	n.stat.Size = 0
	for key, value := range n.chld {
		n.stat.Size += calculateFinalSize(value, path+"/"+key)
	}
	return n.stat.Size
}

var createObjects = func(id int, jobs <-chan containerInfo, wg *sync.WaitGroup, send ...chan<- LoadProjectInfo) {
	defer wg.Done()
	defer CheckPanic()

	for j := range jobs {
		containerPath := j.containerPath
		fs := j.fs
		timestamp := j.timestamp

		logs.Debugf("Fetching data for subdirectory %q", filepath.FromSlash(containerPath))

		nodes := fs.getNode(containerPath, ^uint64(0)).path
		objects, err := api.GetNthLevel(nodes[0], nodes[1], nodes[2])
		if err != nil {
			logs.Error(err)
			continue
		}

		level := 1

		// Object names contain their path from container (in SD-Connect)
		// In SD-Submit, the structure should be flat
		// Creating both subdirectories and the files
		for len(objects) > 0 {
			// uniqueDirs contains the unique directory paths for this particular level
			uniqueDirs := make(map[string]int64)
			// remove contains the indexes that need to be removed from 'objects'
			remove := make([]int, 0, len(objects))

			for i, obj := range objects {
				parts := strings.SplitN(obj.Name, "/", level+1)

				// Prevent the creation of objects that are actually empty directories
				if level == 1 && strings.HasSuffix(obj.Name, "/") {
					remove = append(remove, i)
					continue
				}

				// If true, create the final object file
				if len(parts) < level+1 {
					obj.Name = parts[len(parts)-1]
					removeInvalidChars(&obj)
					objectPath := containerPath + "/" + obj.Name
					logs.Debugf("Creating file %q", filepath.FromSlash(objectPath))
					fs.makeNode(objectPath, obj, fuse.S_IFREG|sRDONLY, 0, timestamp)
					remove = append(remove, i)
					continue
				}

				dir := join(parts[:len(parts)-1])
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
				md := api.Metadata{Bytes: value, Name: path.Base(key)}
				removeInvalidChars(&md)
				p := containerPath + "/" + md.Name
				logs.Debugf("Creating node %q", filepath.FromSlash(p))
				fs.makeNode(p, md, fuse.S_IFDIR|sRDONLY, 0, timestamp)
			}

			level++
		}

		if len(send) > 0 {
			nodes = split(containerPath)
			send[0] <- LoadProjectInfo{Repository: nodes[0], Project: nodes[1], Count: 1}
		}
	}
}

// split deconstructs a filepath string into an array of strings
func split(path string) []string {
	return strings.Split(path, "/")
}

// join constructs a filepath string from an array of strings
func join(path []string) string {
	return strings.Join(path, "/")
}

// lookupNode finds the names and inodes of self and parents all the way to root directory
func (fs *Fuse) lookupNode(n *node, path string) (prnt *node, node *node, dir bool, origPath []string) {
	prnt, node, dir = n, n, true
	parts := split(filepath.ToSlash(path))
	for _, c := range parts {
		if c != "" {
			prnt = node
			if node == nil {
				return
			}

			node = node.chld[c]
			if node != nil {
				origPath = append(origPath, node.originalName)
				dir = (fuse.S_IFDIR == node.stat.Mode&fuse.S_IFMT)
			}
		}
	}
	return
}

// newNode initializes a node struct
var newNode = func(dev uint64, ino uint64, mode uint32, uid uint32, gid uint32, tmsp fuse.Timespec) *node {
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
		0,
		"",
		false}
	// Initialize map of children if node is a directory
	if fuse.S_IFDIR == self.stat.Mode&fuse.S_IFMT {
		self.chld = map[string]*node{}
	}
	return &self
}

// makeNode adds a node into the fuse
func (fs *Fuse) makeNode(path string, meta api.Metadata, mode uint32, dev uint64, timestamp fuse.Timespec) int {
	prnt, node, isDir, _ := fs.lookupNode(fs.root, path)
	if prnt == nil {
		// No such file or directory
		return -fuse.ENOENT
	}
	if node != nil && (isDir == (fuse.S_IFDIR == mode&fuse.S_IFMT)) {
		// File/directory exists
		return -fuse.EEXIST
	}

	// A folder or a file with a different mode but the same name already exists
	if node != nil {
		// Create a unique prefix for file
		i := 1
		for {
			newName := fmt.Sprintf("%s(%d)", meta.Name, i)
			if _, ok := prnt.chld[newName]; !ok {
				// Change name of node (whichever is a file)
				if !isDir {
					prnt.chld[newName] = node
				} else {
					meta.Name = newName
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

	node.stat.Size = meta.Bytes
	node.originalName = meta.OrigName
	prnt.chld[meta.Name] = node
	prnt.stat.Ctim = node.stat.Ctim
	prnt.stat.Mtim = node.stat.Ctim
	return 0
}

func (fs *Fuse) openNode(path string, dir bool) (int, uint64) {
	_, n, _, origPath := fs.lookupNode(fs.root, path)
	if n == nil {
		return -fuse.ENOENT, ^uint64(0)
	}
	if !dir && fuse.S_IFDIR == n.stat.Mode&fuse.S_IFMT {
		return -fuse.EISDIR, ^uint64(0)
	}
	if dir && fuse.S_IFDIR != n.stat.Mode&fuse.S_IFMT {
		return -fuse.ENOTDIR, ^uint64(0)
	}
	n.opencnt++
	if n.opencnt == 1 {
		fs.openmap[n.stat.Ino] = nodeAndPath{node: n, path: origPath}
	}
	return 0, n.stat.Ino
}

func (fs *Fuse) closeNode(fh uint64) int {
	node := fs.openmap[fh].node
	node.opencnt--
	if node.opencnt == 0 {
		delete(fs.openmap, node.stat.Ino)
	}
	return 0
}

func (fs *Fuse) getNode(path string, fh uint64) nodeAndPath {
	if fh == ^uint64(0) {
		_, node, _, origPath := fs.lookupNode(fs.root, path)
		return nodeAndPath{node: node, path: origPath}
	}
	return fs.openmap[fh]
}

func (fs *Fuse) synchronize() func() {
	fs.lock.Lock()
	return func() {
		fs.lock.Unlock()
	}
}
