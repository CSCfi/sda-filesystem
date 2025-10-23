package filesystem

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"sda-filesystem/internal/api"
	"sda-filesystem/test"
)

var errExpected = errors.New("expected error for test")

func TestSearchNode(t *testing.T) {
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	var tests = []struct {
		testname, path string
		nodeMatch      *_Ctype_node_t
	}{
		{
			"OK_1", rep1.ForPath() + "/project/bucket_2/_folder/", &nodeSlice[33],
		},
		{
			"OK_2", rep1.ForPath() + "/project/bucket_1///dir_////another_file", &nodeSlice[28],
		},
		{
			"OK_3", "/" + rep2.ForPath() + "/my-example.com/tiedosto", &nodeSlice[8],
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
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	var tests = []struct {
		testname string
		node     *_Ctype_node_t
		origPath []string
	}{
		{
			"OK_1", &nodeSlice[33], []string{"", rep1.ForPath(), "project", "bucket_2", "?folder"},
		},
		{
			"OK_2", &nodeSlice[28], []string{"", rep1.ForPath(), "project", "bucket_1", "dir+", "another_file"},
		},
		{
			"OK_3", &nodeSlice[8], []string{"", rep2.ForPath(), "https://my-example.com", "tiedosto"},
		},
	}

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			origPath := getNodePathNames(tt.node)

			if !reflect.DeepEqual(origPath, tt.origPath) {
				t.Errorf("Original path incorrect\nExpected=%v\nReceived=%v", tt.origPath, origPath)
			}
		})
	}
}

func TestGetNodeChildren(t *testing.T) {
	fi.nodes = getTestFuse(t)

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

func TestClearPath(t *testing.T) {
	fi.nodes = getTestFuse(t)
	path := rep1.ForPath() + "/project/bucket_1/kansio"

	traverse := map[string]bool{
		rep1.ForPath() + "/bucket_1/kansio/file_1": true,
		rep1.ForPath() + "/bucket_1/kansio/file_2": true,
		rep1.ForPath() + "/bucket_1/kansio/file_3": true,
	}

	origDeleteFileFromCache := api.DeleteFileFromCache
	origObjects := api.GetObjects
	origGetHeaders := api.GetHeaders
	origGetObjectSizesFromSegments := getObjectSizesFromSegments
	origSharedBucketProject := api.SharedBucketProject
	origHeaders := fi.headers
	defer func() {
		api.DeleteFileFromCache = origDeleteFileFromCache
		api.GetObjects = origObjects
		api.GetHeaders = origGetHeaders
		getObjectSizesFromSegments = origGetObjectSizesFromSegments
		api.SharedBucketProject = origSharedBucketProject
		fi.headers = origHeaders
	}()

	fi.headers = map[_Ctype_ino_t]string{30: "vlfvyugyvli", 32: "hbfyucdtkyv", 33: "bftcdvtuftu"}
	api.DeleteFileFromCache = func(rep api.Repo, nodes []string, size int64) {
		delete(traverse, rep.ForPath()+"/"+strings.Join(nodes, "/"))
	}
	time1, _ := time.Parse(time.RFC3339, "2008-10-12T22:10:00Z")
	time2, _ := time.Parse(time.RFC3339, "2017-01-24T08:30:45Z")
	time3, _ := time.Parse(time.RFC3339, "2001-05-01T10:04:05Z")
	api.GetObjects = func(rep api.Repo, bucket, path string, prefix ...string) ([]api.Metadata, error) {
		if rep != rep1 || bucket != "bucket_1" {
			t.Errorf("api.GetObjects() received incorrect repository or bucket")
		}
		if len(prefix) == 0 {
			t.Errorf("api.GetObjects() should have received prefix")
		}
		if prefix[0] != "kansio/" {
			t.Errorf("api.GetObjects() received incorrect prefix. Expected=kansio/, received=%s", prefix[0])
		}

		return []api.Metadata{
			{Size: 45, Name: "kansio/file_1", LastModified: &time1},
			{Size: 6, Name: "kansio/file_2", LastModified: &time2},
			{Size: 142, Name: "kansio/file_3", LastModified: &time3},
		}, nil
	}
	api.GetHeaders = func(rep api.Repo,
		buckets []api.Metadata,
		sharedBuckets map[string]api.SharedBucketsMeta,
	) (api.BatchHeaders, error) {
		exists := slices.ContainsFunc(buckets, func(meta api.Metadata) bool {
			return meta.Name == "bucket_1"
		})
		if !exists {
			t.Errorf("api.GetHeaders() was not instructed to fetch headers for bucket bucket_1")
		}
		if len(sharedBuckets) > 0 {
			t.Errorf("api.GetHeaders() received content in the sharedBuckets map: %v", sharedBuckets)
		}

		batch := make(api.BatchHeaders)
		batch["bucket_1"] = make(map[string]api.VaultHeaderVersions)

		batch["bucket_1"]["kansio/file_1"] = api.VaultHeaderVersions{
			Headers: map[string]api.VaultHeader{
				"1": {Header: "yvdyviditybf"},
			},
			LatestVersion: 1,
		}
		batch["bucket_1"]["kansio/file_2"] = api.VaultHeaderVersions{
			Headers: map[string]api.VaultHeader{
				"3": {Header: "hbfyucdtkyv"},
				"4": {Header: "hubftiuvfti"},
			},
			LatestVersion: 4,
		}

		return batch, nil
	}
	api.SharedBucketProject = func(bucket string) (string, error) {
		return "", nil
	}
	getObjectSizesFromSegments = func(rep api.Repo, bucket string) (map[string]int64, error) {
		return nil, errExpected
	}

	diff := _Ctype_off_t(115)
	origFs := getTestFuse(t)
	nodeSlice := unsafe.Slice(origFs.nodes, fsSize)

	nodeSlice[0].stat.st_size += diff
	nodeSlice[3].stat.st_size += diff
	nodeSlice[20].stat.st_size += diff
	nodeSlice[21].stat.st_size += diff
	nodeSlice[27].stat.st_size += diff
	nodeSlice[29].stat.st_size = 45
	nodeSlice[29].last_modified.tv_sec = _Ctype_time_t(time1.Unix())
	nodeSlice[30].stat.st_size = 6
	nodeSlice[30].last_modified.tv_sec = _Ctype_time_t(time2.Unix())
	nodeSlice[32].stat.st_size = 142
	nodeSlice[32].last_modified.tv_sec = _Ctype_time_t(time3.Unix())

	if err := ClearPath(path); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if origFs.count != fi.nodes.count {
		t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
	}
	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Clearing path changed filesystem incorrectly: %s", err.Error())
	}
	if len(traverse) > 0 {
		t.Errorf("Function did not clear files %v", slices.Collect(maps.Keys(traverse)))
	}
	expectedHeaders := map[_Ctype_ino_t]string{29: "yvdyviditybf", 30: "hubftiuvfti", 33: "bftcdvtuftu"}
	if !reflect.DeepEqual(expectedHeaders, fi.headers) {
		t.Errorf("Headers incorrect\nExpected=%v\nReceived=%v", expectedHeaders, fi.headers)
	}
}

func TestClearPath_Shared(t *testing.T) {
	fi.nodes = getTestFuse(t)
	path := rep1.ForPath() + "/project/shared-bucket"

	origDeleteFileFromCache := api.DeleteFileFromCache
	origObjects := api.GetObjects
	origGetHeaders := api.GetHeaders
	origGetObjectSizesFromSegments := getObjectSizesFromSegments
	origSharedBucketProject := api.SharedBucketProject
	origHeaders := fi.headers
	defer func() {
		api.DeleteFileFromCache = origDeleteFileFromCache
		api.GetObjects = origObjects
		api.GetHeaders = origGetHeaders
		getObjectSizesFromSegments = origGetObjectSizesFromSegments
		api.SharedBucketProject = origSharedBucketProject
		fi.headers = origHeaders
	}()

	fi.headers = map[_Ctype_ino_t]string{33: "bftcdvtuftu"}
	api.DeleteFileFromCache = func(rep api.Repo, nodes []string, size int64) {}
	time1, _ := time.Parse(time.RFC3339, "2008-10-12T22:10:00Z")
	api.GetObjects = func(rep api.Repo, bucket, path string, prefix ...string) ([]api.Metadata, error) {
		if rep != rep1 || bucket != "shared-bucket" {
			t.Errorf("api.GetObjects() received incorrect repository or bucket")
		}
		if len(prefix) > 0 && len(prefix[0]) > 0 {
			t.Errorf("api.GetObjects() should not have received prefix %v", prefix)
		}

		return []api.Metadata{
			{Size: 45, Name: "shared-file.txt", LastModified: &time1},
		}, nil
	}
	api.GetHeaders = func(rep api.Repo,
		buckets []api.Metadata,
		sharedBuckets map[string]api.SharedBucketsMeta,
	) (api.BatchHeaders, error) {
		if len(buckets) > 0 {
			t.Errorf("api.GetHeaders() received content in the buckets slice: %v", buckets)
		}
		expectedSharedBuckets := map[string]api.SharedBucketsMeta{
			"sharing-project-1": {"shared-bucket"},
		}
		if !reflect.DeepEqual(expectedSharedBuckets, sharedBuckets) {
			t.Errorf("api.GetHeaders() received invalid shared buckets\nExpected=%v\nReceived=%v", expectedSharedBuckets, sharedBuckets)
		}

		batch := make(api.BatchHeaders)
		batch["shared-bucket"] = make(map[string]api.VaultHeaderVersions)

		batch["shared-bucket"]["shared-file.txt"] = api.VaultHeaderVersions{
			Headers: map[string]api.VaultHeader{
				"1": {Header: "gycorubgiul"},
				"2": {Header: "hgvdrxtivfy"},
			},
			LatestVersion: 2,
		}

		return batch, nil
	}
	api.SharedBucketProject = func(bucket string) (string, error) {
		return "sharing-project-1", nil
	}
	getObjectSizesFromSegments = func(rep api.Repo, bucket string) (map[string]int64, error) {
		return nil, errExpected
	}

	diff := _Ctype_off_t(3)
	origFs := getTestFuse(t)
	nodeSlice := unsafe.Slice(origFs.nodes, fsSize)

	nodeSlice[0].stat.st_size += diff
	nodeSlice[3].stat.st_size += diff
	nodeSlice[20].stat.st_size += diff
	nodeSlice[24].stat.st_size += diff
	nodeSlice[24].last_modified.tv_sec = _Ctype_time_t(time1.Unix())
	nodeSlice[37].stat.st_size = 45
	nodeSlice[37].last_modified.tv_sec = _Ctype_time_t(time1.Unix())

	if err := ClearPath(path); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if origFs.count != fi.nodes.count {
		t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
	}
	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Clearing path changed filesystem incorrectly: %s", err.Error())
	}
	expectedHeaders := map[_Ctype_ino_t]string{33: "bftcdvtuftu", 37: "hgvdrxtivfy"}
	if !reflect.DeepEqual(expectedHeaders, fi.headers) {
		t.Errorf("Headers incorrect\nExpected=%v\nReceived=%v", expectedHeaders, fi.headers)
	}
}

func TestClearPath_Segments(t *testing.T) {
	fi.nodes = getTestFuse(t)

	// Switch children of SD Connect and SD Apply
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)
	nodeSlice[2], nodeSlice[3] = nodeSlice[3], nodeSlice[2]
	nodeSlice[20].parent = &nodeSlice[2]
	nodeSlice[5].parent = &nodeSlice[3]
	nodeSlice[6].parent = &nodeSlice[3]
	nodeSlice[7].parent = &nodeSlice[3]
	nodeSlice[2].name, nodeSlice[3].name = nodeSlice[3].name, nodeSlice[2].name
	nodeSlice[2].orig_name, nodeSlice[3].orig_name = nodeSlice[3].orig_name, nodeSlice[2].orig_name

	path := rep1.ForPath() + "/old-bucket/dir_2" // old-bucket is treated like the project in this case

	traverse := map[string]bool{
		rep1.ForPath() + "/dir+2/dir3.2.1/file.c4gh":     true,
		rep1.ForPath() + "/dir+2/dir3.2.1/file/h%e%ll+o": true,
		rep1.ForPath() + "/dir+2/logs":                   true,
	}

	origDeleteFileFromCache := api.DeleteFileFromCache
	origObjects := api.GetObjects
	origGetHeaders := api.GetHeaders
	origGetObjectSizesFromSegments := getObjectSizesFromSegments
	origSharedBucketProject := api.SharedBucketProject
	origHeaders := fi.headers
	defer func() {
		api.DeleteFileFromCache = origDeleteFileFromCache
		api.GetObjects = origObjects
		api.GetHeaders = origGetHeaders
		getObjectSizesFromSegments = origGetObjectSizesFromSegments
		api.SharedBucketProject = origSharedBucketProject
		fi.headers = origHeaders
	}()

	fi.headers = map[_Ctype_ino_t]string{16: "vlfvyugyvli", 19: "hbfyucdtkyv"}
	api.DeleteFileFromCache = func(rep api.Repo, nodes []string, size int64) {
		delete(traverse, rep.ForPath()+"/"+strings.Join(nodes, "/"))
	}
	time1, _ := time.Parse(time.RFC3339, "2011-04-24T03:38:45Z")
	time2, _ := time.Parse(time.RFC3339, "2023-07-10T23:11:00Z")
	time3, _ := time.Parse(time.RFC3339, "2021-05-01T10:04:05Z")
	api.GetObjects = func(rep api.Repo, bucket, path string, prefix ...string) ([]api.Metadata, error) {
		if rep != rep1 || bucket != "dir+2" {
			t.Errorf("api.GetObjects() received incorrect repository or bucket")
		}
		if len(prefix) == 0 {
			t.Errorf("api.GetObjects() should have received prefix")
		}
		if prefix[0] != "" {
			t.Errorf("api.GetObjects() received incorrect prefix. Expected=, received=%s", prefix[0])
		}

		return []api.Metadata{
			{Size: 42, Name: "logs", LastModified: &time1},
			{Size: 0, Name: "dir3.2.1/file.c4gh", LastModified: &time2},
			{Size: 0, Name: "dir3.2.1/file/h%e%ll+o", LastModified: &time3},
		}, nil
	}
	api.GetHeaders = func(
		rep api.Repo,
		buckets []api.Metadata,
		sharedBuckets map[string]api.SharedBucketsMeta,
	) (api.BatchHeaders, error) {
		exists := slices.ContainsFunc(buckets, func(meta api.Metadata) bool {
			return meta.Name == "dir+2"
		})
		if !exists {
			t.Errorf("api.GetHeaders() was not instructed to fetch headers for bucket dir+2")
		}

		return nil, nil
	}
	api.SharedBucketProject = func(bucket string) (string, error) {
		return "", nil
	}
	getObjectSizesFromSegments = func(rep api.Repo, bucket string) (map[string]int64, error) {
		if rep != rep1 || bucket != "dir+2" {
			t.Errorf("getObjectSizesFromSegments() received incorrect repository or bucket")
		}

		return map[string]int64{
			"dir3.2.1/file.c4gh":     34,
			"dir3.2.1/file/h%e%ll+o": 17,
		}, nil
	}

	diff := _Ctype_off_t(38)
	origFs := getTestFuse(t)

	// Switch children of SD Connect and SD Apply
	nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
	nodeSlice[2], nodeSlice[3] = nodeSlice[3], nodeSlice[2]
	nodeSlice[20].parent = &nodeSlice[2]
	nodeSlice[5].parent = &nodeSlice[3]
	nodeSlice[6].parent = &nodeSlice[3]
	nodeSlice[7].parent = &nodeSlice[3]
	nodeSlice[2].name, nodeSlice[3].name = nodeSlice[3].name, nodeSlice[2].name
	nodeSlice[2].orig_name, nodeSlice[3].orig_name = nodeSlice[3].orig_name, nodeSlice[2].orig_name

	nodeSlice[0].stat.st_size -= diff
	nodeSlice[3].stat.st_size -= diff
	nodeSlice[7].stat.st_size -= diff
	nodeSlice[7].last_modified.tv_sec = _Ctype_time_t(time2.Unix())
	nodeSlice[10].stat.st_size -= diff
	nodeSlice[10].last_modified.tv_sec = _Ctype_time_t(time2.Unix())
	nodeSlice[15].stat.st_size += _Ctype_off_t(21)
	nodeSlice[15].last_modified.tv_sec = _Ctype_time_t(time2.Unix())
	nodeSlice[17].stat.st_size += _Ctype_off_t(16)
	nodeSlice[17].last_modified.tv_sec = _Ctype_time_t(time3.Unix())
	nodeSlice[16].stat.st_size = 42
	nodeSlice[16].last_modified.tv_sec = _Ctype_time_t(time1.Unix())
	nodeSlice[18].stat.st_size = 34
	nodeSlice[18].last_modified.tv_sec = _Ctype_time_t(time2.Unix())
	nodeSlice[19].stat.st_size = 17
	nodeSlice[19].last_modified.tv_sec = _Ctype_time_t(time3.Unix())

	if err := ClearPath(path); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if origFs.count != fi.nodes.count {
		t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
	}
	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Clearing path changed filesystem incorrectly: %s", err.Error())
	}
	if len(traverse) > 0 {
		t.Errorf("Function did not clear files %v", slices.Collect(maps.Keys(traverse)))
	}
	if len(fi.headers) > 0 {
		t.Errorf("Headers slice should be empty. Received=%v", fi.headers)
	}
}

func TestClearPath_BadPath(t *testing.T) {
	fi.nodes = getTestFuse(t)
	origFs := getTestFuse(t)

	path := "/" + rep1.ForPath() + "/project/bucket-4"
	errStr := "path " + path + " is invalid"

	if err := ClearPath(path); err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
	if origFs.count != fi.nodes.count {
		t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
	}
	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Clearing path changed filesystem: %s", err.Error())
	}

	path = "/" + rep2.ForPath() + "/old-bucket/dir4"
	errStr = "clearing cache only enabled for SD Connect"

	if err := ClearPath(path); err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
	if origFs.count != fi.nodes.count {
		t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
	}
	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Clearing path changed filesystem: %s", err.Error())
	}

	path = "/" + rep1.ForPath() + "/project"
	errStr = "path needs to include at least a bucket"

	if err := ClearPath(path); err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
	if origFs.count != fi.nodes.count {
		t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
	}
	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Clearing path changed filesystem: %s", err.Error())
	}
}

func TestClearPath_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr                string
		objectErr, headerErr, sharedErr error
	}{
		{
			"FAIL_1",
			"cache not cleared since new file sizes could not be obtained: " + errExpected.Error(),
			errExpected, nil, nil,
		},
		{
			"FAIL_2",
			"failed to get headers for bucket bucket_3: " + errExpected.Error(),
			nil, errExpected, nil,
		},
		{
			"FAIL_3",
			"could not determine if bucket bucket_3 is shared: " + errExpected.Error(),
			nil, nil, errExpected,
		},
	}

	origObjects := api.GetObjects
	origGetHeaders := api.GetHeaders
	origSharedBucketProject := api.SharedBucketProject
	origHeaders := fi.headers
	defer func() {
		api.GetObjects = origObjects
		api.GetHeaders = origGetHeaders
		api.SharedBucketProject = origSharedBucketProject
		fi.headers = origHeaders
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetObjects = func(rep api.Repo, bucket, path string, prefix ...string) ([]api.Metadata, error) {
				return nil, tt.objectErr
			}
			api.GetHeaders = func(rep api.Repo, buckets []api.Metadata, sharedBuckets map[string]api.SharedBucketsMeta) (api.BatchHeaders, error) {
				return nil, tt.headerErr
			}
			api.SharedBucketProject = func(bucket string) (string, error) {
				return "", tt.sharedErr
			}
			fi.nodes = getTestFuse(t)
			origFs := getTestFuse(t)

			if err := ClearPath("/" + rep1.ForPath() + "/project/bucket_3"); err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
			if origFs.count != fi.nodes.count {
				t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
			}
			if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
				t.Errorf("Clearing path changed filesystem: %s", err.Error())
			}
		})
	}
}

func TestCheckHeaderExistence_Found(t *testing.T) {
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		t.Errorf("api.GetReencryptedHeader() should not be called")

		return "", 0, nil
	}

	node := &nodeSlice[35]
	node.offset = 0
	node.stat.st_size = 484
	fi.headers = map[_Ctype_ino_t]string{35: "hello"}

	CheckHeaderExistence(node, node.name) // second argument is only for logs, so is does not matter here what it is
	if node.offset != 0 {
		t.Errorf("Node offset incorrect. Expected=0, received=%d", node.offset)
	}
	if !reflect.DeepEqual(fi.headers, map[_Ctype_ino_t]string{35: "hello"}) {
		t.Errorf("Headers were modified to %v", fi.headers)
	}

	origFs := getTestFuse(t)
	nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
	nodeSlice[35].offset = 0
	nodeSlice[35].stat.st_size = 456
	nodeSlice[33].stat.st_size -= 28
	nodeSlice[22].stat.st_size -= 28
	nodeSlice[20].stat.st_size -= 28
	nodeSlice[3].stat.st_size -= 28
	nodeSlice[0].stat.st_size -= 28

	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Checking for header changed filesystem incorrectly: %s", err.Error())
	}
}

func TestCheckHeaderExistence_BadPath(t *testing.T) {
	var tests = []struct {
		testname string
		nodeIdx  int
	}{
		{"FAIL_1", 8},
		{"FAIL_2", 20},
	}

	origGetReencryptedHeader := api.GetReencryptedHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		t.Errorf("api.GetReencryptedHeader() should not be called")

		return "", 0, nil
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			fi.nodes = getTestFuse(t)
			nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

			node := &nodeSlice[tt.nodeIdx]
			node.offset = 0
			fi.headers = make(map[_Ctype_ino_t]string)

			CheckHeaderExistence(node, node.name) // second argument is only for logs, so is does not matter here what it is
			if len(fi.headers) > 0 {
				t.Errorf("Headers were modified to %v", fi.headers)
			}

			origFs := getTestFuse(t)
			nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
			nodeSlice[tt.nodeIdx].offset = 0

			if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
				t.Errorf("Checking for header changed filesystem: %s", err.Error())
			}
		})
	}
}

func TestCheckHeaderExistence_Reencrypted(t *testing.T) {
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		if bucket != "bucket_1" {
			t.Errorf("api.GetReencryptedHeader() received incorrect bucket. Expected=bucket_1, received=%s", bucket)
		}
		if object != "dir+/another_file" {
			t.Errorf("api.GetReencryptedHeader() received incorrect object. Expected=dir+/another_file, received=%s", bucket)
		}

		return "i-am-a-header", 58, nil
	}

	node := &nodeSlice[28]
	node.offset = 0
	fi.headers = make(map[_Ctype_ino_t]string)

	CheckHeaderExistence(node, node.name) // second argument is only for logs, so is does not matter here what it is
	if node.offset != 58 {
		t.Errorf("Node offset incorrect. Expected=58, received=%d", node.offset)
	}
	expectedHeaders := map[_Ctype_ino_t]string{28: "i-am-a-header"}
	if !reflect.DeepEqual(fi.headers, expectedHeaders) {
		t.Errorf("Headers are incorrect\nExpected=%vReceived=%v", expectedHeaders, fi.headers)
	}

	origFs := getTestFuse(t)
	nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
	nodeSlice[28].offset = 58
	nodeSlice[28].stat.st_size = 26
	nodeSlice[26].stat.st_size -= 86
	nodeSlice[21].stat.st_size -= 86
	nodeSlice[20].stat.st_size -= 86
	nodeSlice[3].stat.st_size -= 86
	nodeSlice[0].stat.st_size -= 86

	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Checking for header changed filesystem incorrectly: %s", err.Error())
	}
}

func TestCheckHeaderExistence_NotEncrypted(t *testing.T) {
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origDownloadData := api.DownloadData
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.DownloadData = origDownloadData
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		return "", 0, fmt.Errorf("something happened")
	}
	api.DownloadData = func(rep api.Repo, nodes []string, path string, header *string, startDecrypted, endDecrypted, oldOffset, fileSize int64) ([]byte, error) {
		if rep != rep1 {
			t.Errorf("api.DownloadData() received incorrect repository. Expected=%v, received=%v", rep1, rep)
		}
		expectedNodes := []string{"bucket_3", "testi"}
		if !reflect.DeepEqual(expectedNodes, nodes) {
			t.Errorf("api.DownloadData() received incorrect nodes.\nExpected=%v\nReceived=%v", expectedNodes, nodes)
		}
		if oldOffset != 0 {
			t.Errorf("api.DownloadData() received incorrect old offset. Expected=0, received=%v", oldOffset)
		}

		content := test.GenerateRandomText(int(fileSize))

		return content[startDecrypted:min(endDecrypted, fileSize)], nil
	}

	node := &nodeSlice[36]
	node.offset = 0
	fi.headers = make(map[_Ctype_ino_t]string)

	CheckHeaderExistence(node, node.name) // second argument is only for logs, so is does not matter here what it is
	if node.offset != 0 {
		t.Errorf("Node offset incorrect. Expected=0, received=%d", node.offset)
	}
	if len(fi.headers) > 0 {
		t.Errorf("Headers slice should be empty. Received=%v", fi.headers)
	}

	origFs := getTestFuse(t)
	nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
	nodeSlice[36].offset = 0

	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Checking for header changed filesystem: %s", err.Error())
	}
}

func TestCheckHeaderExistence_UnknownEncryption(t *testing.T) {
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origDownloadData := api.DownloadData
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.DownloadData = origDownloadData
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		return "", 0, fmt.Errorf("something happened")
	}
	api.DownloadData = func(rep api.Repo, nodes []string, path string, header *string, startDecrypted, endDecrypted, oldOffset, fileSize int64) ([]byte, error) {
		if rep != rep1 {
			t.Errorf("api.DownloadData() received incorrect repository. Expected=%v, received=%v", rep1, rep)
		}
		expectedNodes := []string{"bucket_3", "testi"}
		if !reflect.DeepEqual(expectedNodes, nodes) {
			t.Errorf("api.DownloadData() received incorrect nodes.\nExpected=%v\nReceived=%v", expectedNodes, nodes)
		}
		if oldOffset != 0 {
			t.Errorf("api.DownloadData() received incorrect old offset. Expected=0, received=%v", oldOffset)
		}

		content := test.GenerateRandomText(int(fileSize))
		headerBytes, encryptedContent, _ := test.EncryptData(t, content)
		encryptedContent = append(headerBytes, encryptedContent...)

		return encryptedContent[startDecrypted:min(endDecrypted, fileSize)], nil
	}

	node := &nodeSlice[36]
	node.offset = 0
	fi.headers = make(map[_Ctype_ino_t]string)

	CheckHeaderExistence(node, node.name) // second argument is only for logs, so is does not matter here what it is
	if node.offset != 0 {
		t.Errorf("Node offset incorrect. Expected=0, received=%d", node.offset)
	}
	expectedHeaders := map[_Ctype_ino_t]string{36: ""}
	if !reflect.DeepEqual(expectedHeaders, fi.headers) {
		t.Errorf("Headers incorrect\nExpected=%v\nReceived=%v", expectedHeaders, fi.headers)
	}

	origFs := getTestFuse(t)
	nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
	nodeSlice[36].offset = 0

	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Checking for header changed filesystem: %s", err.Error())
	}
}

func TestCheckHeaderExistence_DownloadError(t *testing.T) {
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origDownloadData := api.DownloadData
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		api.DownloadData = origDownloadData
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		return "", 0, errExpected
	}
	api.DownloadData = func(rep api.Repo, nodes []string, path string, header *string, startDecrypted, endDecrypted, oldOffset, fileSize int64) ([]byte, error) {
		return nil, errExpected
	}

	node := &nodeSlice[34]
	node.offset = 0
	fi.headers = make(map[_Ctype_ino_t]string)

	CheckHeaderExistence(node, node.name) // second argument is only for logs, so is does not matter here what it is
	if node.offset != 0 {
		t.Errorf("Node offset incorrect. Expected=0, received=%d", node.offset)
	}
	if len(fi.headers) > 0 {
		t.Errorf("Headers slice should be empty. Received=%v", fi.headers)
	}

	origFs := getTestFuse(t)
	nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
	nodeSlice[34].offset = 0

	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Checking for header changed filesystem: %s", err.Error())
	}
}

func TestCheckHeaderExistence_TooSmall(t *testing.T) {
	fi.nodes = getTestFuse(t)
	nodeSlice := unsafe.Slice(fi.nodes.nodes, fsSize)

	origGetReencryptedHeader := api.GetReencryptedHeader
	origHeaders := fi.headers
	defer func() {
		api.GetReencryptedHeader = origGetReencryptedHeader
		fi.headers = origHeaders
	}()

	api.GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
		if bucket != "bucket_1" {
			t.Errorf("api.GetReencryptedHeader() received incorrect bucket. Expected=bucket_1, received=%s", bucket)
		}
		if object != "dir+/another_file" {
			t.Errorf("api.GetReencryptedHeader() received incorrect object. Expected=dir+/another_file, received=%s", bucket)
		}

		return "i-am-a-header", 124, nil
	}

	node := &nodeSlice[28]
	node.offset = 0
	fi.headers = make(map[_Ctype_ino_t]string)

	CheckHeaderExistence(node, node.name) // second argument is only for logs, so is does not matter here what it is
	if node.offset != 124 {
		t.Errorf("Node offset incorrect. Expected=124, received=%d", node.offset)
	}
	expectedHeaders := map[_Ctype_ino_t]string{28: "i-am-a-header"}
	if !reflect.DeepEqual(fi.headers, expectedHeaders) {
		t.Errorf("Headers are incorrect\nExpected=%vReceived=%v", expectedHeaders, fi.headers)
	}

	origFs := getTestFuse(t)
	nodeSlice = unsafe.Slice(origFs.nodes, fsSize)
	nodeSlice[28].offset = 124

	if err := isValidFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Errorf("Checking for header changed filesystem: %s", err.Error())
	}
}
