package filesystem

/*
#include <errno.h>
#include <stdio.h>
#include <sys/stat.h>
#include "helpers.h"
#include "enabled.h"
*/
import "C"

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"
)

// Crypt4GH constants
const BlockSize int64 = 65536
const MacSize int64 = 28
const CipherBlockSize = BlockSize + MacSize

// Only used on macOS
var pidRegex = regexp.MustCompile(`^\d+[a-z]?$`)

// freeNodes calls C function free_nodes()
// This is a separate Go function so it can be used in tests
func freeNodes(root *C.node_t) {
	C.free_nodes(root)
}

// toGoStr coverts C string to Go string
// This is a separate Go function so it can be used in tests
func toGoStr(str *C.char) string {
	return C.GoString(str)
}

// toCStr coverts C string to Go string
// This is a separate Go function so it can be used in tests
func toCStr(str string) *C.char {
	return C.CString(str)
}

// freeCPtr frees memory allocated in C
// This is a separate Go function so it can be used in tests
func freeCPtr(ptr unsafe.Pointer) {
	C.free(ptr)
}

func searchNode(path string) *C.node_t {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath)) //nolint:nlreturn

	return C.search_node(fi.root.cNode, cpath) //nolint:nlreturn
}

// getNodePath returns the path to the given node in addition to
// a slice of strings containing the original names of nodes along the path
func getNodePath(node *C.node_t) (string, []string) {
	names, origNames := []string{}, []string{}
	for prnt := node; prnt != nil; prnt = prnt.parent {
		names = append(names, toGoStr((prnt.name)))
		origNames = append(origNames, toGoStr((prnt.orig_name)))
	}
	slices.Reverse(names)
	slices.Reverse(origNames)

	return filepath.Join(names...), origNames
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

// WaitForUpdateSignal updates the filesystem if it receives a SIGUSR2 signal.
// The function does not update the filesystem again if it receives a signal while
// an update is already in progress. In the GUI version, the `notify` channel is used
// to trigger the update process instead of calling UpdateFilesystem() directly.
// `notify` should be buffered so that signals are not lost while the GUI is busy.
func WaitForUpdateSignal(notify chan<- any) {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGUSR2)

	handleUpdateSignals(s, notify, UpdateFilesystem)
}

// handleUpdateSignals either sends a value to `notify`, or calls `update` if `notify` is nil, for each signal received from `s`.
// If `notify` already contains a value, the signal is ignored since the GUI has not yet started the previous update.
// The function returns once `s` is closed and a possible update in progress has finished.
// This is a separate function so it can be used in tests
func handleUpdateSignals(s <-chan os.Signal, notify chan<- any, update func()) {
	busy := make(chan any, 1)
	for range s {
		select {
		case busy <- nil:
			go func() {
				defer func() { <-busy }()

				if notify == nil {
					update()

					return
				}

				// notify is needed for GUI
				select {
				case notify <- nil:
				default:
				}
			}()
		default:
		}
	}

	busy <- nil // Wait for update in progress
}

// rwLockState is used to hold a lock that the fuse operations in C also share
func rwLockState() func() {
	runtime.LockOSThread()
	C.pthread_rwlock_wrlock(&fi.state.lock)

	return func() {
		C.pthread_rwlock_unlock(&fi.state.lock)
		runtime.UnlockOSThread()
	}
}

// UnmountFilesystem unmounts fuse, causing the memory in C to be freed
func UnmountFilesystem() error {
	unlockFun := rwLockState()
	api.DeleteWhitelistedKeys()
	unlockFun()

	return mountpoint.Unmount(fi.mount)
}

// UpdateFilesystem updates the filesystem to reflect any changes
// that have occurred the S3 storage. Does not unmount fuse at any point.
func UpdateFilesystem() {
	logs.Info("Updating Data Gateway")

	unlockFun := rwLockState()
	GenerateFilesystem()
	toDelete := maps.Values(fi.forgotten)
	unlockFun()

	// delete_node() requires that the lock is not being held here
	for node := range toDelete {
		ret := C.delete_node(&fi.state, node) //nolint:gocritic,nlreturn
		path, pathNames := getNodePath(node)
		if ret != 0 {
			logs.Warningf("kernel was not able to delete node %s: %v", path, syscall.Errno(-ret))
		}
		rep := api.Repo(pathNames[1])
		if node.children == nil && rep == api.SDConnect {
			api.DeleteFileFromCache(api.SDConnect, pathNames, int64(node.stat.st_size))
		}
	}
}

// ForgetNode is called during the forget operation to free the memory of a single node
//
//export ForgetNode
func ForgetNode(ino C.ino_t) {
	node, ok := fi.forgotten[ino]
	if !ok {
		return
	}

	delete(fi.headers, ino)
	delete(fi.forgotten, ino)
	C.free(unsafe.Pointer(node.name))
	C.free(unsafe.Pointer(node.orig_name))
	C.free(unsafe.Pointer(node.children))
	C.free(unsafe.Pointer(node))
}

// ForgetAllForgotten is called during the destroy operation to make sure
// that the memory of all nodes in fi.forgotten is freed
//
//export ForgetAllForgotten
func ForgetAllForgotten() {
	for ino := range fi.forgotten {
		ForgetNode(ino)
	}
}

// IsValidOpen is only applicable when running on macOS
//
//export IsValidOpen
func IsValidOpen(pid C.int) bool {
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

	return true
}

// FilesOpen checks if any of the files are being used by fuse
func FilesOpen() bool {
	switch runtime.GOOS {
	case "linux":
		output, err := exec.Command("fuser", "-m", fi.mount).Output()
		if err != nil {
			return false
		}

		pids := strings.Fields(string(output))
		for _, pid := range pids {
			isOpen, err := checkPID("/proc/" + pid)
			if err != nil {
				logs.Errorf("Update halted, could not determine if files are open: %w", err)

				return true
			}
			if isOpen {
				return true
			}
		}
	case "darwin":
		// plain stdout trims out the suffix in the pids
		output, err := exec.Command("fuser", "-c", fi.mount).CombinedOutput()
		if err != nil {
			logs.Errorf("Update halted, could not determine if files are open: %w", err)

			return true
		}

		outputLines := strings.Split(strings.TrimSpace(string(output)), "\n")
		pidsLine := outputLines[len(outputLines)-1] // Last non-empty line in output

		pids := slices.DeleteFunc(strings.Fields(pidsLine), func(pid string) bool {
			return pidRegex.FindString(pid) == "" || strings.HasSuffix(pid, "c")
		})

		return len(pids) > 0
	}

	return false
}

// checkPID goes through the file descriptors of a PID and checks if there are files
// being used in the filesystem. Naturally, if the fd leads to a regular file, files are being
// accessed. However, if the fd points to a directory, it may be that, e.g. nautilus,
// is going through the files. In this case, if the fdinfo has a non-zero 'pos' value, assume files
// are being read.
func checkPID(processDir string) (bool, error) {
	fdDir := processDir + "/fd"
	fdinfoDir := processDir + "/fdinfo"
	fdFS := os.DirFS(fdDir)

	entries, err := fs.ReadDir(fdFS, ".")
	if err != nil {
		return false, fmt.Errorf("failed to read fd dir %s: %w", fdDir, err)
	}

	for _, entry := range entries {
		fdName := entry.Name()

		// Read the symlink target
		fdPath := fdDir + "/" + fdName
		target, err := os.Readlink(fdPath)
		if err != nil {
			continue // skip unreadable links
		}
		if !strings.HasPrefix(target, fi.mount) {
			continue
		}
		node := searchNode(strings.TrimPrefix(target, fi.mount))
		if node == nil { // Don't know if this could happen
			continue
		}
		if node.children == nil {
			return true, nil
		}

		fdinfoPath := fdinfoDir + "/" + fdName
		pos, err := readFDPos(fdinfoPath)
		if err != nil {
			return false, fmt.Errorf("failed to read fdinfo for %s: %v", fdPath, err)
		}
		if pos > 0 {
			return true, nil
		}
	}

	return false, nil
}

// readFDPos parses /proc/<pid>/fdinfo/<fd> and returns the pos value
func readFDPos(fdinfoPath string) (int64, error) {
	file, err := os.Open(fdinfoPath)
	if os.IsNotExist(err) { // just in case fd does not exist anymore
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "pos:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			return strconv.ParseInt(fields[1], 10, 64)
		}
	}

	return 0, scanner.Err()
}

// CheckHeaderExistence tries to confirm the existence of a header for object represented by `node`.
// If the header is not found in the collection of headers retrieved from Vault, the header is
// still a part of the file in object storage.
// The object's size is also updated from encrypted size to decrypted size.
// Succesful header retrieval returns node offset, failure returns -1.
//
//export CheckHeaderExistence
func CheckHeaderExistence(node *C.node_t) int64 {
	path, pathNames := getNodePath(node)
	logs.Debugf("Checking existence of header for object %s", path)

	rep := api.Repo(pathNames[1])
	if (rep == api.SDApply && len(pathNames) < 4) ||
		(rep == api.SDConnect && len(pathNames) < 5) {
		logs.Errorf("Path %s is too short for an object", path)

		return -1
	}

	var bucket, object string
	if rep == api.SDConnect {
		bucket = pathNames[3]
		object = strings.Join(pathNames[4:], "/")
	} else {
		bucket = base64.RawURLEncoding.EncodeToString([]byte(pathNames[2]))
		object = strings.Join(pathNames[3:], "/")
	}

	hdr := fi.headers[node.stat.st_ino]
	hdrValue, err := api.GetFileHeader(rep, bucket, object, hdr.owner, hdr.fileID)
	if err != nil {
		logs.Errorf("Failed to retrieve header from Vault for object %s: %v", path, err)

		return -1
	}

	var offset int64
	if hdrValue != "" {
		hdr.value = hdrValue
		fi.headers[node.stat.st_ino] = hdr
		logs.Debugf("Header found for object %s", path)
	} else {
		if rep != api.SDConnect {
			logs.Errorf("Object %s has no header", path)

			return -1
		}

		hdrValue, offset, err = api.GetReencryptedHeader(bucket, object)
		if err != nil {
			logs.Errorf("Failed to retrieve header from Allas for object %s: %w", path, err)

			return -1
		}

		logs.Debugf("Re-encrypted header found for object %s", path)
		hdr.value = hdrValue
		fi.headers[node.stat.st_ino] = hdr
	}

	bodySize := node.stat.st_size - C.int64_t(offset)
	if bodySize < 0 {
		logs.Errorf("File %s is too small (%d bytes) for its header size (%d bytes)", path, bodySize, len(hdr.value))
		delete(fi.headers, node.stat.st_ino)

		return -1
	}

	oldSize := node.stat.st_size
	node.stat.st_size = calculateDecryptedSize(bodySize)
	updateParentSizes(node, oldSize)

	return offset
}

// calculateDecryptedSize calculates the decrypted size of an headerless encrypted file
var calculateDecryptedSize = func(bodySize C.off_t) C.off_t {
	nBlocks := C.off_t(math.Ceil(float64(bodySize) / float64(CipherBlockSize)))

	return bodySize - nBlocks*C.off_t(MacSize)
}

func updateParentSizes(node *C.node_t, oldSize C.off_t) {
	diff := node.stat.st_size - oldSize
	for prnt := node.parent; prnt != nil; prnt = prnt.parent {
		prnt.stat.st_size += diff
	}
}

// DownloadData uses S3 to download a maximum of `size` bytes to the fuse layer. It returns a C buffer
// containing the requested data. The actual length of the buffer must be conveyed to C with `nBytes`.
// If no header is found for node (even an empty one), the file is not encrypted and, therefore, cannot be read.
//
//nolint:nlreturn
//export DownloadData
func DownloadData(node *C.node_t, size C.size_t, offset C.off_t, errNo *C.int, nBytes *C.size_t) *C.char {
	path, pathNames := getNodePath(node)

	rep := api.Repo(pathNames[1])
	if (rep == api.SDApply && len(pathNames) < 4) ||
		(rep == api.SDConnect && len(pathNames) < 5) {
		logs.Errorf("Path %s is too short for an object", path)
		*errNo = C.EFAULT

		return nil
	}

	header := fi.headers[node.stat.st_ino]
	if header.value == "" {
		logs.Errorf("You do not have permission to access file %s: %s", path,
			http.StatusText(http.StatusUnavailableForLegalReasons))
		*errNo = C.EACCES

		return nil
	}

	if offset >= node.stat.st_size {
		return nil
	}

	// Better for api to not to have to know how filesystem is constructed
	if rep == api.SDConnect {
		pathNames = pathNames[3:]
	} else {
		pathNames = pathNames[2:]
	}

	data, err := api.DownloadData(rep, pathNames, path, header.owner, header.fileID, header.value,
		int64(offset), int64(offset)+int64(size), int64(node.offset), int64(node.stat.st_size))
	if err != nil {
		logs.Errorf("Retrieving data failed for %s: %w", path, err)
		*errNo = C.EIO

		return nil
	}
	*nBytes = C.size_t(len(data))

	return (*C.char)(C.CBytes(data))
}
