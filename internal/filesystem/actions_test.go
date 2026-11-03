package filesystem

import (
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"sda-filesystem/internal/api"
	"sda-filesystem/test"
)

var errExpected = errors.New("expected error for test")
var r *rand.Rand

func init() {
	r = rand.New(rand.NewSource(111)) // #nosec G404
}

func TestSearchNode(t *testing.T) {
	testRoot := getTestFuse(t)

	origRoot := fi.root
	fi.root = &goNode{cNode: testRoot}
	t.Cleanup(func() { fi.root = origRoot })

	var tests = []struct {
		testname, path string
		nodeMatch      *_Ctype_struct_Node
	}{
		{
			"OK_1", rep1.ForPath() + "/project/bucket_2/_folder/",
			findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_2", "_folder"),
		},
		{
			"OK_2", rep1.ForPath() + "/project/bucket_1///dir_////another_file",
			findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_1", "dir_", "another_file"),
		},
		{
			"OK_3", "/" + rep2.ForPath() + "/my-example.com/tiedosto",
			findChildNode(t, testRoot, rep2.ForPath(), "my-example.com", "tiedosto"),
		},
		{
			"NOT_FOUND_1", "Rep4/bucket_2/folder/file_3", nil,
		},
		{
			"NOT_FOUND_2", rep1.ForPath() + "/project/bucket_1//dir_/folder///another_folder", nil,
		},
	}

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			node := searchNode(tt.path)

			switch {
			case tt.nodeMatch == nil:
				if node != nil {
					t.Errorf("Should not have returned node %s for path %q", toGoStr(node.orig_name), tt.path)
				}
			case node == nil:
				t.Errorf("Returned nil for path %q", tt.path)
			case node != tt.nodeMatch:
				t.Errorf("Node incorrect for path %q. Expected address %p, received %p", tt.path, tt.nodeMatch, node)
			}
		})
	}
}

func TestGetNodePathNames(t *testing.T) {
	testRoot := getTestFuse(t)

	var tests = []struct {
		testname, expectedPath string
		expectedOrigPath       []string
		node                   *_Ctype_struct_Node
	}{
		{
			"OK_1",
			rep1.ForPath() + "/project/bucket_2/_folder",
			[]string{"", rep1.ForPath(), "project", "bucket_2", "?folder"},
			findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_2", "_folder"),
		},
		{
			"OK_2",
			rep1.ForPath() + "/project/bucket_1/dir_/another_file",
			[]string{"", rep1.ForPath(), "project", "bucket_1", "dir+", "another_file"},
			findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_1", "dir_", "another_file"),
		},
		{
			"OK_3",
			rep2.ForPath() + "/my-example.com/tiedosto",
			[]string{"", rep2.ForPath(), "https://my-example.com", "tiedosto"},
			findChildNode(t, testRoot, rep2.ForPath(), "my-example.com", "tiedosto"),
		},
	}

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			path, origPath := getNodePath(tt.node)

			if path != tt.expectedPath {
				t.Errorf("Path incorrect\nExpected=%s\nReceived=%s", tt.expectedPath, path)
			}
			if !reflect.DeepEqual(origPath, tt.expectedOrigPath) {
				t.Errorf("Original path incorrect\nExpected=%v\nReceived=%v", tt.expectedOrigPath, origPath)
			}
		})
	}
}

func TestGetNodeChildren(t *testing.T) {
	origRoot := fi.root
	fi.root = &goNode{cNode: getTestFuse(t)}
	t.Cleanup(func() { fi.root = origRoot })

	var tests = []struct {
		path     string
		children []string
	}{
		{rep1.ForPath() + "/project/bucket_1", []string{"dir+", "kansio"}},
		{rep1.ForPath() + "/project/bucket_2", []string{"?folder"}},
		{rep1.ForPath() + "/project/bucket_4", nil},
	}

	for i, tt := range tests {
		testname := fmt.Sprintf("OK_%d", i+1)
		t.Run(testname, func(t *testing.T) {
			ret := GetNodeChildren(tt.path)
			if !reflect.DeepEqual(ret, tt.children) {
				t.Errorf("Path %s returned incorrect children\nExpected=%v\nReceived=%v", tt.path, tt.children, ret)
			}
		})
	}
}

func TestCheckPID(t *testing.T) {
	testRoot := getTestFuse(t)
	origRoot := fi.root
	fi.root = &goNode{cNode: testRoot}
	t.Cleanup(func() { fi.root = origRoot })

	tmpMountDir := t.TempDir()
	fi.mount = tmpMountDir

	err := createFuseFiles(testRoot, tmpMountDir)
	if err != nil {
		t.Fatalf("Failed to create filesystem files: %v", err)
	}

	nonexistent := tmpMountDir + "/Substandard-Repo/non-existent-folder"
	if err := os.MkdirAll(nonexistent, 0755); err != nil {
		t.Fatalf("Failed to create folder %s: %s", nonexistent, err.Error())
	}
	outsideFilesystem := t.TempDir()

	var tests = []struct {
		fd, pos []int
		nodes   []string
		isLink  []bool
		resp    bool
	}{
		{
			// One file that should return true
			[]int{3}, []int{-1},
			[]string{rep1.ForPath() + "/project/bucket_1/kansio/file_1"},
			[]bool{true}, true,
		},
		{
			// Four folders and a non-link that should return false
			[]int{27, 85, 5, 1342, 12}, []int{0, -1, 60, 80, -1},
			[]string{
				rep1.ForPath() + "/project/shared_bucket",
				rep1.ForPath() + "/project/",
				nonexistent, outsideFilesystem, "",
			},
			[]bool{true, true, true, true, false}, false,
		},
		{
			// File, and a folder with pos=0, which should return true
			[]int{10, 15}, []int{-1, 0},
			[]string{rep1.ForPath() + "/project/bucket_3/testi", rep1.ForPath() + "/project/shared_bucket"},
			[]bool{true, true}, true,
		},
		{
			// Three folders that should return true
			[]int{28, 50, 120}, []int{200, 60, 0},
			[]string{
				rep1.ForPath() + "/project/shared_bucket",
				nonexistent, rep1.ForPath() + "/project/bucket_1",
			},
			[]bool{true, true, true}, true,
		},
	}

	for i, tt := range tests {
		testname := fmt.Sprintf("OK_%d", i+1)
		t.Run(testname, func(t *testing.T) {
			tmpFdDir := t.TempDir()
			if err := os.MkdirAll(tmpFdDir+"/fd", 0755); err != nil {
				t.Fatalf("Failed to create fd folder %s: %s", tmpFdDir+"/fd", err.Error())
			}
			if err := os.MkdirAll(tmpFdDir+"/fdinfo", 0755); err != nil {
				t.Fatalf("Failed to create fdinfo folder %s: %s", tmpFdDir+"/fdinfo", err.Error())
			}

			for i := range tt.nodes {
				nodePath := tmpMountDir + "/" + tt.nodes[i]
				if tt.nodes[i] == outsideFilesystem {
					nodePath = outsideFilesystem
				}
				fdPath := tmpFdDir + "/fd/" + strconv.Itoa(tt.fd[i])
				if tt.isLink[i] {
					err = os.Symlink(nodePath, fdPath)
					if err != nil {
						t.Fatalf("Failed to create symlink: %s", err.Error())
					}
				} else if err := os.WriteFile(fdPath, []byte("I am not a symlink"), 0600); err != nil {
					t.Fatalf("Failed to create file %s: %s", fdPath, err.Error())
				}

				// Generate fdinfo file where one of the lines is `pos: <int>`
				if tt.pos[i] >= 0 {
					lines := []string{}
					for range 10 {
						lines = append(lines, string(test.GenerateRandomText(100)))
					}
					lines = slices.Insert(lines, r.Intn(len(lines)), fmt.Sprintf("pos: %d\n", tt.pos[i]))

					fdinfoFile := tmpFdDir + "/fdinfo/" + strconv.Itoa(tt.fd[i])
					if err := os.WriteFile(fdinfoFile, []byte(strings.Join(lines, "\n")), 0600); err != nil {
						t.Fatalf("Failed to create fdinfo file %s: %s", fdinfoFile, err.Error())
					}
				}
			}

			resp, err := checkPID(tmpFdDir)
			if err != nil {
				t.Fatalf("Function returned unexpected error: %s", err.Error())
			}
			if resp != tt.resp {
				if tt.resp {
					t.Fatal("Files should be open")
				} else {
					t.Fatal("Files should not be open")
				}
			}
		})
	}
}

func TestCheckPID_FDExistsError(t *testing.T) {
	tmpFdDir := t.TempDir()
	os.RemoveAll(tmpFdDir) // make sure folder does not exist

	errStr := "failed to read fd dir " + tmpFdDir + "/fd: open .: no such file or directory"
	_, err := checkPID(tmpFdDir)
	if err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestCheckPID_PIDError(t *testing.T) {
	testRoot := getTestFuse(t)
	origRoot := fi.root
	fi.root = &goNode{cNode: testRoot}
	t.Cleanup(func() { fi.root = origRoot })

	tmpMountDir := t.TempDir()
	fi.mount = tmpMountDir

	err := createFuseFiles(testRoot, tmpMountDir)
	if err != nil {
		t.Fatalf("Failed to create filesystem files: %v", err)
	}

	tmpFdDir := t.TempDir()
	if err := os.MkdirAll(tmpFdDir+"/fd", 0755); err != nil {
		t.Fatalf("Failed to create fd folder %s: %s", tmpFdDir+"/fd", err.Error())
	}
	if err := os.MkdirAll(tmpFdDir+"/fdinfo", 0755); err != nil {
		t.Fatalf("Failed to create fdinfo folder %s: %s", tmpFdDir+"/fdinfo", err.Error())
	}

	// Test case 1: pos is not an integer
	nodePath := tmpMountDir + "/" + rep1.ForPath() + "/project"
	if err = os.Symlink(nodePath, tmpFdDir+"/fd/42"); err != nil {
		t.Fatalf("Failed to create symlink: %s", err.Error())
	}

	lines := []string{}
	for range 10 {
		lines = append(lines, string(test.GenerateRandomText(100)))
	}
	lines = slices.Insert(lines, r.Intn(len(lines)), "pos: bob\n")

	fdinfoFile := tmpFdDir + "/fdinfo/42"
	if err := os.WriteFile(fdinfoFile, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		t.Fatalf("Failed to create fdinfo file %s: %s", fdinfoFile, err.Error())
	}

	errStr := "failed to read fdinfo for " + tmpFdDir + "/fd/42: strconv.ParseInt: parsing \"bob\": invalid syntax"
	_, err = checkPID(tmpFdDir)
	if err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}

	// Test case 2: line length too long in fdinfo
	lines = []string{string(test.GenerateRandomText(100000))}
	if err := os.WriteFile(fdinfoFile, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		t.Fatalf("Failed to create fdinfo file %s: %s", fdinfoFile, err.Error())
	}

	errStr = "failed to read fdinfo for " + tmpFdDir + "/fd/42: bufio.Scanner: token too long"
	_, err = checkPID(tmpFdDir)
	if err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestCheckHeaderExistence_Found(t *testing.T) {
	testRoot := getTestFuse(t)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origFileHeader := api.GetFileHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.GetFileHeader = origFileHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		t.Errorf("api.GetReencryptedHeader() should not be called")

		return "", 0, nil
	}
	api.GetFileHeader = func(rep api.Repo, bucket, object, owner, id string) (string, error) {
		if bucket != "bucket_2" {
			t.Errorf("api.GetFileHeader() received incorrect bucket. Expected=bucket_2, received=%s", bucket)
		}
		if object != "?folder/test" {
			t.Errorf("api.GetFileHeader() received incorrect object. Expected=?folder/test, received=%s", object)
		}
		if id != "45646746" {
			t.Errorf("api.GetFileHeader() received incorrect id. Expected=45646746, received=%s", id)
		}

		return "hello", nil
	}

	node := findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_2", "_folder", "test")
	node.stat.st_size = 484
	fi.headers = map[_Ctype_ino_t]header{node.stat.st_ino: {fileID: "45646746"}}

	if offset := CheckHeaderExistence(node); offset != 0 {
		t.Errorf("Function returned incorrect offset. Expected=0, received=%d", offset)
	}
	if !reflect.DeepEqual(fi.headers, map[_Ctype_ino_t]header{node.stat.st_ino: {fileID: "45646746", value: "hello"}}) {
		t.Errorf("Headers were modified to %v", fi.headers)
	}

	// The file's encrypted size (484) decrypts to 456 (one block, minus the 28-byte MAC);
	// the resulting 28-byte difference should propagate up through every ancestor's size.
	origFs := getTestFuse(t)
	origNode := findChildNode(t, origFs, rep1.ForPath(), "project", "bucket_2", "_folder", "test")
	origNode.stat.st_size = 456
	diff := origNode.stat.st_size - 484
	for p := origNode.parent; p != nil; p = p.parent {
		p.stat.st_size += diff
	}

	if err := isValidFuse(origFs, testRoot, nil, ""); err != nil {
		t.Errorf("Filesystem affected incorrectly: %s", err.Error())
	}
}

func TestCheckHeaderExistence_ErrorNoChange(t *testing.T) {
	var tests = []struct {
		testname                                string
		path                                    []string
		allowReencryptedHeader, allowFileHeader bool
		reencryptedErr                          error
	}{
		{
			// Bucket-level path is too short for a valid SD Apply object
			"FAIL_BAD_PATH_APPLY", []string{rep2.ForPath(), "bad-bucket"}, false, false, nil,
		},
		{
			// Bucket-level path is too short for a valid SD Connect object
			"FAIL_BAD_PATH_CONNECT", []string{rep1.ForPath(), "project", "bucket_1"}, false, false, nil,
		},
		{
			// SD Apply never falls back to a re-encrypted header
			"FAIL_SDAPPLY_NOT_ENCRYPTED", []string{rep2.ForPath(), "my-example.com", "tiedosto"}, false, true, nil,
		},
		{
			// SD Connect falls back to a re-encrypted header, which also fails here
			"FAIL_SDCONNECT_NOT_ENCRYPTED", []string{rep1.ForPath(), "project", "bucket_3", "testi"}, true, true, errExpected,
		},
	}

	origGetReencryptedHeader := api.GetReencryptedHeader
	origGetFileHeader := api.GetFileHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.GetFileHeader = origGetFileHeader
		fi.headers = origHeaders
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
				if !tt.allowReencryptedHeader {
					t.Errorf("api.GetReencryptedHeader() should not be called")
				}

				return "", 0, tt.reencryptedErr
			}
			api.GetFileHeader = func(rep api.Repo, bucket, object, owner, id string) (string, error) {
				if !tt.allowFileHeader {
					t.Errorf("api.GetFileHeader() should not be called")
				}

				return "", nil
			}

			testRoot := getTestFuse(t)
			node := findChildNode(t, testRoot, tt.path...)
			fi.headers = make(map[_Ctype_ino_t]header)

			if offset := CheckHeaderExistence(node); offset != -1 {
				t.Errorf("Function returned incorrect offset. Expected=-1, received=%d", offset)
			}
			if len(fi.headers) > 0 {
				t.Errorf("Headers were modified to %v", fi.headers)
			}

			origFs := getTestFuse(t)
			if err := isValidFuse(origFs, testRoot, nil, ""); err != nil {
				t.Errorf("Filesystem affected: %s", err.Error())
			}
		})
	}
}

func TestCheckHeaderExistence_Reencrypted(t *testing.T) {
	testRoot := getTestFuse(t)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origGetFileHeader := api.GetFileHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.GetFileHeader = origGetFileHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		if bucket != "bucket_1" {
			t.Errorf("api.GetReencryptedHeader() received incorrect bucket. Expected=bucket_1, received=%s", bucket)
		}
		if object != "dir+/another_file" {
			t.Errorf("api.GetReencryptedHeader() received incorrect object. Expected=dir+/another_file, received=%s", object)
		}

		return "i-am-a-header", 58, nil
	}
	api.GetFileHeader = func(rep api.Repo, bucket, object, owner, id string) (string, error) {
		return "", nil
	}

	node := findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_1", "dir_", "another_file")
	fi.headers = make(map[_Ctype_ino_t]header)

	if offset := CheckHeaderExistence(node); offset != 58 {
		t.Errorf("Function returned incorrect offset. Expected=58, received=%d", offset)
	}
	expectedHeaders := map[_Ctype_ino_t]header{node.stat.st_ino: {value: "i-am-a-header"}}
	if !reflect.DeepEqual(fi.headers, expectedHeaders) {
		t.Errorf("Headers are incorrect\nExpected=%v\nReceived=%v", expectedHeaders, fi.headers)
	}

	// Body size is 112-58=54 bytes, one block, decrypts to 54-28=26 bytes;
	// the resulting 86-byte difference should propagate up through every ancestor's size.
	origFs := getTestFuse(t)
	origNode := findChildNode(t, origFs, rep1.ForPath(), "project", "bucket_1", "dir_", "another_file")
	oldSize := origNode.stat.st_size
	origNode.stat.st_size = 26
	diff := origNode.stat.st_size - oldSize
	for p := origNode.parent; p != nil; p = p.parent {
		p.stat.st_size += diff
	}

	if err := isValidFuse(origFs, testRoot, nil, ""); err != nil {
		t.Errorf("Filesystem affected incorrectly: %s", err.Error())
	}
}

func TestCheckHeaderExistence_FileHeaderError(t *testing.T) {
	testRoot := getTestFuse(t)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origGetFileHeader := api.GetFileHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.GetFileHeader = origGetFileHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		t.Errorf("api.GetReencryptedHeader() should not be called")

		return "", 0, nil
	}
	api.GetFileHeader = func(rep api.Repo, bucket, object, owner, id string) (string, error) {
		return "", errExpected
	}

	node := findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_3", "testi")
	fi.headers = map[_Ctype_ino_t]header{node.stat.st_ino: {owner: "my-project"}}

	if offset := CheckHeaderExistence(node); offset != -1 {
		t.Errorf("Function returned incorrect offset. Expected=-1, received=%d", offset)
	}
	expectedHeaders := map[_Ctype_ino_t]header{node.stat.st_ino: {owner: "my-project"}}
	if !reflect.DeepEqual(fi.headers, expectedHeaders) {
		t.Errorf("Headers should be unchanged\nExpected=%v\nReceived=%v", expectedHeaders, fi.headers)
	}

	origFs := getTestFuse(t)
	if err := isValidFuse(origFs, testRoot, nil, ""); err != nil {
		t.Errorf("Filesystem affected: %s", err.Error())
	}
}

func TestCheckHeaderExistence_TooSmall(t *testing.T) {
	testRoot := getTestFuse(t)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origGetFileHeader := api.GetFileHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.GetFileHeader = origGetFileHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		if bucket != "bucket_1" {
			t.Errorf("api.GetReencryptedHeader() received incorrect bucket. Expected=bucket_1, received=%s", bucket)
		}
		if object != "dir+/another_file" {
			t.Errorf("api.GetReencryptedHeader() received incorrect object. Expected=dir+/another_file, received=%s", object)
		}

		// Offset larger than the file's own (encrypted) size
		return "i-am-a-header", 124, nil
	}
	api.GetFileHeader = func(rep api.Repo, bucket, object, owner, id string) (string, error) {
		return "", nil
	}

	node := findChildNode(t, testRoot, rep1.ForPath(), "project", "bucket_1", "dir_", "another_file")
	fi.headers = make(map[_Ctype_ino_t]header)

	if offset := CheckHeaderExistence(node); offset != -1 {
		t.Errorf("Function returned incorrect offset. Expected=-1, received=%d", offset)
	}
	if len(fi.headers) > 0 {
		t.Errorf("Unexpected values in headers: %v", fi.headers)
	}

	origFs := getTestFuse(t)
	if err := isValidFuse(origFs, testRoot, nil, ""); err != nil {
		t.Errorf("Filesystem affected: %s", err.Error())
	}
}

// Beginning of helper functions for TestUpdateFilesystem()

// collectSubtree collects the paths of `node` and all its descendants, as well as the expected
// calls to api.DeleteFileFromCache() for the SD Connect files among them
func collectSubtree(node *_Ctype_struct_Node, paths, cacheEntries *[]string) {
	path, origNames := getNodePath(node)
	*paths = append(*paths, path)

	if node.children == nil {
		if api.Repo(origNames[1]) == rep1 {
			*cacheEntries = append(*cacheEntries, cacheEntry(rep1, strings.Join(origNames, "/"), int64(node.stat.st_size)))
		}

		return
	}
	for _, child := range unsafe.Slice(node.children, node.chld_count) {
		collectSubtree(child, paths, cacheEntries)
	}
}

// removeTestNode removes `node` from its parent in a test filesystem and updates ancestor sizes and timestamps
func removeTestNode(node *_Ctype_struct_Node) {
	prnt := node.parent
	setTestNodeSize(node, 0)

	children := unsafe.Slice(prnt.children, prnt.chld_count)
	idx := slices.Index(children, node)
	copy(children[idx:], children[idx+1:])
	prnt.chld_count--

	setTestNodeModified(prnt, nil)

	freeNodes(node)
	freeCPtr(unsafe.Pointer(node))
}

// setTestNodeSize changes the size of a file in a test filesystem and updates ancestor sizes
func setTestNodeSize(node *_Ctype_struct_Node, size int64) {
	diff := _Ctype_off_t(size) - node.stat.st_size
	for p := node; p != nil; p = p.parent {
		p.stat.st_size += diff
	}
}

// setTestNodeModified changes the timestamp of a node in a test filesystem if `modified` is not nil.
// The timestamps of directories, starting from `node`, are set to the newest timestamp among their children,
// or 0 if they have no children.
func setTestNodeModified(node *_Ctype_struct_Node, modified *time.Time) {
	if modified != nil {
		node.last_modified.tv_sec = _Ctype_time_t(modified.Unix())
	}

	for p := node; p != nil; p = p.parent {
		if p.children == nil {
			continue // file
		}
		p.last_modified.tv_sec = 0
		for _, child := range unsafe.Slice(p.children, p.chld_count) {
			p.last_modified.tv_sec = max(p.last_modified.tv_sec, child.last_modified.tv_sec)
		}
	}
}

// addTestFile adds a new file under `prnt` in a test filesystem and updates ancestor sizes and timestamps
func addTestFile(prnt *_Ctype_struct_Node, name string, size int64, modified *time.Time) {
	node := allocateNodePointer()
	node.name = toCStr(name)
	node.orig_name = toCStr(name)
	node.stat.st_mode = syscall.S_IFREG | 0644
	node.stat.st_ino = _Ctype_ino_t(uintptr(unsafe.Pointer(node)))
	node.offset = -1
	node.parent = prnt

	children := allocateNodeList(prnt.chld_count + 1)
	newSlice := unsafe.Slice(children, prnt.chld_count+1)
	copy(newSlice, unsafe.Slice(prnt.children, prnt.chld_count))
	newSlice[prnt.chld_count] = node
	slices.SortFunc(newSlice, func(a, b *_Ctype_struct_Node) int {
		return strings.Compare(toGoStr(a.name), toGoStr(b.name))
	})
	freeCPtr(unsafe.Pointer(prnt.children))
	prnt.children = children
	prnt.chld_count++

	setTestNodeSize(node, size)
	setTestNodeModified(node, modified)
}

// getForgottenPaths returns the paths of the nodes in fi.forgotten and checks that their offset is -2
func getForgottenPaths(t *testing.T) []string {
	t.Helper()

	paths := []string{}
	for ino, node := range fi.forgotten {
		path, _ := getNodePath(node)
		if uintptr(ino) != uintptr(unsafe.Pointer(node)) {
			t.Errorf("Forgotten node %s stored with incorrect ino %d", path, ino)
		}
		if node.offset != -2 {
			t.Errorf("Forgotten node %s should have offset -2, received %d", path, node.offset)
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)

	return paths
}

// assertLockReleased makes sure the lock shared with the fuse operations is not held
func assertLockReleased(t *testing.T) {
	t.Helper()

	acquired := make(chan any)
	go func() {
		unlockFun := rwLockState()
		unlockFun()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("Lock was not released")
	}
}

// setupUpdateFilesystem generates the filesystem and returns the mocked storage containing the next version of the fileystem
func setupUpdateFilesystem(t *testing.T) *testStorage {
	t.Helper()

	storage := getTestStorage(t)
	mockTestStorage(t, storage)
	generateTestFilesystem(t)

	if err := isValidFuse(getTestFuse(t), fi.root.cNode, nil, ""); err != nil {
		t.Fatalf("Initial filesystem was not created correctly: %s", err.Error())
	}
	if len(storage.deletedFromCache) > 0 {
		t.Fatalf("api.DeleteFileFromCache() should not be called when filesystem is first created, received %v", storage.deletedFromCache)
	}
	if len(fi.forgotten) > 0 {
		t.Fatalf("No nodes should have been forgotten when filesystem is first created, received %d nodes", len(fi.forgotten))
	}

	return storage
}

func TestUpdateFilesystem(t *testing.T) {
	storage := setupUpdateFilesystem(t)

	// Pretend CheckHeaderExistence() has been called for these files.
	file1 := findChildNode(t, fi.root.cNode, rep1.ForPath(), "project", "bucket_1", "kansio", "file_1")
	file1.stat.st_size = 18
	file1.offset = 5
	testi := findChildNode(t, fi.root.cNode, rep1.ForPath(), "project", "bucket_3", "testi")
	testi.stat.st_size = 120
	testi.offset = 0

	// Nodes that persist through the update should keep their memory address (and therefore ino)
	persistent := map[string]*_Ctype_struct_Node{}
	for _, path := range []string{
		rep1.ForPath() + "/project/bucket_1/kansio/file_1",
		rep1.ForPath() + "/project/bucket_1/kansio",
		rep1.ForPath() + "/project/bucket_3/testi",
		rep2.ForPath() + "/old-bucket/dir_2/logs",
		rep1.ForPath() + "/project/shared_bucket(58dca8)/shared-file-2.txt",
	} {
		persistent[path] = findChildNode(t, fi.root.cNode, strings.Split(path, "/")...)
	}

	// Modify storage. The new timestamp is later than any other in the filesystem,
	// so the timestamp of the root changes as well.
	newTime := parseTime(t, "2026-01-01T00:00:00Z")
	storage.objects[rep1]["bucket_1"] = slices.DeleteFunc(storage.objects[rep1]["bucket_1"], func(meta api.Metadata) bool {
		return meta.Name == "kansio/file_2"
	})
	storage.objects[rep1]["bucket_3"] = []api.Metadata{
		{Size: 200, Name: "testi", LastModified: newTime},
		{Size: 5, Name: "new_file", LastModified: newTime},
	}
	// Renamed bucket shared#bucket should keep its name even though its twin shared_bucket is removed
	storage.buckets[rep1] = slices.DeleteFunc(storage.buckets[rep1], func(meta api.Metadata) bool {
		return strings.HasPrefix(meta.Name, "bucket_2") || meta.Name == "shared_bucket"
	})
	storage.objects[rep2]["old-bucket"] = slices.DeleteFunc(storage.objects[rep2]["old-bucket"], func(meta api.Metadata) bool {
		return strings.HasPrefix(meta.Name, "dir4/")
	})
	storage.buckets[rep2] = slices.DeleteFunc(storage.buckets[rep2], func(meta api.Metadata) bool {
		return meta.Name == "https://my-example.com"
	})

	// Make the same changes to the expected filesystem.
	// my-example.com is the newest SD Apply dataset, so removing it moves the SD-Apply timestamp backwards.
	expectedFs := getTestFuse(t)
	expectedForgotten, expectedCache := []string{}, []string{}
	for _, path := range [][]string{
		{rep1.ForPath(), "project", "bucket_1", "kansio", "file_2"},
		{rep1.ForPath(), "project", "bucket_2"},
		{rep1.ForPath(), "project", "shared_bucket"},
		{rep2.ForPath(), "old-bucket", "dir4"},
		{rep2.ForPath(), "my-example.com"},
	} {
		node := findChildNode(t, expectedFs, path...)
		collectSubtree(node, &expectedForgotten, &expectedCache)
		removeTestNode(node)
	}

	expectedFile1 := findChildNode(t, expectedFs, rep1.ForPath(), "project", "bucket_1", "kansio", "file_1")
	setTestNodeSize(expectedFile1, 18)
	expectedFile1.offset = 5

	expectedTesti := findChildNode(t, expectedFs, rep1.ForPath(), "project", "bucket_3", "testi")
	setTestNodeSize(expectedTesti, 200)
	setTestNodeModified(expectedTesti, newTime)
	expectedCache = append(expectedCache, cacheEntry(rep1, "/"+rep1.ForPath()+"/project/bucket_3/testi", 200))

	addTestFile(findChildNode(t, expectedFs, rep1.ForPath(), "project", "bucket_3"), "new_file", 5, newTime)

	UpdateFilesystem()
	assertLockReleased(t)

	if err := isValidFuse(expectedFs, fi.root.cNode, nil, ""); err != nil {
		t.Errorf("Filesystem was not updated correctly: %s", err.Error())
	}
	for path, node := range persistent {
		if newNode := searchNode(path); newNode != node {
			t.Errorf("Node %s should have kept its address %p, received %p", path, node, newNode)
		}
	}

	slices.Sort(expectedForgotten)
	if forgotten := getForgottenPaths(t); !reflect.DeepEqual(forgotten, expectedForgotten) {
		t.Errorf("Incorrect forgotten nodes\nExpected=%v\nReceived=%v", expectedForgotten, forgotten)
	}

	// Removed SD Connect files are deleted from cache, as well as the modified file.
	// SD Apply files and directories should not be touched.
	slices.Sort(expectedCache)
	slices.Sort(storage.deletedFromCache)
	if !slices.Equal(storage.deletedFromCache, expectedCache) {
		t.Errorf("api.DeleteFileFromCache() called incorrectly\nExpected=%v\nReceived=%v", expectedCache, storage.deletedFromCache)
	}
}

func TestUpdateFilesystem_NoChanges(t *testing.T) {
	storage := setupUpdateFilesystem(t)

	// Every node should keep its memory address (and therefore ino)
	paths := []string{}
	collectSubtree(fi.root.cNode, &paths, &[]string{})
	nodes := map[string]*_Ctype_struct_Node{}
	for _, path := range paths {
		nodes[path] = searchNode(path)
	}
	expectedHeaders := maps.Clone(fi.headers)

	UpdateFilesystem()
	assertLockReleased(t)

	if err := isValidFuse(getTestFuse(t), fi.root.cNode, nil, ""); err != nil {
		t.Errorf("Filesystem should not have changed: %s", err.Error())
	}
	for _, path := range paths {
		if node := searchNode(path); node != nodes[path] {
			t.Errorf("Node %s should have kept its address %p, received %p", path, nodes[path], node)
		}
	}
	if !reflect.DeepEqual(fi.headers, expectedHeaders) {
		t.Errorf("Headers should not have changed\nExpected=%v\nReceived=%v", expectedHeaders, fi.headers)
	}
	if forgotten := getForgottenPaths(t); len(forgotten) > 0 {
		t.Errorf("No nodes should have been forgotten, received %v", forgotten)
	}
	if len(storage.deletedFromCache) > 0 {
		t.Errorf("api.DeleteFileFromCache() should not have been called, received %v", storage.deletedFromCache)
	}
}

// statusError mocks an S3 error that has an HTTP status code
type statusError int

func (se statusError) Error() string {
	return fmt.Sprintf("status code %d", int(se))
}

func (se statusError) HTTPStatusCode() int {
	return int(se)
}

func TestUpdateFilesystem_APIError(t *testing.T) {
	var tests = []struct {
		testname   string
		bucketsErr map[api.Repo]error
		objectsErr map[string]error
		emptied    [][]string // directories that lose all their children in the update
	}{
		{
			// Temporary errors keep the contents from the previous update
			"OK_BUCKETS_SD_CONNECT", map[api.Repo]error{rep1: errExpected}, nil, nil,
		},
		{
			// Expired token is not a reason to remove contents either
			"OK_BUCKETS_SD_APPLY_UNAUTHORIZED", map[api.Repo]error{rep2: statusError(http.StatusUnauthorized)}, nil, nil,
		},
		{
			"OK_OBJECTS", nil,
			map[string]error{"bucket_2": errExpected, "old-bucket": statusError(http.StatusInternalServerError)}, nil,
		},
		{
			// Revoked access means the repository appears empty after the update
			"FAIL_BUCKETS_SD_CONNECT_FORBIDDEN", map[api.Repo]error{rep1: statusError(http.StatusForbidden)}, nil,
			[][]string{{rep1.ForPath()}},
		},
		{
			// SD Apply files are never deleted from cache
			"FAIL_BUCKETS_SD_APPLY_NOT_FOUND", map[api.Repo]error{rep2: statusError(http.StatusNotFound)}, nil,
			[][]string{{rep2.ForPath()}},
		},
		{
			// Revoked access or removed bucket means the bucket appears empty after the update
			"FAIL_OBJECTS", nil,
			map[string]error{"bucket_2": statusError(http.StatusForbidden), "old-bucket": statusError(http.StatusNotFound)},
			[][]string{{rep1.ForPath(), "project", "bucket_2"}, {rep2.ForPath(), "old-bucket"}},
		},
		{
			// Only the bucket with revoked access is emptied
			"FAIL_OBJECTS_MIXED", nil,
			map[string]error{"bucket_2": statusError(http.StatusForbidden), "old-bucket": errExpected},
			[][]string{{rep1.ForPath(), "project", "bucket_2"}},
		},
		{
			// Repository without any buckets appears empty after the update
			"FAIL_BUCKETS_NO_BUCKETS", map[api.Repo]error{rep2: fmt.Errorf("%w for %s", api.ErrNoBuckets, rep2)}, nil,
			[][]string{{rep2.ForPath()}},
		},
		{
			"FAIL_OBJECTS_INVALID_BUCKET_NAME", nil,
			map[string]error{"bucket_2": fmt.Errorf("failed to list objects for bucket_2: %w", api.ErrInvalidBucketName)},
			[][]string{{rep1.ForPath(), "project", "bucket_2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			storage := setupUpdateFilesystem(t)
			if tt.bucketsErr != nil {
				storage.bucketsErr = tt.bucketsErr
			}
			if tt.objectsErr != nil {
				storage.objectsErr = tt.objectsErr
			}

			expectedFs := getTestFuse(t)
			expectedForgotten, expectedCache := []string{}, []string{}
			for _, path := range tt.emptied {
				node := findChildNode(t, expectedFs, path...)
				for _, child := range slices.Clone(unsafe.Slice(node.children, node.chld_count)) {
					collectSubtree(child, &expectedForgotten, &expectedCache)
					removeTestNode(child)
				}
			}

			UpdateFilesystem()
			assertLockReleased(t)

			if err := isValidFuse(expectedFs, fi.root.cNode, nil, ""); err != nil {
				t.Errorf("Filesystem was not updated correctly: %s", err.Error())
			}

			slices.Sort(expectedForgotten)
			if forgotten := getForgottenPaths(t); !reflect.DeepEqual(forgotten, expectedForgotten) {
				t.Errorf("Incorrect forgotten nodes\nExpected=%v\nReceived=%v", expectedForgotten, forgotten)
			}
			slices.Sort(expectedCache)
			slices.Sort(storage.deletedFromCache)
			if !slices.Equal(storage.deletedFromCache, expectedCache) {
				t.Errorf("api.DeleteFileFromCache() called incorrectly\nExpected=%v\nReceived=%v", expectedCache, storage.deletedFromCache)
			}
		})
	}
}

func TestUpdateFilesystem_Panic(t *testing.T) {
	storage := setupUpdateFilesystem(t)

	called := false
	SetSignalBridge(func() { called = true })
	t.Cleanup(func() { SetSignalBridge(nil) })

	storage.panicMsg = "Muahahahaa"
	UpdateFilesystem()
	assertLockReleased(t)

	if !called {
		t.Errorf("signalBridge() not called even though update paniced")
	}
	if err := isValidFuse(getTestFuse(t), fi.root.cNode, nil, ""); err != nil {
		t.Errorf("Filesystem should not have changed: %s", err.Error())
	}
	if forgotten := getForgottenPaths(t); len(forgotten) > 0 {
		t.Errorf("No nodes should have been forgotten, received %v", forgotten)
	}
	if len(storage.deletedFromCache) > 0 {
		t.Errorf("api.DeleteFileFromCache() should not have been called, received %v", storage.deletedFromCache)
	}
}

func TestDownloadData(t *testing.T) {
	var tests = []struct {
		testname, path string
		hdr            header
		expectedNodes  []string
		data           []byte
	}{
		{
			"OK_SD_CONNECT", rep1.ForPath() + "/project/bucket_1/dir_/another_file",
			header{value: "header-1"},
			[]string{"bucket_1", "dir+", "another_file"},
			[]byte("ten bytes!"),
		},
		{
			"OK_SD_CONNECT_SHARED", rep1.ForPath() + "/project/shared_bucket/shared-file.txt",
			header{value: "header-2", owner: "sharing-project"},
			[]string{"shared_bucket", "shared-file.txt"},
			[]byte("forty-two bytes of data from shared bucket"),
		},
		{
			"OK_SD_APPLY", rep2.ForPath() + "/my-example.com/tiedosto",
			header{value: "header-3", owner: "muumi", fileID: "file-id"},
			[]string{"https://my-example.com", "tiedosto"},
			[]byte("xyz"),
		},
		{
			"OK_PAST_END", rep2.ForPath() + "/my-example.com/tiedosto",
			header{value: "header-4"},
			[]string{"https://my-example.com", "tiedosto"},
			[]byte("abcd"),
		},
		{
			"OK_EMPTY_RESPONSE", rep1.ForPath() + "/project/bucket_3/testi",
			header{value: "header-5"},
			[]string{"bucket_3", "testi"},
			[]byte{},
		},
	}

	origDownloadData := api.DownloadData
	origHeaders := fi.headers
	defer func() {
		api.DownloadData = origDownloadData
		fi.headers = origHeaders
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			testRoot := getTestFuse(t)
			node := findChildNode(t, testRoot, strings.Split(tt.path, "/")...)
			node.offset = 12
			fi.headers = map[_Ctype_ino_t]header{node.stat.st_ino: tt.hdr}

			called := false
			api.DownloadData = func(rep api.Repo, nodes []string, path, owner, fileID, hdr string,
				startDecrypted, endDecrypted, oldOffset, fileSize int64,
			) ([]byte, error) {
				called = true
				if expectedRep := api.Repo(strings.Split(tt.path, "/")[0]); rep != expectedRep {
					t.Errorf("api.DownloadData() received incorrect repository. Expected=%s, received=%s", expectedRep, rep)
				}
				if !reflect.DeepEqual(nodes, tt.expectedNodes) {
					t.Errorf("api.DownloadData() received incorrect nodes\nExpected=%v\nReceived=%v", tt.expectedNodes, nodes)
				}
				if path != tt.path {
					t.Errorf("api.DownloadData() received incorrect path. Expected=%s, received=%s", tt.path, path)
				}
				if owner != tt.hdr.owner {
					t.Errorf("api.DownloadData() received incorrect owner. Expected=%s, received=%s", tt.hdr.owner, owner)
				}
				if fileID != tt.hdr.fileID {
					t.Errorf("api.DownloadData() received incorrect file ID. Expected=%s, received=%s", tt.hdr.fileID, fileID)
				}
				if hdr != tt.hdr.value {
					t.Errorf("api.DownloadData() received incorrect header. Expected=%s, received=%s", tt.hdr.value, hdr)
				}
				if startDecrypted != 4 || endDecrypted != 146 {
					t.Errorf("api.DownloadData() received incorrect range. Expected=[6, 148), received=[%d, %d)", startDecrypted, endDecrypted)
				}
				if oldOffset != 12 {
					t.Errorf("api.DownloadData() received incorrect node offset. Expected=12, received=%d", oldOffset)
				}
				if fileSize != int64(node.stat.st_size) {
					t.Errorf("api.DownloadData() received incorrect file size. Expected=%d, received=%d", int64(node.stat.st_size), fileSize)
				}

				return tt.data, nil
			}

			var errNo _Ctype_int
			var nBytes _Ctype_size_t
			buf := DownloadData(node, 142, 4, &errNo, &nBytes)
			if buf != nil {
				defer freeCPtr(unsafe.Pointer(buf))
			}

			if !called {
				t.Errorf("api.DownloadData() was not called")
			}
			if errNo != 0 {
				t.Errorf("Function set errNo to %d", errNo)
			}
			if int(nBytes) != len(tt.data) {
				t.Fatalf("Function returned incorrect number of bytes. Expected=%d, received=%d", len(tt.data), nBytes)
			}
			if nBytes > 0 && buf == nil {
				t.Fatal("Function returned nil buffer")
			}
			if nBytes > 0 {
				data := unsafe.Slice((*byte)(unsafe.Pointer(buf)), nBytes)
				if !slices.Equal(data, tt.data) {
					t.Errorf("Function returned incorrect data\nExpected=%s\nReceived=%s", string(tt.data), string(data))
				}
			}
		})
	}
}

func TestDownloadData_Error(t *testing.T) {
	var tests = []struct {
		testname, path string
		hdr            *header
		tryDownload    bool
		downloadErr    error
		expectedErrNo  int
	}{
		{
			// Bucket-level path is too short for a valid SD Apply object
			"FAIL_BAD_PATH_APPLY", rep2.ForPath() + "/bad-bucket",
			&header{value: "header"}, false, nil,
			int(syscall.EFAULT),
		},
		{
			// Bucket-level path is too short for a valid SD Connect object
			"FAIL_BAD_PATH_CONNECT", rep1.ForPath() + "/project/bucket_1",
			&header{value: "header"}, false, nil,
			int(syscall.EFAULT),
		},
		{
			"FAIL_NO_HEADER", rep1.ForPath() + "/project/bucket_3/testi",
			nil, false, nil,
			int(syscall.EACCES),
		},
		{
			// Header was not found from Vault nor from the object itself
			"FAIL_EMPTY_HEADER", rep2.ForPath() + "/my-example.com/tiedosto",
			&header{owner: "muumi", fileID: "file-id"}, false, nil,
			int(syscall.EACCES),
		},
		{
			"FAIL_UNKNOWN_REPOSITORY", "Bad-Repo",
			nil, false, nil,
			int(syscall.EACCES),
		},
		{
			"FAIL_DOWNLOAD_SD_CONNECT", rep1.ForPath() + "/project/bucket_1/dir_/another_file",
			&header{value: "header"}, true, errExpected,
			int(syscall.EIO),
		},
		{
			"FAIL_DOWNLOAD_SD_APPLY", rep2.ForPath() + "/old-bucket/dir4/another_file",
			&header{value: "header"}, true, errExpected,
			int(syscall.EIO),
		},
		{
			// Reading from the end of the file is not an error, it just returns no data
			"OK_OFFSET_AT_END", rep2.ForPath() + "/my-example.com/tiedosto",
			&header{value: "header"}, false, nil,
			0,
		},
	}

	origDownloadData := api.DownloadData
	origHeaders := fi.headers
	defer func() {
		api.DownloadData = origDownloadData
		fi.headers = origHeaders
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			testRoot := getTestFuse(t)
			node := findChildNode(t, testRoot, strings.Split(tt.path, "/")...)
			fi.headers = map[_Ctype_ino_t]header{}
			if tt.hdr != nil {
				fi.headers[node.stat.st_ino] = *tt.hdr
			}

			called := false
			api.DownloadData = func(rep api.Repo, nodes []string, path, owner, fileID, hdr string,
				startDecrypted, endDecrypted, oldOffset, fileSize int64,
			) ([]byte, error) {
				called = true
				if !tt.tryDownload {
					t.Errorf("api.DownloadData() should not be called")
				}

				return []byte("should not be returned"), tt.downloadErr
			}

			var errNo _Ctype_int
			var nBytes _Ctype_size_t
			buf := DownloadData(node, 34, 6, &errNo, &nBytes)
			if buf != nil {
				freeCPtr(unsafe.Pointer(buf))
				t.Errorf("Function should have returned nil buffer")
			}

			if tt.tryDownload && !called {
				t.Errorf("api.DownloadData() was not called")
			}
			if int(errNo) != tt.expectedErrNo {
				t.Errorf("Function set incorrect errNo. Expected=%v, received=%v",
					syscall.Errno(tt.expectedErrNo), syscall.Errno(errNo))
			}
			if nBytes != 0 {
				t.Errorf("Number of bytes should be 0, received %d", nBytes)
			}

			origFs := getTestFuse(t)
			if err := isValidFuse(origFs, testRoot, nil, ""); err != nil {
				t.Errorf("Filesystem affected: %s", err.Error())
			}
		})
	}
}
