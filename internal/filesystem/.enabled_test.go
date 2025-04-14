package filesystem

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sda-filesystem/internal/api"
	"strings"
	"testing"

	"github.com/billziss-gh/cgofuse/fuse"
)

func TestOpen(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	var tests = []struct {
		testname, path string
		node           *node
		errc           int
	}{
		{"OK_1", rep1 + "/child_2/_folder/test", fs.root.chld[rep1].chld["child_2"].chld["_folder"].chld["test"], 0},
		{"OK_2", "/" + rep2 + "/example.com/tiedosto", fs.root.chld[rep2].chld["example.com"].chld["tiedosto"], 0},
		{"NOT_FOUND", rep2 + "/example.com/file", nil, -fuse.ENOENT},
		{"NOT_FILE", rep1 + "/child_1/dir_", nil, -fuse.EISDIR},
	}

	origIsValidOpen := isValidOpen
	origUpdateAttributes := api.UpdateAttributes
	defer func() {
		isValidOpen = origIsValidOpen
		api.UpdateAttributes = origUpdateAttributes
	}()

	isValidOpen = func() bool { return true }
	api.UpdateAttributes = func(nodes []string, fsPath string, attr any) error {
		return &api.RequestError{StatusCode: 404}
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			errc, fh := fs.Open(tt.path, 0)
			switch {
			case errc != tt.errc:
				t.Errorf("Error code incorrect. Expected %d, received %d", tt.errc, errc)
			case tt.node != nil:
				if fh != tt.node.stat.Ino {
					t.Errorf("File handle incorrect. Expected %d, received %d", tt.node.stat.Ino, fh)
				} else if fs.openmap[tt.node.stat.Ino].node != tt.node {
					t.Errorf("Filesystem's openmap has incorrect value for file handle %d. Expected address %p, received %p",
						fh, tt.node, fs.openmap[tt.node.stat.Ino].node)
				}
			case fh != ^uint64(0):
				t.Errorf("File handle incorrect. Expected %d, received %d", ^uint64(0), fh)
			}
		})
	}
}

func TestOpen_Cancel(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	origIsValidOpen := isValidOpen
	defer func() { isValidOpen = origIsValidOpen }()

	isValidOpen = func() bool { return false }

	errc, fh := fs.Open("/prevent/this/path/from/opening", 0)
	if errc != -fuse.ECANCELED {
		t.Fatalf("Error code incorrect. Expected %d, received %d", -fuse.ECANCELED, errc)
	}
	if fh != ^uint64(0) {
		t.Errorf("File handle incorrect. Expected %d, received %d", ^uint64(0), fh)
	}
}

func TestOpen_Decryption_Check(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	var tests = []struct {
		testname string
		nodes    []string
		fh       uint64
		sizes    []int64
	}{
		{
			"OK_1", []string{api.SDConnect, "child_2", "_folder", "test"},
			fs.root.chld[rep1].chld["child_2"].chld["_folder"].chld["test"].stat.Ino,
			[]int64{444, 244, 93, 90},
		},
		{
			"OK_2", []string{api.SDConnect, "child_1", "kansio", "file_3"},
			fs.root.chld[rep1].chld["child_1"].chld["kansio"].chld["file_3"].stat.Ino,
			[]int64{401, 185, 73, 5},
		},
	}

	origIsValidOpen := isValidOpen
	origUpdateAttributes := api.UpdateAttributes
	defer func() {
		isValidOpen = origIsValidOpen
		api.UpdateAttributes = origUpdateAttributes
	}()

	isValidOpen = func() bool { return true }

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.UpdateAttributes = func(nodes []string, fsPath string, attr any) error {
				size, ok := attr.(*int64)
				if !ok {
					return fmt.Errorf("updateAttributes() was called with incorrect attribute. Expected type *int64, got %v", reflect.TypeOf(attr))
				}
				*size = tt.sizes[len(tt.sizes)-1]

				return nil
			}

			fs := getTestFuse(t, false, 5)
			fs.root.chld[api.SDConnect] = fs.root.chld[rep1]
			fs.root.chld[api.SDConnect].originalName = api.SDConnect

			path := strings.Join(tt.nodes, "/")
			errc, fh := fs.Open(path, 0)
			switch {
			case errc != 0:
				t.Errorf("Error code incorrect for path %s. Expected 0, received %d", path, errc)
			case fh != tt.fh:
				t.Errorf("File handle incorrect for path %s. Expected %d, received %d", path, tt.fh, fh)
			case !fs.openmap[fh].node.decryptionChecked:
				t.Errorf("Field 'decyptionChecked' is not true for node %s", path)
			default:
				prnt := fs.root
				for i := range tt.nodes {
					prnt = prnt.chld[tt.nodes[i]]
					if prnt.stat.Size != tt.sizes[i] {
						t.Errorf("Node %s on path %s has incorrect size. Expected %d, received %d", tt.nodes[i], path, tt.sizes[i], prnt.stat.Size)
					}
				}
			}
		})
	}
}

func TestOpen_Decryption_Check_Error(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	var tests = []struct {
		testname         string
		nodes            []string
		denied           bool
		fh               uint64
		errc, httpStatus int
	}{
		{
			"FAIL_500", []string{api.SDConnect, "child_2", "_folder", "file_1"}, false,
			fs.root.chld[rep1].chld["child_2"].chld["_folder"].chld["file_1"].stat.Ino, -fuse.EIO, 500,
		},
		{
			"FAIL_451", []string{api.SDConnect, "child_1", "kansio", "file_2"}, true,
			fs.root.chld[rep1].chld["child_1"].chld["kansio"].chld["file_2"].stat.Ino, -fuse.EACCES, 451,
		},
	}

	origIsValidOpen := isValidOpen
	origUpdateAttributes := api.UpdateAttributes
	defer func() {
		isValidOpen = origIsValidOpen
		api.UpdateAttributes = origUpdateAttributes
	}()

	isValidOpen = func() bool { return true }

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.UpdateAttributes = func(nodes []string, fsPath string, attr any) error {
				return &api.RequestError{StatusCode: tt.httpStatus}
			}

			fs := getTestFuse(t, false, 5)
			fs.root.chld[api.SDConnect] = fs.root.chld[rep1]
			fs.root.chld[api.SDConnect].originalName = api.SDConnect

			path := strings.Join(tt.nodes, "/")
			errc, fh := fs.Open(path, 0)

			switch {
			case errc != tt.errc:
				t.Errorf("Error code incorrect for path %s. Expected %d, received %d", path, tt.errc, errc)
			case fh != ^uint64(0):
				t.Errorf("File handle incorrect for path %s. Expected %d, received %d", path, ^uint64(0), fh)
			default:
				node := fs.root
				for i := range tt.nodes {
					node = node.chld[tt.nodes[i]]
				}
				if tt.denied == !node.decryptionChecked {
					t.Errorf("Field 'decryptionChecked' incorrect. Expected %t, received %t", tt.denied, node.decryptionChecked)
				} else if tt.denied == !node.denied {
					t.Errorf("Field 'denied' incorrect. Expected %t, received %t", tt.denied, node.denied)
				}
			}
		})
	}
}

func TestOpendir(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	var tests = []struct {
		testname, path string
		node           *node
		errc           int
	}{
		{"OK_1", "/" + rep1 + "/child_1/kansio", fs.root.chld[rep1].chld["child_1"].chld["kansio"], 0},
		{"OK_2", rep2 + "/example.com/", fs.root.chld[rep2].chld["example.com"], 0},
		{"NOT_FOUND", rep1 + "/child_3", nil, -fuse.ENOENT},
		{"NOT_DIR", rep1 + "/child_2/_folder/file_1", nil, -fuse.ENOTDIR},
	}

	origUpdateAttributes := api.UpdateAttributes
	defer func() { api.UpdateAttributes = origUpdateAttributes }()

	api.UpdateAttributes = func(nodes []string, fsPath string, attr any) error {
		return nil
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			errc, fh := fs.Opendir(tt.path)

			switch {
			case errc != tt.errc:
				t.Errorf("Error code incorrect. Expected %d, received %d", tt.errc, errc)
			case tt.node != nil:
				if fh != tt.node.stat.Ino {
					t.Errorf("File handle incorrect. Expected %d, received %d", tt.node.stat.Ino, fh)
				} else if fs.openmap[tt.node.stat.Ino].node != tt.node {
					t.Errorf("Filesystem's openmap has incorrect value for file handle %d. Expected address %p, received %p",
						fh, tt.node, fs.openmap[tt.node.stat.Ino].node)
				}
			case fh != ^uint64(0):
				t.Errorf("File handle incorrect. Expected %d, received %d", ^uint64(0), fh)
			}
		})
	}
}

func TestRelease(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	node := fs.root.chld[rep2].chld["example.com"].chld["tiedosto"]
	fs.openmap[node.stat.Ino] = nodeAndPath{node: node}
	node.opencnt = 2
	ret := fs.Release(rep2+"/example.com/tiedosto", node.stat.Ino)
	switch {
	case ret != 0:
		t.Errorf("Return value incorrect. Expected=0, received=%d", ret)
	case node.opencnt != 1:
		t.Errorf("Node that was closed should have opencnt=1, received=%d", node.opencnt)
	case fs.openmap[node.stat.Ino].node == nil:
		t.Errorf("Node should not have been removed from openmap")
	}

	ret = fs.Release(rep2+"/example.com/tiedosto", node.stat.Ino)

	switch {
	case ret != 0:
		t.Errorf("Return value incorrect. Expected=0, received=%d", ret)
	case node.opencnt != 0:
		t.Errorf("Node that was closed should have opencnt=0, received=%d", node.opencnt)
	case fs.openmap[node.stat.Ino].node != nil:
		t.Errorf("Node should have been removed from openmap")
	}

	if ret := fs.Release(rep2+"/example.com/tiedosto", node.stat.Ino); ret != -fuse.ENOENT {
		t.Errorf("Return value incorrect. Expected=0, received=%d", ret)
	}
}

func TestReleaseDir(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	node := fs.root.chld[rep1].chld["child_1"].chld["kansio"]
	fs.openmap[node.stat.Ino] = nodeAndPath{node: node}
	node.opencnt = 2
	ret := fs.Releasedir(rep1+"/child_1/kansio", node.stat.Ino)
	switch {
	case ret != 0:
		t.Errorf("Return value incorrect. Expected=0, received=%d", ret)
	case node.opencnt != 1:
		t.Errorf("Node that was closed should have opencnt=1, received=%d", node.opencnt)
	case fs.openmap[node.stat.Ino].node == nil:
		t.Errorf("Node should not have been removed from openmap")
	}

	ret = fs.Releasedir(rep1+"/child_1/kansio", node.stat.Ino)

	switch {
	case ret != 0:
		t.Errorf("Return value incorrect. Expected=0, received=%d", ret)
	case node.opencnt != 0:
		t.Errorf("Node that was closed should have opencnt=0, received=%d", node.opencnt)
	case fs.openmap[node.stat.Ino].node != nil:
		t.Errorf("Node should have been removed from openmap")
	}

	if ret := fs.Releasedir(rep1+"/child_1/kansio", node.stat.Ino); ret != -fuse.ENOENT {
		t.Errorf("Return value incorrect. Expected=0, received=%d", ret)
	}
}

func TestGetattr(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	node := fs.root.chld[rep2].chld["example.com"]
	node.stat.Atim = fuse.Now()
	node.stat.Uid = 4
	fs.openmap[node.stat.Ino] = nodeAndPath{node: node}

	var stat fuse.Stat_t
	errc := fs.Getattr(rep2+"/example.com", &stat, node.stat.Ino)

	if errc != 0 {
		t.Errorf("Return value incorrect. Expected=0, received=%d", errc)
	} else if !reflect.DeepEqual(stat, node.stat) {
		t.Errorf("Stat defined incorrectly\nExpected=%v\nReceived=%v", node.stat, stat)
	}

	errc = fs.Getattr(rep2+"/example.com/does_not_exist", &stat, ^uint64(0))

	if errc != -fuse.ENOENT {
		t.Errorf("Return value incorrect. Expected=0, received=%d", errc)
	} else if !reflect.DeepEqual(stat, node.stat) {
		t.Errorf("Stat should not have been modified. Received=%v", stat)
	}
}

func TestRead(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	var tests = []struct {
		testname, path     string
		data, finalContent string
		node               *node
		ofst               int64
		ret, len           int
	}{
		{
			"OK_1", "/" + rep1 + "/child_1/kansio/file_1",
			"All work and no play makes Jack a dull boy", "All work a",
			fs.root.chld[rep1].chld["child_1"].chld["kansio"].chld["file_1"],
			0, 10, 10,
		},
		{
			"OK_2", rep2 + "/example.com/tiedosto",
			"I am very important data. Nice to meet you.",
			"very important data. Nice to meet you.",
			fs.root.chld[rep2].chld["example.com"].chld["tiedosto"],
			5, 38, 100,
		},
		{
			"OK_3", rep1 + "/child_2/_folder/test",
			"This data is too short", "",
			fs.root.chld[rep1].chld["child_2"].chld["_folder"].chld["test"],
			50, 0, 25,
		},
	}

	origDownloadData := api.DownloadData
	defer func() { api.DownloadData = origDownloadData }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.DownloadData = func(nodes []string, path string, start, end, maxEnd int64) ([]byte, error) {
				return []byte(tt.data)[start:end], nil
			}

			buff := make([]byte, tt.len)
			fh := tt.node.stat.Ino
			tt.node.stat.Size = int64(len(tt.data))
			fs.openmap[fh] = nodeAndPath{node: tt.node, path: []string{}}

			ret := fs.Read(tt.path, buff, tt.ofst, fh)
			buff = bytes.Trim(buff, "\x00") // Trim trailing zeroes from buffer

			if ret != tt.ret {
				t.Errorf("Incorrect return value for node %s. Expected %d, received %d", tt.path, tt.ret, ret)
			} else if !reflect.DeepEqual(tt.finalContent, string(buff)) {
				t.Errorf("Buffer incorrect.\nExpected=%s\nReceived=%s", tt.finalContent, string(buff))
			}
		})
	}
}

func TestRead_Error(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	var tests = []struct {
		testname, path string
		node           *node
		ret            int
		denied         bool
	}{
		{
			"NOT_FOUND", rep1 + "/child_4/file", nil, -fuse.ENOENT, false,
		},
		{
			"DOWNLOAD_ERROR", rep2 + "/example.com/tiedosto",
			fs.root.chld[rep2].chld["example.com"].chld["tiedosto"], -fuse.EIO, false,
		},
		{
			"NO_ACCESS", rep1 + "/child_2/_folder/file_1",
			fs.root.chld[rep1].chld["child_2"].chld["_folder"].chld["file_1"], -fuse.EACCES, true,
		},
	}

	origDownloadData := api.DownloadData
	defer func() { api.DownloadData = origDownloadData }()

	api.DownloadData = func(nodes []string, path string, start, end, maxEnd int64) ([]byte, error) {
		return nil, errors.New("Error occurred")
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			buff := make([]byte, 50)

			var fh uint64 = 100
			if tt.node != nil {
				fh = tt.node.stat.Ino
				tt.node.denied = tt.denied
				fs.openmap[fh] = nodeAndPath{node: tt.node, path: []string{}}
			}

			if ret := fs.Read(tt.path, buff, 0, fh); ret != tt.ret {
				t.Errorf("Return value incorrect for path %s. Expected %d, received %d", tt.path, tt.ret, ret)
			}
		})
	}
}

func TestReaddir(t *testing.T) {
	fs := getTestFuse(t, false, 5)

	node := fs.root.chld[rep1]
	fs.openmap[node.stat.Ino] = nodeAndPath{node: node}

	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		if name == ".." {
			if stat != nil {
				t.Errorf("Fill function received incorrect stat for name '..'\nExpected=nil\nReceived=%v", *stat)
			}
		} else if name == "." {
			if stat != &node.stat {
				t.Errorf("Fill function received incorrect stat for name '.'\nExpected=%v\nReceived=%v", node.stat, *stat)
			}
		} else if n, ok := node.chld[name]; ok {
			if stat != &n.stat {
				t.Errorf("Fill function received incorrect stat for name %q\nExpected=%v\nReceived=%v", name, node.stat, *stat)
			}
		} else {
			t.Errorf("Invalid name %q", name)
		}

		return false
	}

	errc := fs.Readdir("", fill, 0, node.stat.Ino)

	if errc != 0 {
		t.Errorf("Return value incorrect. Expected=0, received=%d", errc)
	}

	errc = fs.Readdir(rep2+"/dummy", fill, 0, ^uint64(0))

	if errc != -fuse.ENOENT {
		t.Errorf("Return value incorrect. Expected=0, received=%d", errc)
	}
}
