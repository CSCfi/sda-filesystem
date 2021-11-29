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
	openmap map[uint64]openNode
	hidden  map[uint64]bool // which directories are hidden?
}

// node represents one file or directory
type node struct {
	stat            fuse.Stat_t
	chld            map[string]*node
	opencnt         int
	originalName    string // so that api calls work
	checkDecryption bool
}

// openNode contains the node itself and a list of names which are the original path to the node
type openNode struct {
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
	if err := recover(); err != nil && signalBridge != nil {
		logs.Error(fmt.Errorf("Something went wrong when creating filesystem: %w",
			fmt.Errorf("%v\n\n%s", err, string(debug.Stack()))))
		// Send alert
		signalBridge()
	}
}

// CreateFileSystem initializes the in-memory filesystem database
func CreateFileSystem(send ...chan<- LoadProjectInfo) *Fuse {
	logs.Info("Creating in-memory filesystem database")
	timestamp := fuse.Now()
	fs := Fuse{}
	defer fs.synchronize()()
	fs.ino++
	fs.openmap = map[uint64]openNode{}
	fs.hidden = map[uint64]bool{}
	fs.root = newNode(0, fs.ino, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)

	for _, enabled := range api.GetEnabledRepositories() {
		logs.Info("Beginning filling in ", enabled)

		md := api.Metadata{Name: enabled, OrigName: enabled, Bytes: -1}
		fs.makeNode(enabled, md, fuse.S_IFDIR|sRDONLY, 0, timestamp)
		fs.root.stat.Size += fs.populateFilesystem(enabled, timestamp, send...)

		if node, ok := fs.root.chld[enabled]; ok && api.IsHidden(enabled) {
			fs.hidden[node.stat.Ino] = true
		}

		logs.Infof("%s completed", enabled)
	}

	logs.Info("Filesystem database completed")
	return &fs
}

// MountFilesystem mounts filesystem 'fs' to directory 'mount'
func MountFilesystem(fs *Fuse, mount string) {
	host := fuse.NewFileSystemHost(fs)

	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
		options = append(options, "-o", "volname="+path.Base(mount))
		options = append(options, "-o", "attr_timeout=0")
		options = append(options, "-o", "iosize=262144") // Value not optimized
	} else if runtime.GOOS == "linux" {
		options = append(options, "-o", "attr_timeout=0") // This causes the fuse to call getattr between open and read
		options = append(options, "-o", "auto_unmount")
	} // Still needs windows options

	host.Mount(mount, options)
}

// populateFilesystem creates the nodes (files and directories) of the filesystem
func (fs *Fuse) populateFilesystem(repository string, timestamp fuse.Timespec, send ...chan<- LoadProjectInfo) (repSize int64) {
	projects, err := api.GetFirstLevel(repository)
	if err != nil {
		logs.Error(err)
		return
	}
	if len(projects) == 0 {
		logs.Errorf("No permissions found for %s", repository)
		return
	}

	var wg sync.WaitGroup
	forChannel := make(map[string][]api.Metadata)
	numJobs := 0
	mapLock := sync.RWMutex{}

	for i := range projects {
		wg.Add(1)

		// Remove characters which may interfere with filesystem structure
		projectSafe := removeInvalidChars(projects[i].Name)

		if projects[i].OrigName == "" {
			projects[i].OrigName = projects[i].Name
		}
		if projectSafe != projects[i].OrigName {
			projectSafe = renameDirectory(fs.root.chld[repository], repository, projectSafe)
		}

		projects[i].Name = projectSafe
		projectPath := repository + "/" + projectSafe

		// Create a project directory
		logs.Debugf("Creating directory %q", filepath.FromSlash(projectPath))
		fs.makeNode(projectPath, projects[i], fuse.S_IFDIR|sRDONLY, 0, timestamp)

		go func(project api.Metadata) {
			defer wg.Done()
			defer CheckPanic()

			logs.Debugf("Fetching data for %q", filepath.FromSlash(projectPath))
			containers, err := api.GetSecondLevel(repository, project.OrigName)

			if err != nil {
				logs.Error(err)
				return
			}
			if len(send) > 0 && !api.IsHidden(repository) {
				send[0] <- LoadProjectInfo{Repository: repository, Project: projectSafe, Count: len(containers)}
			}

			for i, c := range containers {
				containerSafe := removeInvalidChars(c.Name)

				if c.OrigName == "" {
					c.OrigName = c.Name
				}
				if containerSafe != c.OrigName {
					containerSafe = renameDirectory(fs.root.chld[repository].chld[projectSafe], projectSafe, containerSafe)
				}
				if len(send) > 0 && api.IsHidden(repository) {
					send[0] <- LoadProjectInfo{Repository: repository, Project: containerSafe, Count: 1}
				}

				containerPath := projectPath + "/" + containerSafe
				containers[i].Name = containerPath
				c.Name = containerSafe

				// Create a container directory
				logs.Debugf("Creating subdirectory %q", filepath.FromSlash(containerPath))
				fs.makeNode(containerPath, c, fuse.S_IFDIR|sRDONLY, 0, timestamp)
			}

			mapLock.Lock()
			forChannel[projectSafe] = containers // LOCK
			numJobs += len(containers)           // LOCK
			mapLock.Unlock()
		}(projects[i])
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
	return calculateFinalSize(fs.root.chld[repository], repository)
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
		objects, err := api.GetThirdLevel(nodes[0], nodes[1], nodes[2])
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
					objectSafe := removeInvalidChars(obj.Name, "/")
					objectPath := containerPath + "/" + objectSafe

					if obj.OrigName == "" {
						obj.OrigName = parts[len(parts)-1]
					}
					obj.Name = path.Base(objectSafe)

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
				dirSafe := removeInvalidChars(key, "/")
				p := containerPath + "/" + dirSafe
				md := api.Metadata{Bytes: value, Name: path.Base(dirSafe), OrigName: path.Base(key)}
				fs.makeNode(p, md, fuse.S_IFDIR|sRDONLY, 0, timestamp)
			}

			level++
		}

		if len(send) > 0 {
			nodes = split(containerPath)
			if api.IsHidden(nodes[0]) {
				send[0] <- LoadProjectInfo{Repository: nodes[0], Project: nodes[2], Count: 1}
			} else {
				send[0] <- LoadProjectInfo{Repository: nodes[0], Project: nodes[1], Count: 1}
			}
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

var removeInvalidChars = func(str string, ignore ...string) string {
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
func (fs *Fuse) lookupNode(n *node, path string) (prnt *node, node *node, dir bool, origPath []string) {
	prnt, node, dir = n, n, true
	parts := split(filepath.ToSlash(path))
	for i, c := range parts {
		if c != "" {
			prnt = node
			if node == nil {
				return
			}

			node = node.chld[c]
			if node != nil {
				origPath = append(origPath, node.originalName)
				dir = (fuse.S_IFDIR == node.stat.Mode&fuse.S_IFMT)
			} else if fs.hidden[prnt.stat.Ino] {
				for _, chld := range prnt.chld {
					var p []string
					prnt, node, dir, p = fs.lookupNode(chld, join(parts[i:]))
					if node != nil {
						origPath = append(origPath, chld.originalName)
						origPath = append(origPath, p...)
						return
					}
				}
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
		fs.openmap[n.stat.Ino] = openNode{node: n, path: origPath}
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

func (fs *Fuse) getNode(path string, fh uint64) openNode {
	if fh == ^uint64(0) {
		_, node, _, origPath := fs.lookupNode(fs.root, path)
		return openNode{node: node, path: origPath}
	}
	return fs.openmap[fh]
}

func (fs *Fuse) synchronize() func() {
	fs.lock.Lock()
	return func() {
		fs.lock.Unlock()
	}
}
