package filesystem

import (
	"fmt"
	"net/url"
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
	fs.root = newNode(fs.ino, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)

	for _, enabled := range api.GetEnabledRepositories() {
		logs.Info("Beginning filling in ", enabled)

		// Create folders for each repository
		md := api.Metadata{Name: enabled, Bytes: -1}
		fs.makeNode(fs.root, md, enabled, fuse.S_IFDIR|sRDONLY, timestamp)

		// These are the folders displayed in GUI
		projects, _ := api.GetNthLevel(enabled, enabled)
		for _, project := range projects {
			projectSafe := project.Name

			// This is mainly here because of SD Submit
			if u, err := url.ParseRequestURI(projectSafe); err == nil {
				projectSafe = strings.TrimLeft(strings.TrimPrefix(projectSafe, u.Scheme), ":/")
			}

			projectSafe = removeInvalidChars(projectSafe)
			projectPath := enabled + "/" + projectSafe

			// Create a project/dataset directory
			logs.Debugf("Creating directory %q", filepath.FromSlash(projectPath))
			_, projectSafe = fs.makeNode(fs.root.chld[enabled], project, projectPath, fuse.S_IFDIR|sRDONLY, timestamp)

			if len(send) > 0 {
				send[0] <- LoadProjectInfo{Repository: enabled, Project: projectSafe}
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
	} else if runtime.GOOS == "windows" {
		options = append(options, "-o", "umask=0222")
		options = append(options, "-o", "uid=-1")
		options = append(options, "-o", "gid=-1")
	}

	logs.Infof("Mounting filesystem at %q", mount)
	host.Mount(mount, options)
}

// PopulateFilesystem creates the rest of the nodes (files and directories) of the filesystem
func (fs *Fuse) PopulateFilesystem(send ...chan<- LoadProjectInfo) {
	timestamp := fuse.Now()
	fs.root.stat.Size = -1

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

				prntNode := fs.root.chld[repository].chld[project]

				projectPath := repository + "/" + project
				logs.Debugf("Fetching data for %q", filepath.FromSlash(projectPath))
				containers, err := api.GetNthLevel(repository, projectPath, prntNode.originalName)

				if err != nil {
					logs.Error(err)
					return
				}

				for i, c := range containers {
					var mode uint32 = fuse.S_IFREG | sRDONLY
					nodeType := "file"

					if api.LevelCount(repository) > 2 {
						mode = fuse.S_IFDIR | sRDONLY
						nodeType = "directory"
					}

					containerSafe := removeInvalidChars(c.Name)
					containerPath := projectPath + "/" + containerSafe

					// Create a file or a container (depending on repository)
					logs.Debugf("Creating %s %q", nodeType, filepath.FromSlash(containerPath))
					_, containerSafe = fs.makeNode(prntNode, c, containerPath, mode, timestamp)
					containers[i].Name = projectPath + "/" + containerSafe
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

var removeInvalidChars = func(str string) string {
	forReplacer := []string{"/", "_", "#", "_", "%", "_", "$", "_", "+",
		"_", "|", "_", "@", "_", ":", ".", "&", ".", "!", ".", "?", ".",
		"<", ".", ">", ".", "'", ".", "\"", "."}

	// Remove characters which may interfere with filesystem structure
	r := strings.NewReplacer(forReplacer...)
	return r.Replace(str)
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

		logs.Debugf("Fetching data for directory %q", filepath.FromSlash(containerPath))

		c := fs.getNode(containerPath, ^uint64(0))
		objects, err := api.GetNthLevel(c.path[0], containerPath, c.path[1], c.path[2])
		if err != nil {
			logs.Error(err)
			continue
		}

		nodesSafe := split(containerPath)
		fs.createLevel(c.node, objects, containerPath, timestamp)

		if len(send) > 0 {
			send[0] <- LoadProjectInfo{Repository: nodesSafe[0], Project: nodesSafe[1], Count: 1}
		}
	}
}

func (fs *Fuse) createLevel(prnt *node, objects []api.Metadata, prntPath string, tmsp fuse.Timespec) {
	dirSize := make(map[string]int64)
	dirChildren := make(map[string][]api.Metadata)

	for _, obj := range objects {
		// Prevent the creation of objects that are actually empty directories
		if strings.HasSuffix(obj.Name, "/") {
			continue
		}

		parts := strings.SplitN(obj.Name, "/", 2)

		// If true, create the final object file
		if len(parts) == 1 {
			objectSafe := removeInvalidChars(parts[0])
			objectPath := prntPath + "/" + objectSafe
			logs.Debugf("Creating file %q", filepath.FromSlash(objectPath))
			fs.makeNode(prnt, obj, objectPath, fuse.S_IFREG|sRDONLY, tmsp)
			continue
		}

		dirSize[parts[0]] += obj.Bytes
		dirChildren[parts[0]] = append(dirChildren[parts[0]], api.Metadata{Name: parts[1], Bytes: obj.Bytes})
	}

	// Create all unique subdirectories at this level
	for key, value := range dirSize {
		md := api.Metadata{Bytes: value, Name: key}
		dirSafe := removeInvalidChars(key)
		p := prntPath + "/" + dirSafe
		logs.Debugf("Creating directory %q", filepath.FromSlash(p))
		n, dirSafe := fs.makeNode(prnt, md, p, fuse.S_IFDIR|sRDONLY, tmsp)
		fs.createLevel(n, dirChildren[key], prntPath+"/"+dirSafe, tmsp)
	}
}

// split deconstructs a filepath string into an array of strings
func split(path string) []string {
	return strings.Split(path, "/")
}

// newNode initializes a node struct
var newNode = func(ino uint64, mode uint32, uid uint32, gid uint32, tmsp fuse.Timespec) *node {
	self := node{
		fuse.Stat_t{
			Dev:      0,
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

// makeNode adds a node into the fuse. Returns created node and its name in filesystem
func (fs *Fuse) makeNode(prnt *node, meta api.Metadata, nodePath string, mode uint32, timestamp fuse.Timespec) (*node, string) {
	name := path.Base(nodePath)
	dir := (fuse.S_IFDIR == mode&fuse.S_IFMT)
	prntPath := path.Dir(nodePath)

	if !dir {
		name = strings.TrimSuffix(name, ".c4gh")
	}

	possibleTwin := prnt.chld[name]

	// A folder or a file with the same name already exists
	if possibleTwin != nil {
		// Create a unique suffix for file/folder
		parts := strings.SplitN(name, ".", 2)
		i := 1
		for {
			beforeDot := parts[0]
			parts[0] = fmt.Sprintf("%s(%d)", parts[0], i)
			newName := strings.Join(parts, ".")
			parts[0] = beforeDot
			if _, ok := prnt.chld[newName]; !ok {
				// Change name of node (whichever is possibly a file)
				if dir && (fuse.S_IFREG == possibleTwin.stat.Mode&fuse.S_IFMT) {
					prnt.chld[newName] = possibleTwin
					logs.Warningf("File %q under directory %q has had its name changed to %q", possibleTwin.originalName, prntPath, newName)
				} else {
					name = newName
				}
				break
			}
			i++
		}
	}

	if dir && name != meta.Name {
		logs.Warningf("Directory %q under directory %q has had its name changed to %q", meta.Name, prntPath, name)
	} else if !dir && name != strings.TrimSuffix(meta.Name, ".c4gh") {
		logs.Warningf("File %q under directory %q has had its name changed to %q", meta.Name, prntPath, name)
	}

	fs.inoLock.Lock()
	fs.ino++                                    // LOCK
	n := newNode(fs.ino, mode, 0, 0, timestamp) // LOCK
	fs.inoLock.Unlock()

	n.stat.Size = meta.Bytes
	n.originalName = meta.Name
	prnt.chld[name] = n
	prnt.stat.Ctim = n.stat.Ctim
	prnt.stat.Mtim = n.stat.Ctim
	return n, name
}

// lookupNode finds the node at the end of path
func (fs *Fuse) lookupNode(path string) (node *node, origPath []string) {
	node = fs.root
	for _, c := range split(filepath.ToSlash(path)) {
		if c != "" {
			if node == nil {
				return
			}

			node = node.chld[c]
			if node != nil {
				origPath = append(origPath, node.originalName)
			}
		}
	}
	return
}

func (fs *Fuse) openNode(path string, dir bool) (int, uint64) {
	n, origPath := fs.lookupNode(path)
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
		node, origPath := fs.lookupNode(path)
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
