package filesystem

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

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
		t.Errorf("Checking for header changed filesystem incorrectly: %s", err.Error())
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
				t.Errorf("Checking for header changed filesystem: %s", err.Error())
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
		t.Errorf("Checking for header changed filesystem incorrectly: %s", err.Error())
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
		t.Errorf("Checking for header changed filesystem: %s", err.Error())
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
		t.Errorf("Checking for header changed filesystem: %s", err.Error())
	}
}
