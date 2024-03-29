package filesystem

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/billziss-gh/cgofuse/fuse"
)

const sRDONLY = 00444
const numRoutines = 4

var signalBridge func()
var host *fuse.FileSystemHost

// Fuse stores the filesystem structure
type Fuse struct {
	fuse.FileSystemBase
	lock    sync.Mutex
	inoLock sync.RWMutex
	ino     uint64
	root    *node
	openmap map[uint64]nodeAndPath
	mount   string
}

// node represents one file or directory
type node struct {
	stat              fuse.Stat_t
	chld              map[string]*node
	opencnt           int
	originalName      string // so that api calls work
	decryptionChecked bool
	denied            bool
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

type Project struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
}

// SetSignalBridge initializes the signal which informs QML that program has paniced
func SetSignalBridge(fn func()) {
	signalBridge = fn
}

// CheckPanic recovers from panic if one occured. Used for GUI
var CheckPanic = func() {
	if signalBridge != nil {
		if err := recover(); err != nil {
			logs.Error(fmt.Errorf("Something went wrong when creating Data Gateway: %w",
				fmt.Errorf("%v\n\n%s", err, string(debug.Stack()))))
			// Send alert
			signalBridge()
		}
	}
}

// InitializeFileSystem initializes the in-memory filesystem database
var InitializeFilesystem = func(send func(Project)) *Fuse {
	logs.Info("Initializing in-memory Data Gateway database")
	timestamp := fuse.Now()
	fs := Fuse{}
	fs.ino++
	fs.openmap = map[uint64]nodeAndPath{}
	fs.root = newNode(fs.ino, fuse.S_IFDIR|sRDONLY, 0, 0, timestamp)
	fs.root.stat.Size = -1

	for _, enabled := range api.GetEnabledRepositories() {
		logs.Info("Beginning filling in ", strings.ReplaceAll(enabled, "-", " "))

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
			logs.Debugf("Creating directory %s", filepath.FromSlash(projectPath))
			_, projectSafe = fs.makeNode(fs.root.chld[enabled], project, projectPath, fuse.S_IFDIR|sRDONLY, timestamp)

			if send != nil {
				send(Project{Repository: enabled, Name: projectSafe})
			}
		}
	}

	return &fs
}

// MountFilesystem mounts filesystem 'fs' to directory 'mount'
func MountFilesystem(fs *Fuse, mount string) {
	host = fuse.NewFileSystemHost(fs)
	fs.mount = mount

	options := []string{}
	switch runtime.GOOS {
	case "darwin":
		options = append(options, "-o", "defer_permissions")
		options = append(options, "-o", "volname="+path.Base(mount))
		options = append(options, "-o", "attr_timeout=0") // This causes the fuse to call getattr between open and read
		options = append(options, "-o", "iosize=262144")  // Value not optimized
	case "linux":
		options = append(options, "-o", "attr_timeout=0") // This causes the fuse to call getattr between open and read
		options = append(options, "-o", "auto_unmount")
	case "windows":
		options = append(options, "-o", "umask=0222")
		options = append(options, "-o", "uid=-1")
		options = append(options, "-o", "gid=-1")
	}

	logs.Infof("Mounting Data Gateway at %s", mount)
	host.Mount(mount, options)
}

// UnmountFilesystem unmounts filesystem if host is defined
func UnmountFilesystem() {
	if host != nil {
		host.Unmount()
	}
}

// RefreshFilesystem clears cache and creates a new fileystem that will reflect any changes
// that have occurred in the repositories. Does not unmount fuse at any point.
func (fs *Fuse) RefreshFilesystem(initFunc func(Project), populateFunc func(string, string, int)) {
	logs.Info("Updating Data Gateway")
	api.ClearCache()

	newFs := InitializeFilesystem(initFunc)
	newFs.PopulateFilesystem(populateFunc)
	fs.ino = newFs.ino
	fs.root = newFs.root
	fs.openmap = newFs.openmap
}

// FilesOpen checks if any of the files are being used by the user
func (fs *Fuse) FilesOpen() bool {
	mount := fs.mount
	switch runtime.GOOS {
	case "linux":
		_, err := exec.Command("fuser", "-m", mount).Output()

		return err == nil
	case "darwin":
		output, err := exec.Command("fuser", "-c", mount).Output()
		if err != nil {
			logs.Errorf("Update halted, could not determine if files are open: %w", err)

			return true
		}

		return len(output) > 0
	case "windows":
		volume, _ := os.Readlink(fs.mount)
		output, err := exec.Command("handle.exe", "-a", "-nobanner", volume).Output()
		if err != nil {
			logs.Errorf("Update halted, could not determine if files are open: %w", err)

			return true
		}

		return strings.Contains(string(output), volume)
	default:
		for _, n := range fs.openmap {
			if n.node.stat.Mode&fuse.S_IFMT == fuse.S_IFREG {
				return true
			}
		}
	}

	return false
}

// ClearPath is desined for situations where a file is edited in the repository and the user wants to read this new data.
// Function clears cache for `path` and updates all its file sizes.
func (fs *Fuse) ClearPath(path string) error {
	logs.Infof("Clearing path %s", path)
	n := fs.getNode(path, ^uint64(0))
	if n.node == nil {
		return fmt.Errorf("Path %s is invalid", path)
	}

	if n.path[0] != api.SDConnect {
		return fmt.Errorf("Clearing cache only enabled for %s", api.SDConnect)
	}

	if len(n.path) < 3 {
		return fmt.Errorf("Path needs to include a bucket")
	}

	containerPath := strings.Join(strings.Split(path, string(os.PathSeparator))[:3], "/")
	objects, err := api.GetNthLevel(n.path[0], containerPath, n.path[1], n.path[2])
	if err != nil {
		return fmt.Errorf("Cache not cleared since new file sizes could not be obtained: %w", err)
	}

	objMap := map[string]int64{}
	for _, obj := range objects {
		objMap[obj.Name] = obj.Bytes
	}

	oldSize := n.node.stat.Size
	timestamp := fuse.Now()
	clearNode(n, objMap, timestamp)
	calculateFinalSize(n.node)
	fs.updateNodeSizesAlongPath(filepath.Dir(path), n.node.stat.Size-oldSize, timestamp)

	logs.Info("Path cleared")

	return nil
}

func clearNode(n nodeAndPath, meta map[string]int64, timestamp fuse.Timespec) {
	if n.node.stat.Mode&fuse.S_IFMT == fuse.S_IFREG {
		api.DeleteFileFromCache(n.path, n.node.stat.Size)
		size, ok := meta[strings.Join(n.path[3:], "/")]
		if ok {
			n.node.stat.Size = size
			n.node.decryptionChecked = false
			n.node.stat.Ctim = timestamp
		}

		return
	}

	if meta != nil {
		n.node.stat.Size = -1
	}

	for _, chld := range n.node.chld {
		clearNode(nodeAndPath{chld, append(n.path, chld.originalName)}, meta, timestamp)
	}
}

// PopulateFilesystem creates the rest of the nodes (files and directories) of the filesystem
func (fs *Fuse) PopulateFilesystem(send func(string, string, int)) {
	timestamp := fuse.Now()

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

				var err error
				var containers []api.Metadata
				projectPath := repository + "/" + project

				if repository == api.SDSubmit {
					// Project, container, and object are terms used in SD Connect.
					// SD Apply, on the other hand, uses datasets and files.
					// Since fetching files for a dataset requires only one http request,
					// one can think of datasets as equivalent to containers.
					// This means that projects do not have a corresponding level in SD Apply, so here we skip
					// filling in SD Apply while filling in projects.
					containers = []api.Metadata{{Name: projectPath}}
				} else {
					logs.Debugf("Fetching data for %s", filepath.FromSlash(projectPath))
					prntNode := fs.root.chld[repository].chld[project]
					containers, err = api.GetNthLevel(repository, projectPath, prntNode.originalName)

					if err != nil {
						logs.Error(err)

						return
					}

					for i, c := range containers {
						var mode uint32 = fuse.S_IFDIR | sRDONLY

						containerSafe := removeInvalidChars(c.Name)
						containerPath := projectPath + "/" + containerSafe

						// Create a file or a container (depending on repository)
						logs.Debugf("Creating directory %s", filepath.FromSlash(containerPath))
						_, containerSafe = fs.makeNode(prntNode, c, containerPath, mode, timestamp)
						containers[i].Name = projectPath + "/" + containerSafe
					}
				}

				mapLock.Lock()
				forChannel[projectPath] = containers // LOCK
				numJobs += len(containers)           // LOCK
				mapLock.Unlock()

				if send != nil {
					send(repository, project, len(containers))
				}
			}(rep, pr)
		}
	}

	wg.Wait()
	if send != nil {
		send("", "", 0) // So that progressbar knows when to start to show progress
	}

	jobs := make(chan containerInfo, numJobs)

	for w := 1; w <= numRoutines; w++ {
		wg.Add(1)
		go createObjects(w, jobs, &wg, send)
	}

	for _, value := range forChannel {
		for i := range value {
			jobs <- containerInfo{containerPath: value[i].Name, timestamp: timestamp, fs: fs}
		}
	}
	close(jobs)

	wg.Wait()

	// Calculate the size of higher level directories whose size currently is just -1.
	calculateFinalSize(fs.root)
	logs.Info("Data Gateway database completed")
}

var removeInvalidChars = func(str string) string {
	forReplacer := []string{"/", "_", "#", "_", "%", "_", "$", "_", "+",
		"_", "|", "_", "@", "_", ":", "_", "&", "_", "!", "_", "?", "_",
		"<", "_", ">", "_", "'", "_", "\"", "_"}

	// Remove characters which may interfere with filesystem structure
	r := strings.NewReplacer(forReplacer...)

	return r.Replace(str)
}

func calculateFinalSize(n *node) int64 {
	if n.stat.Size != -1 {
		return n.stat.Size
	}
	n.stat.Size = 0
	for _, value := range n.chld {
		n.stat.Size += calculateFinalSize(value)
	}

	return n.stat.Size
}

var createObjects = func(_ int, jobs <-chan containerInfo, wg *sync.WaitGroup, send func(string, string, int)) {
	defer wg.Done()
	defer CheckPanic()

	for j := range jobs {
		containerPath := j.containerPath
		fs := j.fs
		timestamp := j.timestamp

		logs.Debugf("Fetching data for %s", filepath.FromSlash(containerPath))

		c := fs.getNode(containerPath, ^uint64(0))
		if c.node == nil {
			logs.Errorf("Bug in code? Cannot find node %s", containerPath)

			continue
		}

		objects, err := api.GetNthLevel(c.path[0], containerPath, c.path[1:]...)
		if err != nil {
			logs.Error(err)

			continue
		}

		nodesSafe := split(containerPath)
		fs.createLevel(c.node, objects, containerPath, timestamp)

		if send != nil {
			send(nodesSafe[0], nodesSafe[1], 1)
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
			logs.Debugf("Creating file %s", filepath.FromSlash(objectPath))
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
		logs.Debugf("Creating directory %s", filepath.FromSlash(p))
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
		false,
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
	newName, origName := "", meta.Name
	if possibleTwin != nil {
		// Create a unique suffix for file/folder and change name of node (whichever is possibly a file)
		changeOtherNode := dir && (fuse.S_IFREG == possibleTwin.stat.Mode&fuse.S_IFMT)
		changeDir := dir && (fuse.S_IFREG != possibleTwin.stat.Mode&fuse.S_IFMT)

		if changeOtherNode {
			origName = possibleTwin.originalName
		}
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(origName)))[0:6]

		if changeDir {
			newName = fmt.Sprintf("%s(%s)", name, sum)
		} else {
			parts := strings.SplitN(name, ".", 2)
			parts[0] = fmt.Sprintf("%s(%s)", parts[0], sum)
			newName = strings.Join(parts, ".")
		}

		if changeOtherNode {
			prnt.chld[newName] = possibleTwin
		} else {
			name = newName
		}
	}

	// Because cli logs cannot print name out correctly without this fix
	if signalBridge == nil {
		origName = strings.ReplaceAll(origName, "%", "%%")
	}

	if dir && name != meta.Name {
		logs.Warningf("Directory %s under directory %s has had its name changed to %s", origName, prntPath, name)
	} else if (!dir && name != strings.TrimSuffix(meta.Name, ".c4gh")) || newName != "" {
		if newName == "" {
			newName = name
		}
		logs.Warningf("File %s under directory %s has had its name changed to %s", origName, prntPath, newName)
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
var lookupNode = func(root *node, path string) (node *node, origPath []string) {
	node = root
	for _, c := range split(path) {
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

func (fs *Fuse) updateNodeSizesAlongPath(path string, diff int64, timestamp fuse.Timespec) {
	node := fs.root
	for _, c := range split(path) {
		if c != "" {
			if node == nil {
				return
			}

			node = node.chld[c]
			if node != nil {
				node.stat.Size += diff
				node.stat.Ctim = timestamp
			}
		}
	}
}

func (fs *Fuse) openNode(path string, dir bool) (int, uint64) {
	n, origPath := lookupNode(fs.root, path)
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
	if node == nil {
		return -fuse.ENOENT
	}
	node.opencnt--
	if node.opencnt == 0 {
		delete(fs.openmap, node.stat.Ino)
	}

	return 0
}

func (fs *Fuse) getNode(path string, fh uint64) nodeAndPath {
	if fh == ^uint64(0) {
		node, origPath := lookupNode(fs.root, path)

		return nodeAndPath{node: node, path: origPath}
	}

	return fs.openmap[fh]
}

func (fs *Fuse) GetNodeChildren(path string) []string {
	n := fs.getNode(path, ^uint64(0))
	if n.node == nil {
		return nil
	}
	chld := make([]string, len(n.node.chld))
	i := 0
	for _, value := range n.node.chld {
		chld[i] = value.originalName
		i++
	}

	sort.Strings(chld)

	return chld
}

func (fs *Fuse) synchronize() func() {
	fs.lock.Lock()

	return func() {
		fs.lock.Unlock()
	}
}
