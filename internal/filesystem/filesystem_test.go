package filesystem

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"
	"unsafe"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
)

var errExpected = errors.New("expected error for test")

const rep1 = api.SDConnect
const rep2 = api.SDApply

const fsSize = 35

var testFuse = `{
    "name": "",
    "nameSafe": "",
    "size": 608,
	"modified": "2025-07-31T23:00:05Z",
    "children": [
		{
			"name": "Bad-Repo",
            "nameSafe": "Bad-Repo",
            "size": 0,
			"modified": "1970-01-01T00:00:00Z"
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
                    "children": []
                },
				{
                    "name": "https://example.com",
                    "nameSafe": "https___example.com",
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
            "size": 416,
			"modified": "2020-12-30T10:00:00Z",
            "children": [
				{
					"name": "project",
					"nameSafe": "project",
					"size": 416,
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
											"name": "file_3",
											"nameSafe": "file_3",
											"size": 10,
											"modified": "2010-01-24T18:34:05Z",
											"children": null
										},
										{
											"name": "file_4",
											"nameSafe": "file_4",
											"size": 10,
											"modified": "2020-12-30T10:00:00Z",
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
							"name": "bucket/2",
							"nameSafe": "bucket_2(b0a409)",
							"size": 151,
							"modified": "2000-02-22T05:33:05Z",
							"children": [
								{
									"name": "test",
									"nameSafe": "test",
									"size": 151,
									"modified": "2000-02-22T05:33:05Z",
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
			"children": []
		}
    ]
}`

type jsonNode struct {
	Name     string      `json:"name"`
	NameSafe string      `json:"nameSafe"`
	Size     int64       `json:"size"`
	Modified *time.Time  `json:"modified"`
	Children *[]jsonNode `json:"children"`
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(string, []string) {})
	os.Exit(m.Run())
}

// getTestFuse returns a *Fuse filled in based on variable testFuse
func getTestFuse(t *testing.T) *_Ctype_struct_Nodes {
	var root jsonNode
	if err := json.Unmarshal([]byte(testFuse), &root); err != nil {
		t.Fatalf("Could not unmarshal json: %s", err.Error())
	}

	n := &_Ctype_struct_Nodes{}
	n.count = 0
	n.nodes = allocateNodeList(fsSize)
	t.Cleanup(func() { freeNodes(n) })

	assignChildren(n, []jsonNode{root}, nil)

	return n
}

func assignChildren(n *_Ctype_struct_Nodes, template []jsonNode, parent *_Ctype_struct_Node) {
	idx := int(n.count)
	n.count += _Ctype_int64_t(len(template))

	for i, bucket := range template {
		meta := api.Metadata{
			Name:         bucket.Name,
			Size:         bucket.Size,
			LastModified: bucket.Modified,
		}

		nodeSlice := unsafe.Slice(n.nodes, fsSize)
		nodeSlice[idx+i] = goNodeToC(newGoNode(meta, bucket.Children != nil), bucket.NameSafe)
		nodeSlice[idx+i].stat.st_ino = _Ctype_ino_t(idx + i)
		nodeSlice[idx+i].parent = parent

		if bucket.Children == nil || len(*bucket.Children) == 0 {
			continue
		}

		nodeSlice[idx+i].children = &nodeSlice[n.count]
		nodeSlice[idx+i].chld_count = _Ctype_int64_t(len(*bucket.Children))
		assignChildren(n, *bucket.Children, &nodeSlice[idx+i])
	}

	return
}

func isSameFuse(fs1 *_Ctype_struct_Node, fs2 *_Ctype_struct_Node, path string) error {
	if fs2.stat.st_size != fs1.stat.st_size {
		return fmt.Errorf("size not correct at node %q. Expected=%d, received=%d", path, fs1.stat.st_size, fs2.stat.st_size)
	}
	if fs1.stat.st_ino != fs2.stat.st_ino {
		return fmt.Errorf("ino not correct at node %q. Expected=%d, received=%d", path, fs1.stat.st_ino, fs2.stat.st_ino)
	}
	if toGoStr(fs2.orig_name) != toGoStr(fs1.orig_name) {
		return fmt.Errorf("original name not correct at node %q. Expected=%s, received=%s", path, toGoStr(fs1.orig_name), toGoStr(fs2.orig_name))
	}
	if fs1.last_modified != fs2.last_modified {
		return fmt.Errorf("timestamp not correct at node %q, Expected=%+v, received=%+v", path, fs1.last_modified, fs2.last_modified)
	}
	if fs1.children == nil && fs2.children != nil {
		return fmt.Errorf("node %q should not have children", path)
	}
	if fs1.children != nil && fs2.children == nil {
		return fmt.Errorf("node %q should have children", path)
	}
	if fs1.children == nil {
		return nil
	}

	// Names of children of fs1 and fs2
	keys1 := make([]string, fs1.chld_count)
	keys2 := make([]string, fs2.chld_count)

	slice1 := unsafe.Slice(fs1.children, fs1.chld_count)
	for i := range slice1 {
		keys1[i] = toGoStr(slice1[i].name)
	}

	slice2 := unsafe.Slice(fs2.children, fs2.chld_count)
	for i := range slice2 {
		keys2[i] = toGoStr(slice2[i].name)
	}

	if !reflect.DeepEqual(keys1, keys2) {
		return fmt.Errorf("children differ at node %q\nExpected=%v\nReceived=%v", path, keys1, keys2)
	}

	for i := range keys1 {
		if err := isSameFuse(&slice1[i], &slice2[i], path+"/"+keys1[i]); err != nil {
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

func TestInitializeFilesystem(t *testing.T) {
	origFs := getTestFuse(t)

	origGetRepositories := api.GetRepositories
	origGetProjectName := api.GetProjectName
	origGetBuckets := api.GetBuckets
	origGetObjects := api.GetObjects
	origGetSegmentedObjects := api.GetSegmentedObjects
	origGetHeaders := api.GetHeaders
	origHeaders := fi.headers
	defer func() {
		api.GetRepositories = origGetRepositories
		api.GetProjectName = origGetProjectName
		api.GetBuckets = origGetBuckets
		api.GetObjects = origGetObjects
		api.GetSegmentedObjects = origGetSegmentedObjects
		api.GetHeaders = origGetHeaders
		fi.headers = origHeaders
	}()

	api.GetRepositories = func() []string {
		return []string{api.SDConnect, api.SDApply, "Bad-Repo", "Substandard-Repo"}
	}
	api.GetProjectName = func() string {
		return "project"
	}
	api.GetBuckets = func(rep string) ([]api.Metadata, error) {
		if rep == rep1 {
			return []api.Metadata{
				{Name: "bucket_1_segments"},
				{Name: "bucket_1"},
				{Name: "bucket_2_segments"},
				{Name: "bucket/2"},
				{Name: "bucket_2"},
			}, nil
		} else if rep == rep2 {
			return []api.Metadata{
				{Name: "https://example.com"},
				{Name: "bad-bucket"},
				{Name: "bad-bucket_segments"},
				{Name: "old-bucket"},
				{Name: "old-bucket_segments"},
			}, nil
		} else if rep == "Substandard-Repo" {
			return nil, nil
		}

		return nil, fmt.Errorf("api.GetBuckets() received invalid repository %q", rep)
	}
	api.GetObjects = func(rep, bucket, path string, prefix ...string) ([]api.Metadata, error) {
		if rep == rep1 {
			switch bucket {
			case "bucket_1":
				time1, _ := time.Parse(time.RFC3339, "2020-12-30T10:00:00Z")
				time2, _ := time.Parse(time.RFC3339, "2010-01-24T18:34:05Z")
				time3, _ := time.Parse(time.RFC3339, "2016-11-02T15:04:05Z")
				time4, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
				time5, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")

				return []api.Metadata{
					{Size: 10, Name: "kansio/file_4", LastModified: &time1},
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
			case "bucket/2":
				time1, _ := time.Parse(time.RFC3339, "2000-02-22T05:33:05Z")

				return []api.Metadata{
					{Size: 151, Name: "test", LastModified: &time1},
				}, nil
			default:
				return nil, fmt.Errorf("api.GetObjects() received invalid project %s", rep+"/"+bucket)
			}
		} else if rep == rep2 {
			switch bucket {
			case "https://example.com":
				time1, _ := time.Parse(time.RFC3339, "2025-07-31T23:00:05Z")

				return []api.Metadata{{Size: 5, Name: "tiedosto", LastModified: &time1}}, nil
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

			return nil, fmt.Errorf("api.GetObjects() received invalid bucket %s", rep+"/"+bucket)
		}

		return nil, fmt.Errorf("api.GetObjects() received invalid repository %s", rep)
	}
	api.GetSegmentedObjects = func(rep, bucket string) ([]api.Metadata, error) {
		if rep == rep1 {
			switch bucket {
			case "bucket_1_segments":
				return []api.Metadata{
					{Size: 112, Name: "dir+/another_file/fyvutilbiyni/00000001", LastModified: nil},
				}, nil
			}

			return nil, fmt.Errorf("api.GetObjects() received invalid bucket %s", rep+"/"+bucket)
		} else if rep == rep2 {
			switch bucket {
			case "https://example.com_segments":
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

			return nil, fmt.Errorf("api.GetObjects() received invalid bucket %s", rep+"/"+bucket)
		}

		return nil, fmt.Errorf("api.GetObjects() received invalid repository %s", rep)
	}
	api.GetHeaders = func(rep string, buckets []api.Metadata) (api.BatchHeaders, error) {
		if rep == rep1 {
			expectedBuckets := []string{"bucket/2", "bucket_1", "bucket_2"}
			bucketsCopy := slices.Clone(buckets)
			sort.Slice(bucketsCopy, func(i, j int) bool { return buckets[i].Name < buckets[j].Name })
			result := slices.CompareFunc(bucketsCopy, expectedBuckets, func(b1 api.Metadata, b2 string) int {
				return cmp.Compare(b1.Name, b2)
			})
			if result != 0 {
				t.Errorf("api.GetHeaders() received invalid buckets\nExpected=%v\nReceived=%v", expectedBuckets, bucketsCopy)
			}

			batch := make(api.BatchHeaders)
			batch["bucket_1"] = make(map[string]api.VaultHeaderVersions)
			batch["bucket_2"] = make(map[string]api.VaultHeaderVersions)

			batch["bucket_1"]["kansio/file_2"] = api.VaultHeaderVersions{
				Headers: map[string]api.VaultHeader{
					"1": {Header: "fyvltvylbui"},
					"2": {Header: "vlfvyugyvli"},
				},
				LatestVersion: 2,
			}
			batch["bucket_1"]["kansio/file_3"] = api.VaultHeaderVersions{
				Headers: map[string]api.VaultHeader{
					"3": {Header: "hbfyucdtkyv"},
				},
				LatestVersion: 3,
			}
			batch["bucket_2"]["?folder/test"] = api.VaultHeaderVersions{
				Headers: map[string]api.VaultHeader{
					"2": {Header: "vhvfkutcflu"},
					"3": {Header: "fyblfyvigyi"},
					"4": {Header: "fdctkvfulbf"},
					"5": {Header: "bftcdvtuftu"},
				},
				LatestVersion: 5,
			}

			return batch, nil
		} else if rep == "Substandard-Repo" {
			return nil, fmt.Errorf("api.GetHeaders() received invalid repository %sv", rep)
		}

		return nil, nil
	}

	fi.nodes = &_Ctype_struct_Nodes{}
	InitialiseFilesystem()
	if fi.nodes == origFs {
		t.Fatal("Global nodes should not point to test nodes")
	}
	t.Cleanup(func() { freeNodes(fi.nodes); fi.nodes.count = 0 })
	if origFs.count != fi.nodes.count {
		t.Fatalf("Node count incorrect. Expected=%v, received=%v", origFs.count, fi.nodes.count)
	}
	if err := isSameFuse(origFs.nodes, fi.nodes.nodes, ""); err != nil {
		t.Fatalf("FUSE was not created correctly: %s", err.Error())
	}
	expectedHeaders := map[_Ctype_ino_t]string{28: "vlfvyugyvli", 29: "hbfyucdtkyv", 33: "bftcdvtuftu"}
	if !reflect.DeepEqual(expectedHeaders, fi.headers) {
		t.Fatalf("Headers incorrect\nExpected=%v\nReceived=%v", expectedHeaders, fi.headers)
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
