package filesystem

/*
#cgo CFLAGS: -DGO_CGO_BUILD -D_FILE_OFFSET_BITS=64 -DFUSE_DARWIN_ENABLE_EXTENSIONS=0 -g -Wall -Wextra -Wno-unused-parameter
#cgo pkg-config: fuse3
#cgo nocallback search_node
#cgo nocallback sort_node_children
#cgo nocallback free_nodes
#include <stdio.h>
#include <time.h>
#include <sys/stat.h>
#include "helpers.h"
#include "enabled.h"

enum { ptr_size = sizeof(void*) };

static node_t fuse_root = {0};

static inline node_t *get_root() {
    return &fuse_root;
}
*/
import "C"

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/sirupsen/logrus"
)

const numRoutines = 4
const segmentsSuffix = "_segments"

var signalBridge func()

var fi = fuseInfo{
	headers:   make(map[C.ino_t]header),
	forgotten: make(map[C.ino_t]*C.node_t),
}

// fuseInfo stores variables relevant to the filesystem
type fuseInfo struct {
	mount     string
	root      *goNode
	state     C.fs_state_t // Only needed for lock and session
	headers   map[C.ino_t]header
	forgotten map[C.ino_t]*C.node_t
	ready     chan<- any
	guiFun    func(api.Repo, string, int)
}

// bucketInfo is a packet of information sent through a channel to createObjects()
type bucketInfo struct {
	path       string
	bucketNode *goNode
	segmented  bool
}

type header struct {
	value  string
	owner  string
	fileID string
}

// goNode is a representation of a file/directory in Go
type goNode struct {
	meta     api.Metadata
	children map[string]*goNode
	cNode    *C.node_t // Helps keep track of pointers in C during filesystem updates
	rootPtr  *goNode   // So that we can keep track which nodes are persistent across updates.
}

// SetSignalBridge initializes the signal which informs Wails that program has paniced
func SetSignalBridge(fn func()) {
	signalBridge = fn
}

// checkPanic recovers from panic if one occured. Used for GUI
var checkPanic = func() {
	if signalBridge != nil {
		if err := recover(); err != nil {
			logs.Errorf("Something went wrong when creating Data Gateway: %w",
				fmt.Errorf("%v\n\n%s", err, string(debug.Stack())))
			// Send alert
			signalBridge()
		}
	}
}

// MountFilesystem mounts filesystem to directory `mount“
func MountFilesystem(mount string, fun func(api.Repo, string, int), ready chan<- any) int {
	logs.Infof("Mounting Data Gateway at %s", mount)
	fi.mount = mount
	fi.ready = ready
	fi.guiFun = fun

	fi.state.root = C.get_root() // C pointer to root node is constant
	GenerateFilesystem()

	m := C.CString(mount)
	defer C.free(unsafe.Pointer(m)) //nolint:nlreturn
	defer C.fflush(nil)

	var fuseDebug C.int
	if logs.GetLevel() == logrus.TraceLevel {
		fuseDebug = 1
	}

	return int(C.mount_filesystem(&fi.state, m, fuseDebug)) //nolint:gocritic,nlreturn
}

// allocateNodeList reserves memory for an array of node pointers in C.
// This is a separate Go function so it can be used in tests
func allocateNodeList(length C.size_t) **C.node_t {
	return (**C.node_t)(C.malloc(C.ptr_size * length))
}

// allocateNodePointer reserves memory for a single node pointer in C
// and initializes the memory with zeroes.
// This is a separate Go function so it can be used in tests
func allocateNodePointer() *C.node_t {
	return (*C.node_t)(C.calloc(1, C.sizeof_struct_Node))
}

// GenerateFilesystem creates the in-memory filesystem. It is also used for filesystem updates
func GenerateFilesystem() {
	logs.Info("Initializing in-memory Data Gateway filesystem")
	defer checkPanic()

	//nolint:nlreturn
	// First get the metadata from S3 and build a tree in Go to represent the filesystem structure
	oldRoot := fi.root
	fi.root = newGoNode(api.Metadata{Name: "", Size: 0, LastModified: nil}, true)
	fi.root.cNode = C.get_root()
	if oldRoot != nil {
		fi.root.children = oldRoot.children
	}

	numJobs := 0
	bucketNodes := make(map[string]map[string]*goNode)
	repositories := api.GetRepositories()

	for _, rep := range repositories {
		logs.Infof("Beginning filling in %s", rep)

		// Create folder for repository (SD Connect/SD Apply)
		meta := api.Metadata{Name: rep.ForPath(), Size: 0, LastModified: nil}
		makeNode(fi.root.children, meta, true, false, "")

		// Get buckets for repository
		buckets, err := api.GetBuckets(rep)
		if err != nil {
			logs.Error(err)
			keepContents(fi.root.children[rep.ForPath()], rep.ForPath(), err)

			continue
		}

		buckets, segmentBuckets := separateSegmentBuckets(buckets)
		numJobs += len(buckets)

		parentChildren := fi.root.children[rep.ForPath()].children
		parentPath := rep.ForPath()

		// Create project directory for SD Connect
		if rep == api.SDConnect {
			project := api.GetProjectName()
			projectPath := parentPath + "/" + project
			logs.Debugf("Creating directory %s", filepath.FromSlash(projectPath))
			meta := api.Metadata{Name: project, Size: 0, LastModified: nil}
			makeNode(parentChildren, meta, true, false, parentPath)

			parentChildren = parentChildren[project].children
			parentPath = projectPath

			if fi.guiFun != nil {
				fi.guiFun(rep, project, len(buckets))
			}
		}

		bucketNodes[parentPath] = make(map[string]*goNode, len(buckets))

		// Create bucket directories
		for i := range buckets {
			bucketPath := parentPath + "/" + buckets[i].Name
			logs.Debugf("Creating directory %s", filepath.FromSlash(bucketPath))
			bucketSafe := makeNode(parentChildren, buckets[i], true, true, parentPath)

			bucketNodes[parentPath][bucketSafe] = parentChildren[bucketSafe]

			if rep != api.SDConnect && fi.guiFun != nil {
				fi.guiFun(rep, bucketSafe, 1)
			}
		}

		for i := range segmentBuckets {
			bucketName := strings.TrimSuffix(segmentBuckets[i].Name, segmentsSuffix)
			// S3 buckets assumed to not have any weird characters so the orignal name and
			// safe name should be the same
			// SD Apply datasets that represent buckets in this filesystem are assumed to
			// not be segmented
			if node, ok := parentChildren[bucketName]; ok {
				node.meta.Name = segmentBuckets[i].Name
			}
		}
	}

	if fi.guiFun != nil {
		fi.guiFun("", "", 0) // So that progressbar knows when to start to show progress
	}

	var wg sync.WaitGroup
	jobs := make(chan bucketInfo, numJobs)

	for w := 1; w <= numRoutines; w++ {
		wg.Add(1)
		go createObjects(w, jobs, &wg)
	}

	for path := range bucketNodes {
		for nameSafe, node := range bucketNodes[path] {
			segmented := strings.HasSuffix(node.meta.Name, segmentsSuffix)
			node.meta.Name = strings.TrimSuffix(node.meta.Name, segmentsSuffix)
			jobs <- bucketInfo{"/" + path + "/" + nameSafe, node, segmented}
		}
	}
	close(jobs)

	wg.Wait()

	// Construct the tree of nodes for C
	goNodesToC(fi.root, "")
	fi.root.cNode.stat.st_ino = 1 // FUSE_ROOT_ID

	logs.Info("Data Gateway filesystem completed")

	select {
	case fi.ready <- nil:
	default:
	}
}

func separateSegmentBuckets(buckets []api.Metadata) ([]api.Metadata, []api.Metadata) {
	lastIdx := len(buckets) - 1
	for i := 0; i <= lastIdx; i++ {
		if strings.HasSuffix(buckets[i].Name, segmentsSuffix) {
			for ; lastIdx > i; lastIdx-- {
				if !strings.HasSuffix(buckets[lastIdx].Name, segmentsSuffix) {
					break
				}
			}
			buckets[i], buckets[lastIdx] = buckets[lastIdx], buckets[i]
			lastIdx--
		}
	}

	return buckets[:lastIdx+1], buckets[lastIdx+1:]
}

// goNodesToC sets up the field `cNode` under `node`. The fuse operations
// have a C pointer to the root so when we initialize the `cNode` field here,
// the changes will be visible in C. `cNode` fields may already be set if
// goNodesToC is called during a filesystem update. The function also fills in
// the array of children for `cNode`, recursively initializing the children as well.
func goNodesToC(node *goNode, name string) {
	cNode := node.cNode
	// ino will later give us the pointer to the node directly
	ino := C.ino_t(uintptr(unsafe.Pointer(cNode)))
	cNode.stat.st_ino = ino

	if cNode.name == nil {
		cNode.name = C.CString(name)
	}
	if cNode.orig_name == nil {
		cNode.orig_name = C.CString(node.meta.Name)
	}

	// Prune old nodes that no longer exist in storage
	for name, child := range node.children {
		if child.rootPtr != fi.root {
			collectForgotten(child.cNode)
			delete(node.children, name)
		}
	}

	oldLastModified := cNode.last_modified.tv_sec

	if node.children == nil {
		// File node
		cNode.stat.st_mode = syscall.S_IFREG | 0644
		cNode.stat.st_nlink = 1

		newLastModified := C.time_t(node.meta.LastModified.Unix())
		// We do not want to update size if the timestamp has not changed
		// since the size that comes from S3 is the encrypted size and
		// node's size might have changed to be the decrypted size
		// depending on whether CheckHeaderExistence has been called for the node
		if newLastModified != oldLastModified {
			cNode.last_modified.tv_sec = newLastModified
			cNode.stat.st_size = C.off_t(node.meta.Size)
			cNode.offset = -1
		}

		oldHeader := fi.headers[ino]
		if oldHeader.owner != node.meta.Owner || oldHeader.fileID != node.meta.ID {
			fi.headers[ino] = header{owner: node.meta.Owner, fileID: node.meta.ID}
		}
	} else {
		// Directory node
		childCount := C.size_t(len(node.children))
		cNode.stat.st_mode = syscall.S_IFDIR | 0744
		cNode.stat.st_nlink = C.nlink_t(2 + childCount)
		cNode.stat.st_size = 0
		// Timestamp is recalculated from the children so that it can also move backwards
		// if the newest child has been removed. An empty directory has timestamp 0.
		cNode.last_modified.tv_sec = 0

		C.free(unsafe.Pointer(cNode.children))
		cNode.children = allocateNodeList(childCount)
		cNode.chld_count = childCount
		nodeSlice := unsafe.Slice(cNode.children, childCount)

		i := 0
		for name, child := range node.children {
			if child.cNode == nil {
				child.cNode = allocateNodePointer()
			}
			goNodesToC(child, name)
			child.cNode.parent = cNode
			nodeSlice[i] = child.cNode

			// Now we update the parent node's timestamp and size
			cNode.stat.st_size += nodeSlice[i].stat.st_size
			modified := nodeSlice[i].last_modified.tv_sec
			if modified > cNode.last_modified.tv_sec {
				cNode.last_modified.tv_sec = modified
			}

			i++
		}
	}

	if oldLastModified != 0 && oldLastModified != cNode.last_modified.tv_sec {
		ret := C.invalidate_node(&fi.state, cNode) //nolint:gocritic,nlreturn
		path, pathNames := getNodePath(cNode)
		if ret != 0 {
			logs.Warningf("kernel was not able to invalidate node %s: %v", path, syscall.Errno(-ret))
		}
		if node.children == nil && api.Repo(pathNames[1]) == api.SDConnect {
			api.DeleteFileFromCache(api.SDConnect, pathNames, int64(cNode.stat.st_size))
		}
	}

	// Making sure sorting and searching use the same comparison function (the one in C)
	C.sort_node_children(cNode)
}

// collectForgotten is called when a node and all its children need to be removed from kernel cache.
// It assigns offset as -2 so that any possible lookup() calls dismiss these nodes outright.
func collectForgotten(node *C.node_t) {
	node.offset = -2
	fi.forgotten[node.stat.st_ino] = node
	for _, child := range unsafe.Slice(node.children, node.chld_count) {
		collectForgotten(child)
	}
}

var createObjects = func(_ int, jobs <-chan bucketInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	defer checkPanic()

	for j := range jobs {
		node := j.bucketNode
		path := j.path
		segmented := j.segmented

		nodesSafe := strings.Split(path, "/")
		repository := api.Repo(nodesSafe[1])

		logs.Debugf("Fetching data for %s", filepath.FromSlash(path))
		objects, err := api.GetObjects(repository, node.meta.Name, path, node.meta.Owner, "")
		if err != nil {
			logs.Error(err)
			keepContents(node, path, err)

			continue
		}
		segmentSizes := map[string]int64{}
		if segmented {
			segmentSizes, err = getObjectSizesFromSegments(repository, node.meta.Name)
			if err != nil {
				logs.Warningf("Object sizes may not be correct: %s", err.Error())
			}
		}

		meta := make([]api.Metadata, 0, len(objects))
		for i := range objects {
			// Prevent the creation of objects that are actually empty directories
			// Not sure if this is still a problem once we move to S3
			if strings.HasSuffix(objects[i].Name, "/") {
				continue
			}

			objects[i].Owner = node.meta.Owner
			if objects[i].Size == 0 {
				objects[i].Size = segmentSizes[objects[i].Name]
			}

			meta = append(meta, objects[i])
		}

		createLevel(node, meta, path)

		if fi.guiFun != nil {
			fi.guiFun(repository, nodesSafe[2], 1)
		}
	}
}

// keepContents is called when the contents of `node` could not be retrieved from storage during a filesystem update.
// Unless `err` indicates that the contents are no longer accessible, the contents from the previous update are kept
// so that a temporary error does not make files disappear from the filesystem.
func keepContents(node *goNode, path string, err error) {
	if len(node.children) == 0 || contentsInaccessible(err) {
		return
	}

	logs.Warningf("Showing previous contents of %s since they could not be updated", filepath.FromSlash(path))
	for _, child := range node.children {
		markPersistent(child)
	}
}

// contentsInaccessible checks `err` to see if it indicates that the data is definetly no longer
// accessible to the user
func contentsInaccessible(err error) bool {
	if errors.Is(err, api.ErrNoBuckets) || errors.Is(err, api.ErrInvalidBucketName) {
		return true
	}

	var se interface{ HTTPStatusCode() int }
	if !errors.As(err, &se) {
		return false
	}

	return se.HTTPStatusCode() == http.StatusForbidden || se.HTTPStatusCode() == http.StatusNotFound
}

// markPersistent marks `node` and all its descendants as still existing in storage
// so that they are not removed from the filesystem
func markPersistent(node *goNode) {
	node.rootPtr = fi.root
	for _, child := range node.children {
		markPersistent(child)
	}
}

// getObjectSizesFromSegments is used for getting the object sizes for buckets that
// have a matching segments bucket.
var getObjectSizesFromSegments = func(rep api.Repo, bucket string) (map[string]int64, error) {
	logs.Debugf("Fetching possible object sizes for bucket %s from matching segments bucket", bucket)
	objects, err := api.GetSegmentedObjects(rep, bucket+segmentsSuffix)
	if err != nil {
		return map[string]int64{}, fmt.Errorf("cannot fetch object sizes for bucket %s in %s: %w", bucket, rep, err)
	}

	objectSizes := make(map[string]int64)
	for i := range objects {
		pathNames := strings.Split(objects[i].Name, "/")
		actualObjectName := strings.Join(pathNames[:max(len(pathNames)-2, 0)], "/")
		objectSizes[actualObjectName] += objects[i].Size
	}

	return objectSizes, nil
}

func createLevel(prnt *goNode, meta []api.Metadata, prntPath string) {
	childrenMeta := make(map[string][]api.Metadata)

	for i := range meta {
		parts := strings.SplitN(meta[i].Name, "/", 2)

		// If true, create the final object file
		if len(parts) == 1 {
			logs.Debugf("Creating file %s", filepath.FromSlash(prntPath+"/"+meta[i].Name))
			makeNode(prnt.children, meta[i], false, false, prntPath)

			continue
		}

		meta[i].Name = parts[1]
		childrenMeta[parts[0]] = append(childrenMeta[parts[0]], meta[i])
	}

	// Create all unique subdirectories at this level
	for key, value := range childrenMeta {
		md := api.Metadata{Name: key, Size: 0, LastModified: nil}
		logs.Debugf("Creating directory %s", filepath.FromSlash(prntPath+"/"+key))
		dirSafe := makeNode(prnt.children, md, true, false, prntPath)
		createLevel(prnt.children[dirSafe], value, prntPath+"/"+dirSafe)
	}
}

// makeNode adds a node into the Go tree structure. Returns node's name in filesystem.
func makeNode(siblings map[string]*goNode, meta api.Metadata, isDir, isBucket bool, pathSafe string) string {
	name := meta.Name
	if isBucket {
		// SD Apply datasets may be URLs; SD Connect buckets are never URLs, so no need to worry about them running through here
		if u, err := url.ParseRequestURI(meta.Name); err == nil {
			name = strings.TrimLeft(strings.TrimPrefix(name, u.Scheme), ":/")
		}
	}
	name = removeInvalidChars(name) // Redundant for SD Connect buckets and projects
	if !isDir {
		name = strings.TrimSuffix(name, ".c4gh")
	}

	// possibleMatch is either the node itself if the filesystem is currently being updated,
	// or it can be a new node that just has the same safe name
	possibleMatch, ok := siblings[name]

	// Node already existed
	if reuseNode(possibleMatch, meta, isDir) {
		return name
	}

	// A folder or a file with the same safe name already exists
	newName, origName := "", meta.Name
	if ok {
		twin := possibleMatch
		// Create a unique suffix for file/folder and change name of node (whichever is possibly a file)
		// Do not change other node if it is from a previous iteration so that safe names stay the same across updates
		changeOtherNode := isDir && twin.children == nil && twin.cNode == nil
		changeDir := isDir && !changeOtherNode

		if changeOtherNode {
			origName = twin.meta.Name
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
			siblings[newName] = twin
		} else {
			name = newName
			// Node already existed under the unique name in a previous iteration
			if reuseNode(siblings[name], meta, isDir) {
				return name
			}
		}
	}

	if isDir && name != meta.Name {
		logs.Warningf("Directory %q under directory %s has had its name changed to %q", meta.Name, pathSafe, name)
	} else if (!isDir && name != strings.TrimSuffix(meta.Name, ".c4gh")) || newName != "" {
		if newName == "" {
			newName = name
		}
		logs.Warningf("File %q under directory %s has had its name changed to %q", origName, pathSafe, newName)
	}

	siblings[name] = newGoNode(meta, isDir)

	return name
}

// reuseNode checks whether `node` represents the same file/directory as `meta`.
// If it does, the node is updated with `meta` and marked as persistent.
func reuseNode(node *goNode, meta api.Metadata, isDir bool) bool {
	if node == nil || node.meta.Name != meta.Name || (node.children != nil) != isDir {
		return false
	}

	node.meta = meta
	node.rootPtr = fi.root

	return true
}

func newGoNode(meta api.Metadata, isDir bool) *goNode {
	node := goNode{meta, nil, nil, fi.root}
	if isDir {
		node.children = make(map[string]*goNode)
	}

	return &node
}

var invalidCharReplacer = strings.NewReplacer(
	"/", "_", "#", "_", "%", "_", "$", "_", "+", "_",
	"|", "_", "@", "_", ":", "_", "&", "_", "!", "_",
	"?", "_", "<", "_", ">", "_", "'", "_", "\"", "_",
)

var removeInvalidChars = func(str string) string {
	// Remove characters which may interfere with filesystem structure
	return invalidCharReplacer.Replace(str)
}
