package filesystem

/*
#cgo CFLAGS: -D GO_CGO_BUILD -D_FILE_OFFSET_BITS=64 -g -Wall -Wextra -Wno-unused-parameter
#cgo linux pkg-config: fuse3
#cgo darwin pkg-config: fuse
#cgo nocallback search_node
#cgo nocallback sort_node_children
#cgo nocallback free_nodes
#include <stdio.h>
#include <time.h>
#include <sys/stat.h>
#include "helpers.h"
#include "enabled.h"

static nodes_t fuse_nodes = {0};

static inline nodes_t *get_nodes() {
    return &fuse_nodes;
}
*/
import "C"

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strconv"
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

var fi = fuseInfo{}

// fuseInfo stores variables relevant to the filesystem
type fuseInfo struct {
	mount   string
	headers map[C.ino_t]string
	nodes   *C.nodes_t
	ready   chan<- any
	guiFun  func(string, string, int)
	mu      sync.RWMutex
}

// bucketInfo is a packet of information sent through a channel to createObjects()
type bucketInfo struct {
	path       string
	headers    map[string]api.VaultHeaderVersions
	bucketNode *goNode
	segmented  bool
}

// goNode is a representation of a file/directory in Go before it is moved to C
type goNode struct {
	meta     api.Metadata
	header   string
	children map[string]*goNode
}

type metadata struct {
	api.Metadata
	header      string
	segmentSize int64
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

// MountFilesystem mounts filesystem to directory 'mount'
func MountFilesystem(mount string, fun func(string, string, int), ready chan<- any) int {
	logs.Infof("Mounting Data Gateway at %s", mount)
	fi.mount = mount
	fi.ready = ready
	fi.guiFun = fun
	fi.mu = sync.RWMutex{}

	fi.mu.Lock()
	fi.nodes = C.get_nodes()
	InitialiseFilesystem()

	m := C.CString(mount)
	defer C.free(unsafe.Pointer(m)) //nolint:nlreturn
	defer C.fflush(nil)

	fuseDebug := 0
	if logs.GetLevel() == logrus.TraceLevel {
		fuseDebug = 1
	}

	return int(C.mount_filesystem(m, C.int(fuseDebug)))
}

//export GetFilesystem
func GetFilesystem() *C.nodes_t {
	fi.mu.Unlock()

	return fi.nodes
}

// allocateNodeList reserves memory for a node struct array in C.
// This is a separate Go function so it can be used in tests
func allocateNodeList(length int) *C.node_t {
	return (*C.node_t)(C.malloc(C.sizeof_struct_Node * C.size_t(length)))
}

// InitialiseFilesystem initializes the in-memory filesystem database
func InitialiseFilesystem() {
	logs.Info("Initializing in-memory Data Gateway database")
	defer checkPanic()

	// First get the metadata from S3 and build a tree in Go to represent the filesystem structure
	root := newGoNode(api.Metadata{Name: "", Size: 0, LastModified: nil}, true)

	numJobs := 0
	bucketNodes := make(map[string]map[string]*goNode)
	batchHeaders := make(map[string]api.BatchHeaders)
	repositories := api.GetRepositories()

	for _, rep := range repositories {
		logs.Info("Beginning filling in ", api.ToPrint(rep))

		// Create folder for repository
		meta := api.Metadata{Name: rep, Size: 0, LastModified: nil}
		makeNode(root.children, meta, true, "")

		// Get buckets for repository
		buckets, err := api.GetBuckets(rep)
		if err != nil {
			logs.Error(err)

			continue
		}
		buckets, segmentBuckets := separateSegmentBuckets(buckets)

		numJobs += len(buckets)

		parentChildren := root.children[rep].children
		parentPath := rep

		if rep == api.SDConnect {
			project := api.GetProjectName()
			projectPath := rep + "/" + project
			logs.Debugf("Creating directory /%s", filepath.FromSlash(projectPath))
			meta := api.Metadata{Name: project, Size: 0, LastModified: nil}
			makeNode(parentChildren, meta, true, api.SDConnect)

			parentPath = projectPath
			parentChildren = parentChildren[project].children

			if fi.guiFun != nil {
				fi.guiFun(rep, project, len(buckets))
			}
		}

		// Fetching headers
		headers, err := api.GetHeaders(rep, buckets)
		if err != nil {
			logs.Errorf("Failed to retrieve file headers for %s: %s", api.ToPrint(rep), err.Error())

			continue
		}
		logs.Infof("Retrieved file headers for %s", api.ToPrint(rep))
		batchHeaders[parentPath] = headers

		bucketNodes[parentPath] = make(map[string]*goNode, len(buckets))

		// Create bucket directories
		for i := range buckets {
			bucketPath := parentPath + "/" + buckets[i].Name
			logs.Debugf("Creating directory /%s", filepath.FromSlash(bucketPath))
			bucketSafe := makeNode(parentChildren, buckets[i], true, parentPath)

			bucketNodes[parentPath][bucketSafe] = parentChildren[bucketSafe]

			if rep != api.SDConnect && fi.guiFun != nil {
				fi.guiFun(rep, bucketSafe, 1)
			}
		}

		for i := range segmentBuckets {
			bucketName := strings.TrimSuffix(segmentBuckets[i].Name, segmentsSuffix)
			// Buckets assumed to not have any weird characters so the orignal name and
			// safe name should be the same
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
			jobs <- bucketInfo{"/" + path + "/" + nameSafe, batchHeaders[path][node.meta.Name], node, segmented}
		}
	}
	close(jobs)

	wg.Wait()

	// Construct the array of nodes for C
	num, objs := numberOfNodes(root)
	fi.headers = make(map[C.ino_t]string, objs)
	fi.nodes.nodes = allocateNodeList(num)
	fi.nodes.count = 1
	nodeSlice := unsafe.Slice(fi.nodes.nodes, num)
	nodeSlice[0] = goNodeToC(root, "")
	nodeSlice[0].parent = nil
	addNodeChildrenToC(nodeSlice, root, 0)

	logs.Info("Data Gateway database completed")

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

func numberOfNodes(node *goNode) (numNodes int, numObjects int) {
	if node.children == nil {
		return 1, 1
	}

	numNodes++
	for i := range node.children {
		num, obj := numberOfNodes(node.children[i])
		numNodes += num
		numObjects += obj
	}

	return
}

func goNodeToC(node *goNode, name string) C.node_t {
	cNode := C.node_t{}
	cNode.orig_name = C.CString(node.meta.Name)
	cNode.name = C.CString(name)
	cNode.stat.st_size = C.off_t(node.meta.Size)
	cNode.chld_count = 0
	cNode.children = nil
	cNode.last_modified.tv_sec = 0
	cNode.last_modified.tv_nsec = 0
	cNode.offset = -1

	if node.meta.LastModified != nil {
		cNode.last_modified.tv_sec = C.time_t(node.meta.LastModified.Unix())
	}

	if node.children == nil {
		cNode.stat.st_mode = syscall.S_IFREG | 0444
		cNode.stat.st_nlink = 1
	} else {
		cNode.stat.st_mode = syscall.S_IFDIR | 0444
		cNode.stat.st_nlink = C.nlink_t(2 + len(node.children))
	}

	return cNode
}

// addNodeChildrenToC adds the children of `prnt` to slice `nodeSlice`, which represents the array of nodes in C.
func addNodeChildrenToC(nodeSlice []C.node_t, prnt *goNode, prntIdx C.int64_t) (C.off_t, C.time_t) {
	startIdx := fi.nodes.count
	chldCount := C.int64_t(len(prnt.children))

	nodeSlice[prntIdx].chld_count = chldCount
	nodeSlice[prntIdx].children = &nodeSlice[startIdx] // Pointer to first child in array

	// Make sure all the children are placed sequentially in the array
	fi.nodes.count += chldCount

	i := startIdx
	for name, chld := range prnt.children {
		nodeSlice[i] = goNodeToC(chld, name)
		nodeSlice[i].parent = &nodeSlice[prntIdx]
		i++
	}

	// Making sure sorting and searching use the same comparison function (the one in C)
	C.sort_node_children(&nodeSlice[prntIdx])

	i = startIdx
	for ; i < startIdx+chldCount; i++ {
		ino := C.ino_t(i)
		nodeSlice[i].stat.st_ino = ino // ino is equivalent to the node's index in the array
		chld := prnt.children[C.GoString(nodeSlice[i].name)]

		size := nodeSlice[i].stat.st_size
		modified := nodeSlice[i].last_modified.tv_sec

		if len(chld.children) > 0 {
			size, modified = addNodeChildrenToC(nodeSlice, chld, i)
		} else if chld.header != "" {
			fi.headers[ino] = chld.header
		}

		nodeSlice[prntIdx].stat.st_size += size
		if modified > nodeSlice[prntIdx].last_modified.tv_sec {
			nodeSlice[prntIdx].last_modified.tv_sec = modified
		}
	}

	return nodeSlice[prntIdx].stat.st_size, nodeSlice[prntIdx].last_modified.tv_sec
}

var createObjects = func(_ int, jobs <-chan bucketInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	defer checkPanic()

	for j := range jobs {
		node := j.bucketNode
		path := j.path
		headers := j.headers
		segmented := j.segmented

		nodesSafe := strings.Split(path, "/")
		repository := nodesSafe[1]

		logs.Debugf("Fetching data for %s", filepath.FromSlash(path))
		objects, err := api.GetObjects(repository, node.meta.Name, path)
		if err != nil {
			logs.Error(err)

			continue
		}
		segmentSizes := map[string]int64{}
		if segmented {
			segmentSizes, err = getObjectSizesFromSegments(repository, node.meta.Name)
			if err != nil {
				logs.Warningf("Object sizes may not be correct: %s", err.Error())
			}
		}

		meta := make([]metadata, 0, len(objects))
		for i := range objects {
			// Prevent the creation of objects that are actually empty directories
			// Not sure if this is still a problem
			if strings.HasSuffix(objects[i].Name, "/") {
				continue
			}

			header := ""
			versions, ok := headers[objects[i].Name]
			if ok {
				header = versions.Headers[strconv.Itoa(versions.LatestVersion)].Header
			}
			meta = append(meta, metadata{objects[i], header, segmentSizes[objects[i].Name]})
		}

		createLevel(node, meta, path)

		if fi.guiFun != nil {
			fi.guiFun(repository, nodesSafe[2], 1)
		}
	}
}

// getObjectSizesFromSegments is used for getting the object sizes for buckets that
// have a matching segments bucket.
var getObjectSizesFromSegments = func(rep, bucket string) (map[string]int64, error) {
	logs.Debugf("Fetching possible object sizes for bucket %s from matching segments bucket", rep+"/"+bucket)
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

func createLevel(prnt *goNode, meta []metadata, prntPath string) {
	childrenMeta := make(map[string][]metadata)

	for i := range meta {
		parts := strings.SplitN(meta[i].Name, "/", 2)

		// If true, create the final object file
		if len(parts) == 1 {
			logs.Debugf("Creating file %s", filepath.FromSlash(prntPath+"/"+meta[i].Name))
			objectSafe := makeNode(prnt.children, meta[i].Metadata, false, prntPath)

			prnt.children[objectSafe].header = meta[i].header
			if prnt.children[objectSafe].meta.Size == 0 {
				prnt.children[objectSafe].meta.Size = meta[i].segmentSize
			}

			continue
		}

		meta[i].Name = parts[1]
		childrenMeta[parts[0]] = append(childrenMeta[parts[0]], meta[i])
	}

	// Create all unique subdirectories at this level
	for key, value := range childrenMeta {
		md := api.Metadata{Name: key, Size: 0, LastModified: nil}
		logs.Debugf("Creating directory %s", filepath.FromSlash(prntPath+"/"+key))
		dirSafe := makeNode(prnt.children, md, true, prntPath)
		createLevel(prnt.children[dirSafe], value, prntPath+"/"+dirSafe)
	}
}

// makeNode adds a node into the Go tree structure. Returns node's name in filesystem.
func makeNode(siblings map[string]*goNode, meta api.Metadata, isDir bool, pathSafe string) string {
	name := removeInvalidChars(meta.Name) // Redundant for buckets and projects
	if !isDir {
		name = strings.TrimSuffix(name, ".c4gh")
	}

	possibleTwin, ok := siblings[name]

	// A folder or a file with the same name already exists
	newName, origName := "", meta.Name
	if ok {
		// Create a unique suffix for file/folder and change name of node (whichever is possibly a file)
		changeOtherNode := isDir && possibleTwin.children == nil
		changeDir := isDir && possibleTwin.children != nil

		if changeOtherNode {
			origName = possibleTwin.meta.Name
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
			siblings[newName] = possibleTwin
		} else {
			name = newName
		}
	}

	if isDir && name != meta.Name {
		logs.Warningf("Directory %s under directory %s has had its name changed to %s", meta.Name, pathSafe, name)
	} else if (!isDir && name != strings.TrimSuffix(meta.Name, ".c4gh")) || newName != "" {
		if newName == "" {
			newName = name
		}
		logs.Warningf("File %s under directory %s has had its name changed to %s", origName, pathSafe, newName)
	}

	siblings[name] = newGoNode(meta, isDir)

	return name
}

func newGoNode(meta api.Metadata, isDir bool) *goNode {
	node := goNode{meta, "", nil}
	if isDir {
		node.children = make(map[string]*goNode)
	}

	return &node
}

var removeInvalidChars = func(str string) string {
	forReplacer := []string{"/", "_", "#", "_", "%", "_", "$", "_", "+",
		"_", "|", "_", "@", "_", ":", "_", "&", "_", "!", "_", "?", "_",
		"<", "_", ">", "_", "'", "_", "\"", "_"}

	// Remove characters which may interfere with filesystem structure
	r := strings.NewReplacer(forReplacer...)

	return r.Replace(str)
}
