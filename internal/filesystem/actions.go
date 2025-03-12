package filesystem

/*
#include <stdio.h>
#include <sys/stat.h>
#include "helpers.h"
#include "enabled.h"
*/
import "C"

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/neicnordic/crypt4gh/model/headers"
)

func searchNode(path string) *C.node_t {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	return C.search_node(fi.nodes, cpath)
}

// getPathNodeNames returns a slice of strings containing the original names of nodes along the path to `node`
func getNodePathNames(node *C.node_t) []string {
	names := []string{}
	for prnt := node; prnt != nil; prnt = prnt.parent {
		names = append(names, C.GoString(prnt.orig_name))
	}
	slices.Reverse(names)

	return names
}

// GetNodeChildren returns the children of node at the end of `path`
func GetNodeChildren(path string) []string {
	node := searchNode(path)
	if node == nil {
		return nil
	}

	goNodes := unsafe.Slice(node.children, node.chld_count)
	children := make([]string, node.chld_count)

	for i := range int(node.chld_count) {
		children[i] = C.GoString(goNodes[i].orig_name)
	}

	return children
}

func updateParentSizes(node *C.node_t, oldSize C.off_t) {
	diff := node.stat.st_size - oldSize
	for prnt := node.parent; prnt != nil; prnt = prnt.parent {
		prnt.stat.st_size += diff
	}
}

// UnmountFilesystem unmounts fuse, which automatically frees memory in C
func UnmountFilesystem() {
	cMount := C.CString(fi.mount)
	defer C.free(unsafe.Pointer(cMount))

	errno := C.unmount_filesystem(cMount)
	if errno > 0 {
		logs.Warningf("Unmounting filesystem failed: %s", syscall.Errno(errno).Error())
	}
}

// RefreshFilesystem clears cache and creates a new fileystem that will reflect any changes
// that have occurred in the repositories. Does not unmount fuse at any point.
func RefreshFilesystem() {
	logs.Info("Updating Data Gateway")
	C.free_nodes(fi.nodes)
	api.ClearCache()
	InitialiseFilesystem()
}

//export IsValidOpen
func IsValidOpen(pid C.int) bool {
	switch runtime.GOOS {
	case "darwin":
		for _, process := range []string{"Finder", "QuickLook"} {
			grep := exec.Command("pgrep", "-f", process)
			if res, err := grep.Output(); err == nil {
				pids := strings.Split(string(res), "\n")

				for i := range pids {
					pidInt, err := strconv.Atoi(pids[i])
					if err == nil && pidInt == int(pid) {
						logs.Debugf("%s trying to preview files", process)

						return false
					}
				}
			}
		}
	case "windows":
		filter := fmt.Sprintf("PID eq %d", pid)
		task := exec.Command("tasklist", "/FI", filter, "/fo", "table", "/nh")
		if res, err := task.Output(); err == nil {
			parts := strings.Fields(string(res))
			if parts[0] == "explorer.exe" {
				logs.Debug("Explorer trying to preview files")

				return false
			}
		}
	}

	return true
}

// FilesOpen checks if any of the files are being used by fuse
func FilesOpen() bool {
	switch runtime.GOOS {
	case "linux":
		_, err := exec.Command("fuser", "-m", fi.mount).Output()

		return err == nil
	case "darwin":
		output, err := exec.Command("fuser", "-c", fi.mount).Output()
		if err != nil {
			logs.Errorf("Update halted, could not determine if files are open: %w", err)

			return true
		}

		return len(output) > 0
	case "windows":
		volume, _ := os.Readlink(fi.mount)
		output, err := exec.Command("handle.exe", "-a", "-nobanner", volume).Output()
		if err != nil {
			logs.Errorf("Update halted, could not determine if files are open: %w", err)

			return true
		}

		return strings.Contains(string(output), volume)
	}

	return false
}

// ClearPath is designed for situations where a file is edited in the repository and the user wants to read this new data
// without updating the entire filesystem. Function clears cache for `path` and updates all its file sizes.
// Does not support adding new objects to fuse.
func ClearPath(path string) error {
	logs.Infof("Clearing path %s", path)

	node := searchNode(path)
	if node == nil {
		return fmt.Errorf("path %s is invalid", path)
	}

	pathNames := getNodePathNames(node)
	if pathNames[1] != api.SDConnect {
		return fmt.Errorf("clearing cache only enabled for SD Connect")
	}
	if len(pathNames) < 4 {
		return fmt.Errorf("path needs to include at least a bucket")
	}

	rep := pathNames[1]
	bucket := pathNames[3]
	prefix := strings.Join(pathNames[4:], "/")
	if node.children != nil && prefix != "" {
		prefix += "/"
	}
	objects, err := api.GetObjects(rep, bucket, strings.Join(pathNames[:3], "/"), prefix)
	if err != nil {
		return fmt.Errorf("cache not cleared since new file sizes could not be obtained: %w", err)
	}
	batchHeaders, err := api.GetHeaders(rep, []api.Metadata{{Name: bucket}})
	if err != nil {
		return fmt.Errorf("failed to get headers for bucket %s: %w", bucket, err)
	}

	bucketPath := strings.Join(pathNames[:4], "/")
	objMap := make(map[string]metadata, len(objects))
	for i := range objects {
		header := ""
		versions, ok := batchHeaders[bucket][objects[i].Name]
		if ok {
			header = versions.Headers[strconv.Itoa(versions.LatestVersion)].Header
		}
		objMap[bucketPath+"/"+objects[i].Name] = metadata{objects[i], header}
	}

	oldSize := node.stat.st_size
	clearNode(node, pathNames, objMap)
	updateParentSizes(node, oldSize)

	logs.Info("Path cleared")

	return nil
}

func clearNode(node *C.node_t, pathNodes []string, meta map[string]metadata) (C.off_t, C.time_t) {
	if node.children == nil {
		api.DeleteFileFromCache(pathNodes, int64(node.stat.st_size))
		path := strings.Join(pathNodes, "/")
		obj, ok := meta[path]
		if ok {
			node.offset = -1
			delete(fi.headers, node.stat.st_ino)
			if obj.header != "" {
				fi.headers[node.stat.st_ino] = obj.header
			}
			node.stat.st_size = C.off_t(obj.Size)
			node.last_modified.tv_sec = C.time_t(obj.LastModified.Unix())
		}
	} else {
		node.stat.st_size = 0
		node.last_modified.tv_sec = 0

		children := unsafe.Slice(node.children, node.chld_count)
		for i := range children {
			size, modified := clearNode(&children[i], append(pathNodes, C.GoString(children[i].orig_name)), meta)
			node.stat.st_size += size
			if modified > node.last_modified.tv_sec {
				node.last_modified.tv_sec = modified
			}
		}
	}

	return node.stat.st_size, node.last_modified.tv_sec
}

// CheckHeaderExistence tries to confirm the existence of a header for object represented by `node`.
// If the header is not found in the collection of headers retreived from vault, the header is
// still attached to the file in object storage.
// The object's size is also updated to from encrypted size to decrypted size.
//
//export CheckHeaderExistence
func CheckHeaderExistence(node *C.node_t, cpath *C.cchar_t) {
	path := C.GoString(cpath)
	logs.Debugf("Checking existence of header for object %s", path)

	header, ok := fi.headers[node.stat.st_ino]
	if !ok {
		pathNames := getNodePathNames(node)
		if pathNames[1] != api.SDConnect { // Update once SD Submit has been added
			logs.Errorf("Object %s has no header", path)

			return
		}

		var err error
		var offset int64
		object := strings.Join(pathNames[4:], "/")
		header, offset, err = api.GetReencryptedHeader(pathNames[3], object)
		if err != nil {
			logs.Warningf("Failed to retrieve possible header for %s: %w", path, err)
			logs.Infof("Testing if file %s is encrypted with unknown key", path)

			buffer := make([]byte, 0, 2*api.CipherBlockSize)
			_, err := api.DownloadData(pathNames, path, nil,
				0, 2*api.CipherBlockSize, 0, int64(node.stat.st_size))
			if err != nil {
				logs.Errorf("Could not test if file is encrypted: %v", err)

				return
			}
			if _, err = headers.ReadHeader(bytes.NewReader(buffer)); err == nil {
				fi.headers[node.stat.st_ino] = ""
				logs.Infof("File %s is encrypted", path)
			} else {
				logs.Warningf("File %s is not encrypted", path)
			}

			return
		}

		logs.Debugf("Re-encrypted header found for object %s", path)
		fi.headers[node.stat.st_ino] = header
		node.offset = C.int64_t(offset)
	} else {
		logs.Debugf("Header found for object %s", path)
	}

	if header != "" {
		bodySize := node.stat.st_size - node.offset
		if bodySize < 0 {
			logs.Errorf("File %s is too small (%d bytes) for its header size (%d bytes)", path, bodySize, len(header))

			return
		}
		oldSize := node.stat.st_size
		node.stat.st_size = calculateDecryptedSize(bodySize)
		updateParentSizes(node, oldSize)
	}
}

// calculateDecryptedSize calculates the decrypted size of an encrypted file
var calculateDecryptedSize = func(bodySize C.off_t) C.off_t {
	// Calculate number of cipher blocks in body
	blocks := C.off_t(math.Floor(float64(bodySize) / float64(api.CipherBlockSize)))
	// the last block can be smaller than 64kiB
	remainder := bodySize%C.off_t(api.CipherBlockSize) - C.off_t(api.MacSize)
	if remainder < 0 {
		remainder += C.off_t(api.MacSize)
	}

	// Add the previous info back together
	return blocks*C.off_t(api.BlockSize) + remainder
}

// DownloadData uses s3 to download data to fill `cbuffer`. It returns the amount of bytes that were
// copied to cbuffer, or, if the request failed, a negative integer, which will be interpreted in the C function
// calling DownloadData(). If no header is found for node (even an empty one), the file is not encrypted
// and cannot be read.
//
//export DownloadData
func DownloadData(node *C.node_t, cpath *C.cchar_t, cbuffer *C.char, size C.size_t, offset C.off_t) C.int {
	buffer := unsafe.Slice((*byte)(unsafe.Pointer(cbuffer)), C.int(size))
	pathNames := getNodePathNames(node)
	path := C.GoString(cpath)

	if len(pathNames) < 5 { // Needs to be modified once SD Submit is added
		logs.Errorf("Path %s is too short for an object", path)

		return -1
	}

	header, ok := fi.headers[node.stat.st_ino]
	if !ok {
		logs.Errorf("You do not have permission to access file %s: %s", path,
			http.StatusText(http.StatusUnavailableForLegalReasons))

		return -2
	}

	if offset >= node.stat.st_size {
		return 0
	}

	data, err := api.DownloadData(pathNames, path, &header,
		int64(offset), int64(offset)+int64(size), int64(node.offset), int64(node.stat.st_size))
	if err != nil {
		logs.Errorf("Retrieving data failed for %s: %w", path, err)

		return -3
	}

	return C.int(copy(buffer, data))
}
