package filesystem

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
)

const rep1 = api.SDConnect
const rep2 = api.SDApply

var testFuse = `{
	"name": "",
	"nameSafe": "",
	"size": 650,
	"modified": "2025-07-31T23:00:05Z",
	"children": [
		{
			"name": "Bad-Repo",
			"nameSafe": "Bad-Repo",
			"size": 0,
			"modified": "1970-01-01T00:00:00Z",
			"dir": true
		},
		{
			"name": "` + rep2 + `",
			"nameSafe": "` + rep2 + `",
			"size": 192,
			"modified": "2025-07-31T23:00:05Z",
			"children": [
				{
					"name": "bad-bucket",
					"nameSafe": "bad-bucket",
					"size": 0,
					"modified": "1970-01-01T00:00:00Z",
					"children": [],
					"dir": true
  				},
				{
					"name": "https://my-example.com",
					"nameSafe": "my-example.com",
					"size": 5,
					"modified": "2025-07-31T23:00:05Z",
					"children": [
						{
							"name": "tiedosto",
							"nameSafe": "tiedosto",
							"size": 5,
							"modified": "2025-07-31T23:00:05Z",
							"children": null
						}
					]
				},
				{
					"name": "old-bucket",
					"nameSafe": "old-bucket",
					"size": 187,
					"modified": "2020-02-13T12:00:00Z",
					"children": [
						{
							"name": "dir4",
							"nameSafe": "dir4",
							"size": 50,
							"modified": "2020-02-13T12:00:00Z",
							"children": [
								{
									"name": "another_file",
									"nameSafe": "another_file",
									"size": 10,
									"modified": "2020-02-13T12:00:00Z",
									"children": null
								},
								{
									"name": "another+file.c4gh",
									"nameSafe": "another_file(07fed4)",
									"size": 27,
									"modified": "2020-02-13T12:00:00Z",
									"children": null
								},
								{
									"name": "another_file.c4gh",
									"nameSafe": "another_file(63af19)",
									"size": 13,
									"modified": "2020-02-13T12:00:00Z",
									"children": null
								}
							]
						},
						{
							"name": "dir+2",
							"nameSafe": "dir_2",
							"size": 137,
							"modified": "2020-02-13T12:00:00Z",
							"children": [
								{
									"name": "dir3.2.1",
									"nameSafe": "dir3(fb761a).2.1",
									"size": 6,
									"modified": "2020-02-13T12:00:00Z",
									"children": null
								},
								{
									"name": "dir3.2.1",
									"nameSafe": "dir3.2.1",
									"size": 30,
									"modified": "2020-02-13T12:00:00Z",
									"children": [
										{
											"name": "file",
											"nameSafe": "file",
											"size": 1,
											"modified": "2020-02-13T12:00:00Z",
											"children": [
												{
													"name": "h%e%ll+o",
													"nameSafe": "h_e_ll_o",
													"size": 1,
													"modified": "2020-02-13T12:00:00Z",
													"children": null
												}
											]
										},
										{
											"name": "file.c4gh",
											"nameSafe": "file(1bb764)",
											"size": 29,
											"modified": "2020-02-13T12:00:00Z",
											"children": null
										}
									]
								},
								{
									"name": "logs",
									"nameSafe": "logs",
									"size": 101,
									"modified": "2020-02-13T12:00:00Z",
									"children": null
								}
							]
						}
					]
				}
            ]
        },
        {
            "name": "` + rep1 + `",
            "nameSafe": "` + rep1 + `",
            "size": 458,
			"modified": "2020-12-30T10:00:00Z",
            "children": [
				{
					"name": "project",
					"nameSafe": "project",
					"size": 458,
					"modified": "2020-12-30T10:00:00Z",
					"children": [
						{
							"name": "bucket_1",
							"nameSafe": "bucket_1",
							"size": 200,
							"modified": "2020-12-30T10:00:00Z",
							"children": [
								{
									"name": "dir+",
									"nameSafe": "dir_",
									"size": 112,
									"modified": "2006-01-02T15:04:05Z",
									"children": [
										{
											"name": "another_file",
											"nameSafe": "another_file",
											"size": 112,
											"modified": "2006-01-02T15:04:05Z",
											"children": null
										}
									]
								},
								{
									"name": "kansio",
									"nameSafe": "kansio",
									"size": 88,
									"modified": "2020-12-30T10:00:00Z",
									"children": [
										{
											"name": "file_1",
											"nameSafe": "file_1",
											"size": 23,
											"modified": "2006-01-02T15:04:05Z",
											"children": null
										},
										{
											"name": "file_2",
											"nameSafe": "file_2",
											"size": 45,
											"modified": "2016-11-02T15:04:05Z",
											"children": null
										},
										{
											"name": "file@3",
											"nameSafe": "file_3",
											"size": 10,
											"modified": "2020-12-30T10:00:00Z",
											"children": null
										},
										{
											"name": "file_3",
											"nameSafe": "file_3(d8b8f5)",
											"size": 10,
											"modified": "2010-01-24T18:34:05Z",
											"children": null
										}
									]
								}
							]
						},
						{
							"name": "bucket_2",
							"nameSafe": "bucket_2",
							"size": 65,
							"modified": "2011-01-02T10:45:55Z",
							"children": [
								{
									"name": "?folder",
									"nameSafe": "_folder",
									"size": 65,
									"modified": "2011-01-02T10:45:55Z",
									"children": [
										{
											"name": "file_1",
											"nameSafe": "file_1",
											"size": 3,
											"modified": "2011-01-02T10:45:55Z",
											"children": null
										},
										{
											"name": "test",
											"nameSafe": "test",
											"size": 62,
											"modified": "2000-02-22T05:33:05Z",
											"children": null
										}
									]
								}
							]
						},
						{
							"name": "bucket_3",
							"nameSafe": "bucket_3",
							"size": 151,
							"modified": "2000-02-22T05:33:05Z",
							"children": [
								{
									"name": "testi",
									"nameSafe": "testi",
									"size": 151,
									"modified": "2000-02-22T05:33:05Z",
									"children": null
								}
							]
						},
						{
							"name": "shared_bucket",
							"nameSafe": "shared_bucket",
							"size": 42,
							"modified": "1999-09-12T06:30:00Z",
							"children": [
								{
									"name": "shared-file.txt",
									"nameSafe": "shared-file.txt",
									"size": 42,
									"modified": "1999-09-12T06:30:00Z",
									"children": null
								}
							]
						},
						{
							"name": "shared#bucket",
							"nameSafe": "shared_bucket(58dca8)",
							"size": 0,
							"modified": "2000-01-15T19:00:00Z",
							"children": [
								{
									"name": "shared-file-2.txt",
									"nameSafe": "shared-file-2.txt",
									"size": 0,
									"modified": "2000-01-15T19:00:00Z",
									"children": null
								}
							]
						}
					]
				}
			]
        },
		{
			"name": "Substandard-Repo",
			"nameSafe": "Substandard-Repo",
			"size": 0,
			"modified": "1970-01-01T00:00:00Z",
			"children": [],
			"dir": true
		}
	]
}`

type jsonNode struct {
	Name     string      `json:"name"`
	NameSafe string      `json:"nameSafe"`
	Size     int64       `json:"size"`
	Modified *time.Time  `json:"modified"`
	Children *[]jsonNode `json:"children"`
	Dir      bool        `json:"dir"` // Force node with no children to be a directory
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(string, []string) {})
	os.Exit(m.Run())
}

// getTestFuse returns a *FsState filled in based on variable testFuse
func getTestFuse(t *testing.T) *_Ctype_struct_Node {
	var root jsonNode
	if err := json.Unmarshal([]byte(testFuse), &root); err != nil {
		t.Fatalf("Could not unmarshal json: %s", err.Error())
	}

	fsRoot := &_Ctype_struct_Node{}
	t.Cleanup(func() { freeNodes(fsRoot) })

	assignChildren(fsRoot, root)
	fsRoot.stat.st_ino = 1

	return fsRoot
}

func assignChildren(node *_Ctype_struct_Node, template jsonNode) {
	node.name = toCStr(template.NameSafe)
	node.orig_name = toCStr(template.Name)
	node.stat.st_size = _Ctype_off_t(template.Size)
	node.last_modified.tv_sec = _Ctype_time_t(template.Modified.Unix())
	node.stat.st_ino = _Ctype_ino_t(uintptr(unsafe.Pointer(node)))
	node.offset = -1

	hasChildren := template.Children != nil && len(*template.Children) > 0
	isDir := template.Dir || hasChildren

	node.stat.st_mode = syscall.S_IFREG | 0644
	if isDir {
		node.stat.st_mode = syscall.S_IFDIR | 0744
	}
	if !isDir {
		return
	}

	// Real directories are always given an allocated children array, even when empty
	childCount := 0
	if hasChildren {
		childCount = len(*template.Children)
	}
	node.children = allocateNodeList(_Ctype_size_t(childCount))
	node.chld_count = _Ctype_size_t(childCount)
	if !hasChildren {
		return
	}

	nodeSlice := unsafe.Slice(node.children, childCount)

	for i, child := range *template.Children {
		nodeSlice[i] = allocateNodePointer()
		nodeSlice[i].parent = node
		assignChildren(nodeSlice[i], child)
	}
}

// findChildNode walks down node via child names, matching against each node's
// safe name in turn. It does not rely on search_node/find_child_node so it can
// be used to independently compute an expected node in tests.
func findChildNode(t *testing.T, node *_Ctype_struct_Node, names ...string) *_Ctype_struct_Node {
	for _, name := range names {
		if node == nil || node.children == nil {
			t.Fatalf("Failed to find node %s", strings.Join(names, "/"))
		}

		var next *_Ctype_struct_Node
		for _, child := range unsafe.Slice(node.children, node.chld_count) {
			if toGoStr(child.name) == name {
				next = child

				break
			}
		}
		node = next
	}

	return node
}

func isValidFuse(origNode *_Ctype_struct_Node, node, parent *_Ctype_struct_Node, path string) error {
	if toGoStr(node.orig_name) != toGoStr(origNode.orig_name) {
		return fmt.Errorf("original name not correct at node %q. Expected=%s, received=%s", path, toGoStr(origNode.orig_name), toGoStr(node.orig_name))
	}
	if node.stat.st_size != origNode.stat.st_size {
		return fmt.Errorf("size not correct at node %q. Expected=%d, received=%d", path, origNode.stat.st_size, node.stat.st_size)
	}
	if parent == nil && node.stat.st_ino != 1 {
		return fmt.Errorf("ino not correct at node %q. Expected=1, received=%d", path, node.stat.st_ino)
	} else if parent != nil && uintptr(node.stat.st_ino) != uintptr(unsafe.Pointer(node)) {
		return fmt.Errorf("ino not correct at node %q. Expected=%d, received=%d", path, uintptr(unsafe.Pointer(node)), uintptr(node.stat.st_ino))
	}
	if node.stat.st_mode != origNode.stat.st_mode {
		return fmt.Errorf("mode not correct at node %q. Expected=%d, received=%d", path, origNode.stat.st_mode, node.stat.st_mode)
	}
	if origNode.last_modified != node.last_modified {
		return fmt.Errorf("timestamp not correct at node %q, Expected=%+v, received=%+v", path, origNode.last_modified, node.last_modified)
	}
	if origNode.children == nil && node.children != nil {
		return fmt.Errorf("node %q should not have children", path)
	}
	if origNode.children != nil && node.children == nil {
		return fmt.Errorf("node %q should have children", path)
	}
	if node.parent != parent {
		return fmt.Errorf("parent not correct at node %q. Expected=%p, received=%p", path, parent, node.parent)
	}
	if node.children == nil {
		if origNode.offset != node.offset {
			return fmt.Errorf("node %q should have offset %d, received %d", path, origNode.offset, node.offset)
		}

		return nil
	}

	// Names of children of node and origNode
	keys1 := make([]string, origNode.chld_count)
	keys2 := make([]string, node.chld_count)

	slice1 := unsafe.Slice(origNode.children, origNode.chld_count)
	for i := range slice1 {
		keys1[i] = toGoStr(slice1[i].name)
	}

	slice2 := unsafe.Slice(node.children, node.chld_count)
	for i := range slice2 {
		keys2[i] = toGoStr(slice2[i].name)
	}

	if !reflect.DeepEqual(keys1, keys2) {
		return fmt.Errorf("children differ at node %q\nExpected=%v\nReceived=%v", path, keys1, keys2)
	}

	for i := range keys1 {
		if err := isValidFuse(slice1[i], slice2[i], node, path+"/"+keys1[i]); err != nil {
			return err
		}
	}

	return nil
}

func createFuseFiles(node *_Ctype_struct_Node, path string) error {
	nodePath := path + "/" + toGoStr(node.name)
	if node.stat.st_mode&syscall.S_IFREG > 0 {
		err := os.WriteFile(nodePath, []byte("I am a file"), 0600)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %s", nodePath, err.Error())
		}

		return nil
	}

	if err := os.MkdirAll(nodePath, 0755); err != nil {
		return fmt.Errorf("failed to create folder %s: %s", nodePath, err.Error())
	}
	if node.chld_count == 0 {
		return nil
	}

	children := unsafe.Slice(node.children, node.chld_count)
	for i := range children {
		err := createFuseFiles(children[i], nodePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestSetSignalBridge_And_CheckPanic(t *testing.T) {
	called := false
	SetSignalBridge(func() {
		called = true
	})

	defer func() {
		if !called {
			t.Fatal("signalBridge() not called even though code paniced")
		}
	}()
	defer checkPanic()

	panic("Muahahahaa")
}

func TestGenerateFilesystem(t *testing.T) {
	testRoot := getTestFuse(t)

	origGetRepositories := api.GetRepositories
	origGetProjectName := api.GetProjectName
	origGetBuckets := api.GetBuckets
	origGetObjects := api.GetObjects
	origGetSegmentedObjects := api.GetSegmentedObjects
	origHeaders := fi.headers
	defer func() {
		api.GetRepositories = origGetRepositories
		api.GetProjectName = origGetProjectName
		api.GetBuckets = origGetBuckets
		api.GetObjects = origGetObjects
		api.GetSegmentedObjects = origGetSegmentedObjects
		fi.headers = origHeaders
	}()

	api.GetRepositories = func() []api.Repo {
		return []api.Repo{rep1, rep2, "Bad-Repo", "Substandard-Repo"}
	}
	api.GetProjectName = func() string {
		return "project"
	}
	api.GetBuckets = func(rep api.Repo) ([]api.Metadata, error) {
		switch rep {
		case rep1:
			return []api.Metadata{
				{Name: "bucket_1_segments"},
				{Name: "bucket_1"},
				{Name: "bucket_2_segments"},
				{Name: "bucket_2"},
				{Name: "bucket_3"},
				{Name: "shared#bucket", Owner: "sharing-project-1"},
				{Name: "shared_bucket", Owner: "sharing-project-2"},
			}, nil
		case rep2:
			return []api.Metadata{
				{Name: "https://my-example.com", Owner: "muumi"},
				{Name: "bad-bucket"},
				{Name: "bad-bucket_segments"},
				{Name: "old-bucket"},
				{Name: "old-bucket_segments"},
			}, nil
		case "Substandard-Repo":
			return nil, nil
		}

		return nil, fmt.Errorf("api.GetBuckets() received invalid repository %q", rep)
	}
	api.GetObjects = func(rep api.Repo, bucket, path, owner, prefix string) ([]api.Metadata, error) {
		switch rep {
		case rep1:
			switch bucket {
			case "bucket_1":
				time1, _ := time.Parse(time.RFC3339, "2020-12-30T10:00:00Z")
				time2, _ := time.Parse(time.RFC3339, "2010-01-24T18:34:05Z")
				time3, _ := time.Parse(time.RFC3339, "2016-11-02T15:04:05Z")
				time4, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
				time5, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")

				return []api.Metadata{
					{Size: 10, Name: "kansio/file@3", LastModified: &time1},
					{Size: 10, Name: "kansio/file_3", LastModified: &time2},
					{Size: 45, Name: "kansio/file_2", LastModified: &time3},
					{Size: 23, Name: "kansio/file_1", LastModified: &time4},
					{Size: 0, Name: "dir+/another_file", LastModified: &time5},
				}, nil
			case "bucket_2":
				time1, _ := time.Parse(time.RFC3339, "2000-02-22T05:33:05Z")
				time2, _ := time.Parse(time.RFC3339, "2011-01-02T10:45:55Z")

				return []api.Metadata{
					{Size: 62, Name: "?folder/test", LastModified: &time1},
					{Size: 3, Name: "?folder/file_1", LastModified: &time2},
				}, nil
			case "bucket_3":
				time1, _ := time.Parse(time.RFC3339, "2000-02-22T05:33:05Z")

				return []api.Metadata{
					{Size: 151, Name: "testi", LastModified: &time1},
				}, nil
			case "shared_bucket":
				time1, _ := time.Parse(time.RFC3339, "1999-09-12T06:30:00Z")

				return []api.Metadata{
					{Size: 42, Name: "shared-file.txt", LastModified: &time1},
				}, nil
			case "shared#bucket":
				time1, _ := time.Parse(time.RFC3339, "2000-01-15T19:00:00Z")

				return []api.Metadata{
					{Size: 0, Name: "shared-file-2.txt", LastModified: &time1},
				}, nil
			default:
				return nil, fmt.Errorf("api.GetObjects() received invalid %s bucket %s", rep, bucket)
			}
		case rep2:
			switch bucket {
			case "https://my-example.com":
				time1, _ := time.Parse(time.RFC3339, "2025-07-31T23:00:05Z")

				return []api.Metadata{
					{Size: 5, Name: "tiedosto", LastModified: &time1, ID: "e72b6f25-62df-4a03-bf07-1f0b35a9684e"},
				}, nil
			case "old-bucket":
				time1, _ := time.Parse(time.RFC3339, "2020-02-13T12:00:00Z")

				return []api.Metadata{
					{Size: 0, Name: "dir+2/dir3.2.1/file.c4gh", LastModified: &time1},
					{Size: 1, Name: "dir+2/dir3.2.1/file/h%e%ll+o", LastModified: &time1},
					{Size: 1, Name: "dir5/", LastModified: &time1},
					{Size: 6, Name: "dir+2/dir3.2.1", LastModified: &time1},
					{Size: 0, Name: "dir+2/logs", LastModified: &time1},
					{Size: 0, Name: "dir4/another_file", LastModified: &time1},
					{Size: 0, Name: "dir4/another_file.c4gh", LastModified: &time1},
					{Size: 27, Name: "dir4/another+file.c4gh", LastModified: &time1},
				}, nil
			}

			return nil, fmt.Errorf("api.GetObjects() received invalid %s bucket %s", rep, bucket)
		}

		return nil, fmt.Errorf("api.GetObjects() received invalid repository %s", rep)
	}
	api.GetSegmentedObjects = func(rep api.Repo, bucket string) ([]api.Metadata, error) {
		switch rep {
		case rep1:
			if bucket == "bucket_1_segments" {
				return []api.Metadata{
					{Size: 112, Name: "dir+/another_file/fyvutilbiyni/00000001", LastModified: nil},
				}, nil
			}

			return nil, fmt.Errorf("api.GetObjects() received invalid %s bucket %s", rep, bucket)
		case rep2:
			switch bucket {
			case "https://my-example.com_segments":
				return []api.Metadata{{Size: 5, Name: "tiedosto/gybtvtro6vtrob/00000001", LastModified: nil}}, nil
			case "old-bucket_segments":
				return []api.Metadata{
					{Size: 4, Name: "dir+2/dir3.2.1/file.c4gh/ftkuvdticyidtyvi/00000001", LastModified: nil},
					{Size: 5, Name: "dir+2/dir3.2.1/file.c4gh/ftkuvdticyidtyvi/00000002", LastModified: nil},
					{Size: 50, Name: "dir+2/logs/driyvfyuvfubofyuv/00000001", LastModified: nil},
					{Size: 5, Name: "dir4/another_file.c4gh/ftuovrubotov/00000002", LastModified: nil},
					{Size: 11, Name: "dir+2/logs/driyvfyuvfubofyuv/00000002", LastModified: nil},
					{Size: 40, Name: "dir+2/logs/driyvfyuvfubofyuv/00000003", LastModified: nil},
					{Size: 11, Name: "dir+2/dir3.2.1/file.c4gh/ftkuvdticyidtyvi/00000003", LastModified: nil},
					{Size: 9, Name: "dir+2/dir3.2.1/file.c4gh/ftkuvdticyidtyvi/00000004", LastModified: nil},
					{Size: 10, Name: "dir4/another_file/fkuvruycvrurui/00000001", LastModified: nil},
					{Size: 8, Name: "dir4/another_file.c4gh/ftuovrubotov/00000001", LastModified: nil},
				}, nil
			}

			return nil, fmt.Errorf("api.GetObjects() received invalid %s bucket %s", rep, bucket)
		}

		return nil, fmt.Errorf("api.GetObjects() received invalid repository %s", rep)
	}

	GenerateFilesystem()
	if fi.root.cNode == testRoot {
		t.Fatal("Global root should not point to test root")
	}
	if fi.state.root != nil {
		t.Fatal("C state root should not be defined")
	}
	t.Cleanup(func() { freeNodes(fi.root.cNode); fi.root.cNode.chld_count = 0 })
	if err := isValidFuse(testRoot, fi.root.cNode, nil, ""); err != nil {
		t.Fatalf("FUSE was not created correctly: %s", err.Error())
	}
	expectedHeaders := map[string]header{
		"/SD-Apply/my-example.com/tiedosto":                           {fileID: "e72b6f25-62df-4a03-bf07-1f0b35a9684e", owner: "muumi"},
		"/SD-Connect/project/shared_bucket/shared-file.txt":           {owner: "sharing-project-2"},
		"/SD-Connect/project/shared_bucket(58dca8)/shared-file-2.txt": {owner: "sharing-project-1"},
	}
	for path, expectedHdr := range expectedHeaders {
		node := searchNode(path)
		if node == nil {
			t.Fatalf("Could not find node %s", path)
		}
		header, ok := fi.headers[node.stat.st_ino]
		if !ok {
			t.Fatalf("No header found for node %s", path)
		}
		if header != expectedHdr {
			t.Errorf("Incorrect header for node %s\nExpected=%v\nReceived=%v", path, expectedHdr, header)
		}
		delete(fi.headers, node.stat.st_ino)
	}
	if len(fi.headers) > 0 {
		t.Fatalf("Too many headers, leftover: %+v", fi.headers)
	}
}

func TestRemoveInvalidChars(t *testing.T) {
	var tests = []struct {
		original, modified string
	}{
		{"b.a:d!s/t_r@i+n|g", "b.a_d_s_t_r_i_n_g"},
		{"qwerty__\"###hello<html>$$money$$", "qwerty______hello_html___money__"},
		{"%_csc::>d>p>%%'hello'", "__csc___d_p____hello_"},
	}

	for i, tt := range tests {
		testname := fmt.Sprintf("OK_%d", i+1)
		t.Run(testname, func(t *testing.T) {
			ret := removeInvalidChars(tt.original)
			if ret != tt.modified {
				t.Errorf("String %s should have become %s, got %s", tt.original, tt.modified, ret)
			}
		})
	}
}
