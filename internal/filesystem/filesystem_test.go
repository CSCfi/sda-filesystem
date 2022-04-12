package filesystem

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/billziss-gh/cgofuse/fuse"
)

var testFuse = `{
	"name": "",
	"nameSafe": "",
	"size": -421,
    "children": [
		{
			"name": "Rep1",
			"nameSafe": "Rep1",
			"size": -416,
			"children": [
				{
					"name": "child+1",
					"nameSafe": "child_1",
					"size": -200,
					"children": [
						{
							"name": "kansio",
							"nameSafe": "kansio",
							"size": -88,
							"children": [
								{
									"name": "file_1",
									"nameSafe": "file_1",
									"size": 23,
									"children": null
								},
								{
									"name": "file_2",
									"nameSafe": "file_2",
									"size": 45,
									"children": null
								},
								{
									"name": "file_3",
									"nameSafe": "file_3",
									"size": 20,
									"children": null
								}
							]
						},
						{
							"name": "dir+",
							"nameSafe": "dir_",
							"size": 112,
							"children": [
								{
									"name": "folder",
									"nameSafe": "folder",
									"size": 112,
									"children": []
								}
							]
						}
					]
				},
				{
					"name": "child_2",
					"nameSafe": "child_2",
					"size": -216,
					"children": [
						{
							"name": "dir",
							"nameSafe": "dir",
							"size": 151,
							"children": []
						},
						{
							"name": "+folder",
							"nameSafe": "_folder",
							"size": 65,
							"children": [
								{
									"name": "file_1",
									"nameSafe": "file_1",
									"size": 3,
									"children": null
								},
								{
									"name": "test",
									"nameSafe": "test",
									"size": 62,
									"children": null
								}
							]
						}
					]
				},
				{
					"name": "child+2",
					"nameSafe": "child_2(3e08d3)",
					"size": 0,
					"children": []
				}
			]
		},
		{
			"name": "Rep2",
			"nameSafe": "Rep2",
			"size": -5,
			"children": [
				{
					"name": "https://example.com",
					"nameSafe": "example.com",
					"size": 5,
					"children": [
						{
							"name": "tiedosto",
							"nameSafe": "tiedosto",
							"size": 5,
							"children": null
						}
					]
				}
			]
		}
	]
}`

var testObjects = `{
	"name": "dir1",
	"nameSafe": "dir1",
	"size": 187,
	"children": [
		{
			"name": "dir+2",
			"nameSafe": "dir_2",
			"size": 137,
			"children": [
				{
					"name": "dir3.2.1",
					"nameSafe": "dir3.2.1",
					"size": 30,
					"children": [
						{
							"name": "file.c4gh",
							"nameSafe": "file(1bb764)",
							"size": 29,
							"children": null
						},
						{
							"name": "file",
							"nameSafe": "file",
							"size": 1,
							"children": [
								{
									"name": "h%e%ll+o",
									"nameSafe": "h%e%ll_o",
									"size": 1,
									"children": null
								}
							]
						}
					]
				},
				{
					"name": "dir3.2.1",
					"nameSafe": "dir3(fb761a).2.1",
					"size": 6,
					"children": null
				},
				{
					"name": "logs",
					"nameSafe": "logs",
					"size": 101,
					"children": null
				}
			]
		},
		{
			"name": "dir4",
			"nameSafe": "dir4",
			"size": 50,
			"children": [
				{
					"name": "another_file",
					"nameSafe": "another_file",
					"size": 10,
					"children": null
				},
				{
					"name": "another_file.c4gh",
					"nameSafe": "another_file(63af19)",
					"size": 13,
					"children": null
				},
				{
					"name": "another+file.c4gh",
					"nameSafe": "another_file(07fed4)",
					"size": 27,
					"children": null
				}
			]
		}
	]
}`

const rep1 = "Rep1"
const rep2 = "Rep2"

type jsonNode struct {
	Name     string      `json:"name"`
	NameSafe string      `json:"nameSafe"`
	Size     int64       `json:"size"`
	Children *[]jsonNode `json:"children"`
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(i int, s []string) {})
	os.Exit(m.Run())
}

// getTestFuse returns a *Fuse filled in based on variable testFuse
func getTestFuse(t *testing.T, sizeUnfinished bool, maxLevel int) (fs *Fuse) {
	var nodes jsonNode
	if err := json.Unmarshal([]byte(testFuse), &nodes); err != nil {
		t.Fatalf("Could not unmarshal json: %s", err.Error())
	}

	fs = &Fuse{}
	fs.root = &node{}
	fs.root.stat.Mode = fuse.S_IFDIR | sRDONLY
	fs.root.chld = map[string]*node{}
	fs.root.stat.Ino = 1
	fs.openmap = map[uint64]nodeAndPath{1: {fs.root, []string{"path"}}}

	if sizeUnfinished && nodes.Size < 0 {
		fs.root.stat.Size = -1
	} else if nodes.Size < 0 {
		fs.root.stat.Size = -nodes.Size
	} else {
		fs.root.stat.Size = nodes.Size
	}

	assignChildren(fs.root, nodes.Children, sizeUnfinished, maxLevel, 2)
	return
}

func assignChildren(n *node, template *[]jsonNode, sizeUnfinished bool, maxLevel int, ino uint64) uint64 {
	for i, child := range *template {
		n.chld[child.NameSafe] = &node{}
		n.chld[child.NameSafe].originalName = child.Name
		n.chld[child.NameSafe].stat.Ino = ino

		if sizeUnfinished && child.Size < 0 {
			n.chld[child.NameSafe].stat.Size = -1
		} else if child.Size < 0 {
			n.chld[child.NameSafe].stat.Size = -child.Size
		} else {
			n.chld[child.NameSafe].stat.Size = child.Size
		}

		if child.Children != nil {
			n.chld[child.NameSafe].stat.Mode = fuse.S_IFDIR | sRDONLY
			n.chld[child.NameSafe].chld = map[string]*node{}
			if maxLevel > 0 {
				ino = assignChildren(n.chld[child.NameSafe], (*template)[i].Children, sizeUnfinished, maxLevel-1, ino+1)
			}
		} else {
			n.chld[child.NameSafe].stat.Mode = fuse.S_IFREG | sRDONLY
		}

		ino++
	}
	return ino
}

func isSameFuse(fs1 *node, fs2 *node, path string) error {
	if fs2.stat.Mode != fs1.stat.Mode {
		return fmt.Errorf("Mode not correct at node %s", path)
	}
	if fs2.stat.Size != fs1.stat.Size {
		return fmt.Errorf("Size not correct at node %s. Expected %d, received %d", path, fs1.stat.Size, fs2.stat.Size)
	}
	if fs2.originalName != fs1.originalName {
		return fmt.Errorf("Original name not correct at node %s. Expected %s, received %s", path, fs1.originalName, fs2.originalName)
	}
	if fuse.S_IFDIR != fs2.stat.Mode&fuse.S_IFMT {
		return nil
	}
	if fs2.chld == nil {
		return fmt.Errorf("chld field not initialized at node %s", path)
	}

	// Names of children of fs1 and fs2
	keys1 := make([]string, len(fs1.chld))
	keys2 := make([]string, len(fs2.chld))

	i := 0
	for k := range fs1.chld {
		keys1[i] = k
		i++
	}

	i = 0
	for k := range fs2.chld {
		keys2[i] = k
		i++
	}

	sort.Strings(keys1)
	sort.Strings(keys2)

	if !reflect.DeepEqual(keys1, keys2) {
		return fmt.Errorf("Children differ at node %s. Should be %v, received %v", path, keys1, keys2)
	}

	for _, k := range keys1 {
		if err := isSameFuse(fs1.chld[k], fs2.chld[k], strings.TrimPrefix(path+"/"+k, "/")); err != nil {
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
	defer CheckPanic()

	panic("Muahahahaa")
}

func TestInitializeFilesystem(t *testing.T) {
	origFs := getTestFuse(t, true, 1)

	origNewNode := newNode
	origEnabledRepositories := api.GetEnabledRepositories
	origNthLevel := api.GetNthLevel
	origRemoveInvalidChars := removeInvalidChars

	defer func() {
		newNode = origNewNode
		api.GetEnabledRepositories = origEnabledRepositories
		api.GetNthLevel = origNthLevel
		removeInvalidChars = origRemoveInvalidChars
	}()

	newNode = func(ino uint64, mode uint32, uid uint32, gid uint32, tmsp fuse.Timespec) *node {
		n := &node{}
		n.stat.Mode = mode
		n.chld = map[string]*node{}
		return n
	}
	api.GetEnabledRepositories = func() []string {
		return []string{rep1, rep2}
	}
	api.GetNthLevel = func(rep, fsPath string, nodes ...string) ([]api.Metadata, error) {
		if len(nodes) > 0 {
			return nil, fmt.Errorf("Third parameter of api.GetNthLevel() should have been empty, received %v", nodes)
		}
		if rep == rep1 {
			return []api.Metadata{{Bytes: -1, Name: "child+1"}, {Bytes: -1, Name: "child_2"}, {Bytes: 0, Name: "child+2"}}, nil
		} else if rep == rep2 {
			return []api.Metadata{{Bytes: 5, Name: "https://example.com"}}, nil
		}
		return nil, fmt.Errorf("api.GetNthLevel() received invalid repository %q", rep)
	}
	removeInvalidChars = func(str string) string {
		return strings.ReplaceAll(str, "+", "_")
	}

	ret := InitializeFileSystem(nil)
	if ret == nil || ret.root == nil {
		t.Fatal("Filesystem or root is nil")
	}
	if err := isSameFuse(origFs.root, ret.root, "/"); err != nil {
		t.Fatalf("FUSE was not created correctly: %s", err.Error())
	}
}

func TestRefreshFilesystem(t *testing.T) {
	fs := getTestFuse(t, false, 1)
	newFs := getTestFuse(t, false, 5)

	fs.RefreshFilesystem(newFs)

	if fs.ino != newFs.ino {
		t.Errorf("Ino was not correct. Expected=%d, received=%d", newFs.ino, fs.ino)
	}
	if fs.root != newFs.root {
		t.Errorf("Root was not correct\nExpected=%v\nReceived=%v", *newFs.root, *fs.root)
	}
	if reflect.ValueOf(fs.openmap).Pointer() != reflect.ValueOf(newFs.openmap).Pointer() {
		t.Errorf("Openmap was not correct\nExpected=%v\nReceived=%v", newFs.openmap, fs.openmap)
	}
}

func TestPopulateFilesystem(t *testing.T) {
	origFs := getTestFuse(t, false, 5)
	fs := getTestFuse(t, true, 1)

	origNewNode := newNode
	origCheckPanic := CheckPanic
	origNthLevel := api.GetNthLevel
	origRemoveInvalidChars := removeInvalidChars
	origCreateObjects := createObjects
	origLevelCount := api.LevelCount

	defer func() {
		newNode = origNewNode
		CheckPanic = origCheckPanic
		api.GetNthLevel = origNthLevel
		removeInvalidChars = origRemoveInvalidChars
		createObjects = origCreateObjects
		api.LevelCount = origLevelCount
	}()

	newNode = func(ino uint64, mode uint32, uid uint32, gid uint32, tmsp fuse.Timespec) *node {
		n := &node{}
		n.stat.Mode = mode
		n.chld = map[string]*node{}
		return n
	}
	CheckPanic = func() {}
	api.GetNthLevel = func(rep, fsPath string, nodes ...string) ([]api.Metadata, error) {
		if len(nodes) != 1 {
			return nil, fmt.Errorf("Third parameter of api.GetNthLevel() should have had length 1, received %v that has length %d", nodes, len(nodes))
		}
		if rep == rep1 {
			if nodes[0] == "child+1" {
				return []api.Metadata{{Bytes: -1, Name: "kansio"}, {Bytes: 112, Name: "dir+"}}, nil
			} else if nodes[0] == "child_2" {
				return []api.Metadata{{Bytes: 151, Name: "dir"}, {Bytes: 65, Name: "+folder"}}, nil
			} else {
				return nil, fmt.Errorf("api.GetNthLevel() received invalid project %s", rep+"/"+nodes[0])
			}
		} else if rep == rep2 {
			if nodes[0] == "https://example.com" {
				return []api.Metadata{{Bytes: 5, Name: "tiedosto"}}, nil
			} else {
				return nil, fmt.Errorf("api.GetNthLevel() received invalid project %s", rep+"/"+nodes[0])
			}
		}
		return nil, fmt.Errorf("api.GetNthLevel() received invalid repository %s", rep)
	}
	removeInvalidChars = func(str string) string {
		return strings.ReplaceAll(str, "+", "_")
	}
	createObjects = func(id int, jobs <-chan containerInfo, wg *sync.WaitGroup, send func(string, string, int)) {
		defer wg.Done()

		for j := range jobs {
			nodes := strings.Split(j.containerPath, "/")
			if len(nodes) != 3 {
				t.Errorf("Invalid containerPath %s", j.containerPath)
				continue
			}

			repository := nodes[0]
			project := nodes[1]
			container := nodes[2]

			if _, ok := origFs.root.chld[repository]; !ok {
				t.Errorf("Invalid repository %s in containerInfo", repository)
				continue
			}
			if _, ok := origFs.root.chld[repository].chld[project]; !ok {
				t.Errorf("Invalid project %s in containerInfo", repository+"/"+project)
				continue
			}
			if _, ok := origFs.root.chld[repository].chld[project].chld[container]; !ok {
				t.Errorf("Invalid container %s in containerInfo", repository+"/"+project+"/"+container)
				continue
			}
			for key, value := range origFs.root.chld[repository].chld[project].chld[container].chld {
				j.fs.root.chld[repository].chld[project].chld[container].chld[key] = value
			}
		}
	}
	api.LevelCount = func(rep string) int {
		if rep == rep1 {
			return 3
		} else if rep == rep2 {
			return 2
		}
		t.Fatalf("api.LevelCount() received invalid repository %s", rep)
		return 0
	}

	fs.PopulateFilesystem(nil)
	if err := isSameFuse(origFs.root, fs.root, "/"); err != nil {
		t.Fatalf("FUSE was not created correctly: %s", err.Error())
	}
}

func TestCreateObjects(t *testing.T) {
	origFs := getTestFuse(t, false, 5)
	fs := getTestFuse(t, false, 5)

	origNewNode := newNode
	origCheckPanic := CheckPanic
	origNthLevel := api.GetNthLevel
	origRemoveInvalidChars := removeInvalidChars

	defer func() {
		newNode = origNewNode
		CheckPanic = origCheckPanic
		api.GetNthLevel = origNthLevel
		removeInvalidChars = origRemoveInvalidChars
	}()

	rep := rep1
	pr := "child_2"
	cont := "dir"
	SetSignalBridge(nil)

	newNode = func(ino uint64, mode uint32, uid uint32, gid uint32, tmsp fuse.Timespec) *node {
		n := &node{}
		n.stat.Mode = mode
		n.chld = map[string]*node{}
		return n
	}
	CheckPanic = func() {}
	api.GetNthLevel = func(repository, fsPath string, nodes ...string) ([]api.Metadata, error) {
		if repository != rep {
			return nil, fmt.Errorf("GetNthLevel() received incorrect repository %s, expected %s", repository, rep)
		}
		if len(nodes) != 2 {
			return nil, fmt.Errorf("GetNthLevel() received invalid third parameter %v", nodes)
		}
		if nodes[0] != pr {
			t.Fatalf("GetNthLevel() received incorrect project %s, expected %q", rep+"/"+nodes[0], rep+"/"+pr)
		}
		if nodes[1] != cont {
			t.Fatalf("GetNthLevel() received incorrect container %q, expected %q", rep+"/"+pr+"/"+nodes[1], rep+"/"+pr+"/"+cont)
		}
		return []api.Metadata{{Bytes: 29, Name: "dir1/dir+2/dir3.2.1/file.c4gh"},
			{Bytes: 1, Name: "dir1/dir+2/dir3.2.1/file/h%e%ll+o"},
			{Bytes: 1, Name: "dir1/dir5/"},
			{Bytes: 6, Name: "dir1/dir+2/dir3.2.1"},
			{Bytes: 101, Name: "dir1/dir+2/logs"},
			{Bytes: 10, Name: "dir1/dir4/another_file"},
			{Bytes: 13, Name: "dir1/dir4/another_file.c4gh"},
			{Bytes: 27, Name: "dir1/dir4/another+file.c4gh"}}, nil
	}
	removeInvalidChars = func(str string) string {
		return strings.ReplaceAll(str, "+", "_")
	}

	var wg sync.WaitGroup
	jobs := make(chan containerInfo, 1)
	wg.Add(1)
	go createObjects(0, jobs, &wg, nil)
	jobs <- containerInfo{containerPath: rep + "/" + pr + "/" + cont, timestamp: fuse.Timespec{}, fs: fs}
	close(jobs)
	wg.Wait()

	var nodes jsonNode
	if err := json.Unmarshal([]byte(testObjects), &nodes); err != nil {
		t.Fatalf("Could not unmarshal json: %s", err.Error())
	}
	assignChildren(origFs.root.chld[rep].chld[pr].chld[cont], &[]jsonNode{nodes}, false, 5, 1)

	if err := isSameFuse(origFs.root, fs.root, ""); err != nil {
		t.Errorf("Objects not added correctly: %s", err.Error())
	}
}

func TestCreateObjects_Get_Node_Fail(t *testing.T) {
	origFs := getTestFuse(t, false, 5)
	fs := getTestFuse(t, false, 5)

	origCheckPanic := CheckPanic
	origNthLevel := api.GetNthLevel

	defer func() {
		CheckPanic = origCheckPanic
		api.GetNthLevel = origNthLevel
	}()

	CheckPanic = func() {}
	api.GetNthLevel = func(rep, fsPath string, nodes ...string) ([]api.Metadata, error) {
		return nil, nil
	}

	var wg sync.WaitGroup
	jobs := make(chan containerInfo, 1)
	wg.Add(1)
	go createObjects(0, jobs, &wg, nil)
	jobs <- containerInfo{containerPath: "Rep3/child_2/dir", timestamp: fuse.Timespec{}, fs: fs}
	close(jobs)
	wg.Wait()

	if err := isSameFuse(origFs.root, fs.root, ""); err != nil {
		t.Errorf("Fuse should not have been modified: %s", err.Error())
	}
}

func TestCreateObjects_Nth_Level_Fail(t *testing.T) {
	origFs := getTestFuse(t, false, 5)
	fs := getTestFuse(t, false, 5)

	origCheckPanic := CheckPanic
	origNthLevel := api.GetNthLevel

	defer func() {
		CheckPanic = origCheckPanic
		api.GetNthLevel = origNthLevel
	}()

	CheckPanic = func() {}
	api.GetNthLevel = func(rep, fsPath string, nodes ...string) ([]api.Metadata, error) {
		return nil, errors.New("Error occurred")
	}

	var wg sync.WaitGroup
	jobs := make(chan containerInfo, 1)
	wg.Add(1)
	go createObjects(0, jobs, &wg, nil)
	jobs <- containerInfo{containerPath: "Rep1/child_2/dir", timestamp: fuse.Timespec{}, fs: fs}
	close(jobs)
	wg.Wait()

	if err := isSameFuse(origFs.root, fs.root, ""); err != nil {
		t.Errorf("Fuse should not have been modified: %s", err.Error())
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
		testname := fmt.Sprintf("REMOVE_%d", i+1)
		t.Run(testname, func(t *testing.T) {
			ret := removeInvalidChars(tt.original)
			if ret != tt.modified {
				t.Errorf("String %s should have become %s, got %s", tt.original, tt.modified, ret)
			}
		})
	}
}

func TestNewNode(t *testing.T) {
	var tests = []struct {
		dir  bool
		ino  uint64
		tmsp fuse.Timespec
	}{
		{true, 32, fuse.Now()},
		{false, 57, fuse.NewTimespec(time.Now().AddDate(0, 0, -1))},
	}

	for i, tt := range tests {
		testname := fmt.Sprintf("NEW_NODE_%d", i+1)
		t.Run(testname, func(t *testing.T) {
			var mode uint32
			if tt.dir {
				mode = fuse.S_IFDIR | sRDONLY
			} else {
				mode = fuse.S_IFREG | sRDONLY
			}

			node := newNode(tt.ino, mode, 0, 0, tt.tmsp)

			if node == nil {
				t.Error("Node is nil")
				return
			}

			if node.stat.Ino != tt.ino {
				t.Errorf("File serial number incorrect. Expected %d, got %d", tt.ino, node.stat.Ino)
			} else if node.stat.Mode != mode {
				t.Errorf("Mode incorrect. Expected %d, got %d", mode, node.stat.Mode)
			} else if node.stat.Atim != tt.tmsp {
				t.Errorf("Atim field incorrect. Expected %q, got %q", tt.tmsp.Time().String(), node.stat.Atim.Time().String())
			} else if node.stat.Ctim != tt.tmsp {
				t.Errorf("Ctim field incorrect. Expected %q, got %q", tt.tmsp.Time().String(), node.stat.Ctim.Time().String())
			} else if node.stat.Mtim != tt.tmsp {
				t.Errorf("Mtim field incorrect. Expected %q, got %q", tt.tmsp.Time().String(), node.stat.Mtim.Time().String())
			} else if node.stat.Birthtim != tt.tmsp {
				t.Errorf("Birthtim field incorrect. Expected %q, got %q", tt.tmsp.Time().String(), node.stat.Birthtim.Time().String())
			} else if tt.dir && node.chld == nil {
				t.Errorf("Node's chld field was not initialized")
			}
		})
	}
}

func TestLookupNode(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	var tests = []struct {
		testname, path string
		nodeMatch      *node
		origPath       []string
	}{
		{
			"OK_1", "Rep1/child_2/_folder/",
			fs.root.chld["Rep1"].chld["child_2"].chld["_folder"],
			[]string{"Rep1", "child_2", "+folder"},
		},
		{
			"OK_2", "Rep1/child_1///dir_////folder",
			fs.root.chld["Rep1"].chld["child_1"].chld["dir_"].chld["folder"],
			[]string{"Rep1", "child+1", "dir+", "folder"},
		},
		{
			"OK_3", "Rep2/example.com/tiedosto",
			fs.root.chld["Rep2"].chld["example.com"].chld["tiedosto"],
			[]string{"Rep2", "https://example.com", "tiedosto"},
		},
		{
			"NOT_FOUND_1", "Rep4/child_2/folder/file_3", nil, []string{""},
		},
		{
			"NOT_FOUND_2", "Rep1/child_1//dir_/folder///another_folder", nil, []string{""},
		},
	}

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			node, origPath := lookupNode(fs.root, tt.path)

			if tt.nodeMatch == nil {
				if node != nil {
					t.Errorf("Should not have returned node for path %s", tt.path)
				}
			} else if node == nil {
				t.Errorf("Returned nil for path %s", tt.path)
			} else if node != tt.nodeMatch {
				t.Errorf("Node incorrect for path %q. Expected address %p, got %p", tt.path, tt.nodeMatch, node)
			} else if !reflect.DeepEqual(origPath, tt.origPath) {
				t.Errorf("Original path incorrect for path %s\nExpected %v\nReceived %v", tt.path, tt.origPath, origPath)
			}
		})
	}
}
