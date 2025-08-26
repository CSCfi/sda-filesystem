package airlock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"sda-filesystem/test"

	"github.com/neicnordic/crypt4gh/keys"
	"github.com/neicnordic/crypt4gh/streaming"
)

var errExpected = errors.New("expected error for test")

func TestMain(m *testing.M) {
	logs.SetSignal(func(string, []string) {})
	os.Exit(m.Run())
}

func TestExportPossible(t *testing.T) {
	var tests = []struct {
		testname                   string
		connect, manager, possible bool
	}{
		{"OK_1", true, true, true},
		{"OK_2", false, true, false},
		{"OK_3", true, false, false},
	}

	origSDConnectEnabled := api.SDConnectEnabled
	origIsProjectManager := api.IsProjectManager
	defer func() {
		api.SDConnectEnabled = origSDConnectEnabled
		api.IsProjectManager = origIsProjectManager
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.SDConnectEnabled = func() bool {
				return tt.connect
			}
			api.IsProjectManager = func() bool {
				return tt.manager
			}

			possible := ExportPossible()
			if possible != tt.possible {
				t.Errorf("Function returned incorrect boolean value\nExpected=%t\nReceived=%t", tt.possible, possible)
			}
		})
	}
}

func TestWalkDirs(t *testing.T) {
	tmpDir := t.TempDir()

	var tests = []struct {
		testname, prefix, expectedBucket                   string
		selection, objects, expectedFiles, expectedObjects []string
	}{
		{
			"OK_1", "bucket/subfolder", "bucket",
			[]string{tmpDir + "/file.txt", tmpDir + "/dir/../dir/"},
			[]string{"old-file.txt.c4gh", "hello.txt.c4gh", "another-file.txt.c4gh"},
			[]string{
				tmpDir + "/file.txt", tmpDir + "/dir/file2.txt", tmpDir + "/dir/subdir/file.txt",
				tmpDir + "/dir/subdir/another-file.txt", tmpDir + "/dir/subdir2/event.log",
				tmpDir + "/dir/subdir2/fatal.log",
			},
			[]string{
				"subfolder/file.txt.c4gh", "subfolder/dir/file2.txt.c4gh", "subfolder/dir/subdir/file.txt.c4gh",
				"subfolder/dir/subdir/another-file.txt.c4gh", "subfolder/dir/subdir2/event.log.c4gh",
				"subfolder/dir/subdir2/fatal.log.c4gh",
			},
		},
		{
			"OK_2", "test-bucket", "test-bucket",
			[]string{tmpDir + "/file.txt", tmpDir + "/run.sh", tmpDir + "/dir/subdir2"}, nil,
			[]string{
				tmpDir + "/file.txt", tmpDir + "/run.sh",
				tmpDir + "/dir/subdir2/event.log", tmpDir + "/dir/subdir2/fatal.log",
			},
			[]string{"file.txt.c4gh", "run.sh.c4gh", "subdir2/event.log.c4gh", "subdir2/fatal.log.c4gh"},
		},
	}

	if err := os.MkdirAll(tmpDir+"/dir/subdir", 0755); err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	if err := os.MkdirAll(tmpDir+"/dir/subdir2", 0755); err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}

	content := []byte("hello world\n")
	err := os.WriteFile(tmpDir+"/file.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/run.sh", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/file2.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir/file.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir/another-file.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir2/event.log", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir2/fatal.log", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.Symlink(tmpDir+"/file.txt", tmpDir+"/file-link.txt")
	if err != nil {
		t.Fatalf("Failed to create symlink: %s", err.Error())
	}
	err = os.Symlink(tmpDir+"/file.txt", tmpDir+"/dir/subdir/file-link.txt")
	if err != nil {
		t.Fatalf("Failed to create symlink: %s", err.Error())
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			set, err := WalkDirs(tt.selection, tt.objects, tt.prefix)

			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else {
				slices.Sort(set.Objects)
				slices.Sort(set.Files)
				slices.Sort(tt.expectedFiles)
				slices.Sort(tt.expectedObjects)

				switch {
				case set.Bucket != tt.expectedBucket:
					t.Errorf("Received incorrect bucket\nExpected=%s\nReceived=%s", tt.expectedBucket, set.Bucket)
				case !reflect.DeepEqual(set.Files, tt.expectedFiles):
					t.Errorf("Received incorrect files\nExpected=%q\nReceived=%q", tt.expectedFiles, set.Files)
				case !reflect.DeepEqual(set.Objects, tt.expectedObjects):
					t.Errorf("Received incorrect objects\nExpected=%q\nReceived=%q", tt.expectedObjects, set.Objects)
				}
			}
		})
	}
}

func TestWalkDirs_Error(t *testing.T) {
	tmpDir := t.TempDir()

	var tests = []struct {
		testname, errStr string
		selection        []string
	}{
		{
			"FAIL_1",
			"objects derived from the selection of files are not unique",
			[]string{tmpDir + "/file.txt", tmpDir + "/file.txt"},
		},
		{
			"FAIL_2",
			"you have already selected files with similar object names",
			[]string{tmpDir + "/dir/subdir2"},
		},
		{
			"FAIL_3",
			"lstat " + tmpDir + "/dir2/file.txt: no such file or directory",
			[]string{tmpDir + "/file.txt", tmpDir + "/dir/subdir", tmpDir + "/dir2/file.txt"},
		},
	}

	if err := os.MkdirAll(tmpDir+"/dir/subdir", 0755); err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	if err := os.MkdirAll(tmpDir+"/dir/subdir2", 0755); err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}

	content := []byte("hello world\n")
	err := os.WriteFile(tmpDir+"/file.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/run.sh", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/file2.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir/file.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir/another-file.txt", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir2/event.log", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.WriteFile(tmpDir+"/dir/subdir2/fatal.log", content, 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	err = os.Symlink(tmpDir+"/file.txt", tmpDir+"/file-link.txt")
	if err != nil {
		t.Fatalf("Failed to create symlink: %s", err.Error())
	}
	err = os.Symlink(tmpDir+"/file.txt", tmpDir+"/dir/subdir/file-link.txt")
	if err != nil {
		t.Fatalf("Failed to create symlink: %s", err.Error())
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			existingObjects := []string{"old-file.txt.c4gh", "dir/subdir2/fatal.log.c4gh", "another-file.txt.c4gh"}
			_, err := WalkDirs(tt.selection, existingObjects, "test-bucket/dir")

			if err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestValidateBucket(t *testing.T) {
	var tests = []struct {
		testname, bucket string
		createBucket     bool
	}{
		{"OK_1", "i-am-a-bucket-that-has-a-long-long-long-name", true},
		{"OK_2", "42istheanswertoeverything", false},
		{"OK_3", "h3ll0-w0r1d", true},
	}

	origBucketExists := api.BucketExists
	origCreateBucket := api.CreateBucket
	defer func() {
		api.BucketExists = origBucketExists
		api.CreateBucket = origCreateBucket
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.BucketExists = func(rep, bucket string) (bool, error) {
				if rep != api.SDConnect {
					t.Errorf("api.BucketExists() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != tt.bucket {
					t.Errorf("api.BucketExists() received incorrect bucket. Expected=%s, received=%s", tt.bucket, bucket)
				}

				return !tt.createBucket, nil
			}
			createBucketCalled := false
			api.CreateBucket = func(rep, bucket string) error {
				if rep != api.SDConnect {
					t.Errorf("api.CreateBucket() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != tt.bucket {
					t.Errorf("api.CreateBucket() received incorrect bucket. Expected=%s, received=%s", tt.bucket, bucket)
				}
				if !tt.createBucket {
					t.Errorf("api.CreateBucket() should not have been called for bucket %s", bucket)
				}
				createBucketCalled = true

				return nil
			}

			created, err := ValidateBucket(tt.bucket)
			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			}
			if created != tt.createBucket {
				t.Errorf("Function returned incorrect boolean value for bucket %s\nExpected=%t\nReceived=%t", tt.bucket, tt.createBucket, created)
			}
			if tt.createBucket && !createBucketCalled {
				t.Errorf("api.CreateBucket() was not called for bucket %s", tt.bucket)
			}
		})
	}
}

func TestValidateBucket_Error(t *testing.T) {
	var tests = []struct {
		testname, bucket, errStr string
		createBucket             bool
		existsErr, createErr     error
	}{
		{
			"FAIL_1",
			"i-am-a-bucket-that-has-a-long-long-long-long-long-long-long-name",
			"bucket name should be between 3 and 63 characters long",
			false, nil, nil,
		},
		{
			"FAIL_2", "hi",
			"bucket name should be between 3 and 63 characters long",
			false, nil, nil,
		},
		{
			"FAIL_3", "no spaces allowed",
			"bucket name should only contain Latin letters (a-z), numbers (0-9) and hyphens (-)",
			false, nil, nil,
		},
		{
			"FAIL_4", "-bucket",
			"bucket name should start with a lowercase letter or a number",
			false, nil, nil,
		},
		{
			"FAIL_5", "test-bucket",
			errExpected.Error(), false, errExpected, nil,
		},
		{
			"FAIL_6", "test-bucket",
			errExpected.Error(), true, nil, errExpected,
		},
	}

	origBucketExists := api.BucketExists
	origCreateBucket := api.CreateBucket
	defer func() {
		api.BucketExists = origBucketExists
		api.CreateBucket = origCreateBucket
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.BucketExists = func(rep, bucket string) (bool, error) {
				return !tt.createBucket, tt.existsErr
			}
			api.CreateBucket = func(rep, bucket string) error {
				return tt.createErr
			}

			_, err := ValidateBucket(tt.bucket)
			if err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestGetFileDetails(t *testing.T) {
	var tests = []struct {
		testname, message string
		bytes             int64
	}{
		{"OK_1", "pipe message", 164},
		{"OK_2", "another_message", 167},
	}

	tmpDir := t.TempDir()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			filename := tmpDir + "/file.txt"
			err := os.WriteFile(filename, []byte(tt.message), 0600)
			if err != nil {
				t.Fatalf("Failed to create file: %s", err.Error())
			}

			rc, bytes, err := getFileDetails(filename)
			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			}
			if bytes != tt.bytes {
				t.Errorf("Function returned incorrect bytes. Expected=%d, received=%d", tt.bytes, bytes)
			}
			if rc == nil {
				t.Error("Function returned nil file pointer")
			} else {
				if message, err := io.ReadAll(rc); err != nil {
					t.Errorf("Failed to read from file: %s", err.Error())
				} else if tt.message != string(message) {
					t.Errorf("Received incorrect message\nExpected=%s\nReceived=%s", tt.message, string(message))
				}
				if err = rc.Close(); err != nil {
					t.Errorf("Closing file caused error: %s", err.Error())
				}
			}
		})
	}
}

func TestFileDetails_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	errStr := fmt.Sprintf("open %s/file.txt: no such file or directory", tmpDir)
	if _, _, err := getFileDetails(tmpDir + "/file.txt"); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestCheckObjectExistences_UserInput(t *testing.T) {
	var tests = []struct {
		testname, userInput, errStr string
		inputObjects                []string
		existingObjects             []api.Metadata
	}{
		{
			"OK_1", "", "",
			[]string{
				"subfolder/another-dir/file.txt.c4gh",
				"file.out.c4gh",
			},
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-object"},
			},
		},
		{
			"OK_2", "yes\n", "",
			[]string{
				"subfolder/another-dir/file.txt.c4gh",
			},
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-dir/file.txt.c4gh"},
				{Name: "subfolder/another-object"},
				{Name: "subfolder/dir/file.txt"},
			},
		},
		{
			"OK_3", "y\nno\n", "",
			[]string{
				"subfolder/another-dir/file.txt.c4gh",
				"some-object",
			},
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-dir/file.txt.c4gh"},
				{Name: "subfolder/another-object"},
			},
		},
		{
			"FAIL_NO", "ye\nno\n", "not permitted to override data",
			[]string{
				"some-object",
			},
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-dir/file.txt.c4gh"},
			},
		},
		{
			"FAIL_INPUT", "blaa", "failed to read user input: EOF",
			[]string{
				"subfolder/another-dir/file.txt.c4gh",
			},
			[]api.Metadata{
				{Name: "subfolder/another-dir/file.txt.c4gh"},
			},
		},
	}

	origBucketExists := api.BucketExists
	origGetObjects := api.GetObjects
	defer func() {
		api.BucketExists = origBucketExists
		api.GetObjects = origGetObjects
	}()

	api.BucketExists = func(rep, bucket string) (bool, error) {
		if rep != api.SDConnect {
			t.Errorf("api.BucketExists() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
		}
		if bucket != "bucket" {
			t.Errorf("api.BucketExists() received incorrect bucket. Expected=bucket, received=%s", bucket)
		}

		return true, nil
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetObjects = func(rep, bucket, path string, prefix ...string) ([]api.Metadata, error) {
				if rep != api.SDConnect {
					t.Errorf("api.GetObjects() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != "bucket" {
					t.Errorf("api.GetObjects() received incorrect bucket. Expected=bucket, received=%s", bucket)
				}
				if len(prefix) > 0 {
					t.Errorf("api.GetObjects() should not have received prefix")
				}

				return tt.existingObjects, nil
			}

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			set := UploadSet{Bucket: "bucket", Objects: tt.inputObjects, Exists: make([]bool, len(tt.inputObjects))}
			err := CheckObjectExistences(&set, strings.NewReader(tt.userInput))

			os.Stdout = sout
			null.Close()

			switch {
			case tt.errStr != "":
				if err == nil {
					t.Errorf("Function did not return error")
				} else if err.Error() != tt.errStr {
					t.Errorf("Function returned incorrect error\nExpected=%q\nReceived=%q", tt.errStr, err.Error())
				}
			case err != nil:
				t.Errorf("Function returned unexpected error: %s", err.Error())
			}
		})
	}
}

func TestCheckObjectExistences_NilReader(t *testing.T) {
	var tests = []struct {
		testname        string
		inputObjects    []string
		existingObjects []api.Metadata
		expectedExists  []bool
	}{
		{
			"OK_1",
			[]string{
				"subfolder/another-dir/file.txt.c4gh",
				"file.out.c4gh",
			},
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-object"},
			},
			[]bool{false, false},
		},
		{
			"OK_2",
			[]string{
				"subfolder/another-dir/file.txt.c4gh",
			},
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-dir/file.txt.c4gh"},
				{Name: "subfolder/another-object"},
				{Name: "subfolder/dir/file.txt"},
			},
			[]bool{true},
		},
		{
			"OK_3",
			[]string{
				"subfolder/another-dir/file.txt.c4gh",
				"some-object",
				"file.txt",
			},
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-dir/file.txt.c4gh"},
				{Name: "subfolder/another-object"},
			},
			[]bool{true, true, false},
		},
	}

	origBucketExists := api.BucketExists
	origGetObjects := api.GetObjects
	defer func() {
		api.BucketExists = origBucketExists
		api.GetObjects = origGetObjects
	}()

	api.BucketExists = func(rep, bucket string) (bool, error) {
		if rep != api.SDConnect {
			t.Errorf("api.BucketExists() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
		}
		if bucket != "bucket" {
			t.Errorf("api.BucketExists() received incorrect bucket. Expected=bucket, received=%s", bucket)
		}

		return true, nil
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetObjects = func(rep, bucket, path string, prefix ...string) ([]api.Metadata, error) {
				if rep != api.SDConnect {
					t.Errorf("api.GetObjects() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != "bucket" {
					t.Errorf("api.GetObjects() received incorrect bucket. Expected=bucket, received=%s", bucket)
				}
				if len(prefix) > 0 {
					t.Errorf("api.GetObjects() should not have received prefix")
				}

				return tt.existingObjects, nil
			}

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			set := UploadSet{Bucket: "bucket", Objects: tt.inputObjects, Exists: make([]bool, len(tt.inputObjects))}
			err := CheckObjectExistences(&set, nil)

			os.Stdout = sout
			null.Close()

			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedExists, set.Exists) {
				t.Errorf("Set has incorrect values for 'exists' field\nExpected=%v\nReceived=%v", tt.expectedExists, set.Exists)
			}
		})
	}
}

func TestCheckObjectExistence_Error(t *testing.T) {
	origBucketExists := api.BucketExists
	origGetObjects := api.GetObjects
	defer func() {
		api.BucketExists = origBucketExists
		api.GetObjects = origGetObjects
	}()

	api.GetObjects = func(rep, bucket, path string, prefix ...string) ([]api.Metadata, error) {
		return nil, errExpected
	}

	errStr := "could not determine if export will overwrite data: " + errExpected.Error()
	set := UploadSet{Bucket: "bucket", Objects: []string{"file.txt.c4gh"}, Exists: make([]bool, 1)}
	err := CheckObjectExistences(&set, strings.NewReader(""))
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUpload(t *testing.T) {
	var tests = []struct {
		testname, projectType, bucket string
		files, objects                []string
		createBucket                  bool
		fileSize, segmentSize         int64
		metadata                      map[string]string
	}{
		{
			"OK_1", "default", "test-bucket",
			[]string{"test-file.txt"}, []string{"test-file.txt.c4gh"},
			false, 9463686, 1 << 27, nil,
		},
		{
			"OK_2", "default", "test-bucket",
			[]string{"test-file.txt", "subfolder/test-file.txt"},
			[]string{"test-file.txt.c4gh", "subfolder/test-file.txt.c4gh"},
			true, (1 << 40) * 1.5, 1 << 28, nil,
		},
		{
			"OK_3", "findata", "test-bucket",
			[]string{"test-file.txt"}, []string{"test-file.txt.c4gh"},
			false, 9463686, 1 << 27, map[string]string{"journal_number": "journal890"},
		},
		{
			"OK_4", "findata", "test-bucket",
			[]string{"test-file.txt", "subfolder/test-file.txt", "another-file.txt"},
			[]string{"test-file.txt.c4gh", "subfolder/test-file.txt.c4gh", "another-file.txt.c4gh"},
			true, (1 << 40) * 1.5, 1 << 28,
			map[string]string{"journal_number": "journal456", "author_email": "some.address@gmail.com"},
		},
	}

	origGetPublicKey := api.GetPublicKey
	origGetFileDetails := getFileDetails
	origProjectName := api.GetProjectName
	origGetProjectType := api.GetProjectType
	origPostHeader := api.PostHeader
	origUploadObject := api.UploadObject
	origDeleteObject := api.DeleteObject
	origPublicKey := ai.publicKey
	defer func() {
		api.GetPublicKey = origGetPublicKey
		getFileDetails = origGetFileDetails
		api.GetProjectName = origProjectName
		api.GetProjectType = origGetProjectType
		api.PostHeader = origPostHeader
		api.UploadObject = origUploadObject
		api.DeleteObject = origDeleteObject
		ai.publicKey = origPublicKey
	}()

	api.GetProjectName = func() string {
		return "findata-project"
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			publicKey, privateKey, err := keys.GenerateKeyPair()
			if err != nil {
				t.Fatalf("Could not generate key pair: %s", err.Error())
			}
			var content sync.Map
			var receivedContent sync.Map
			for i := range tt.objects {
				content.Store(tt.objects[i], test.GenerateRandomText(100))
				receivedContent.Store(tt.objects[i], make([]byte, 0))
			}

			api.GetPublicKey = func() ([32]byte, error) {
				return publicKey, nil
			}
			api.GetProjectType = func() string {
				return tt.projectType
			}
			getFileDetails = func(filename string) (io.ReadCloser, int64, error) {
				idx := slices.Index(tt.files, filename)
				if idx < 0 {
					return nil, 0, fmt.Errorf("getFileDetails() received invalid file name %s, expectd to be one of %v", filename, tt.files)
				}
				obj := tt.objects[idx]
				value, _ := content.Load(obj)
				rc := io.NopCloser(bytes.NewReader(value.([]byte)))

				return rc, tt.fileSize, nil
			}
			api.PostHeader = func(header []byte, bucket, object string) error {
				if bucket != "test-bucket" {
					t.Errorf("api.PostHeader() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
				}
				if !slices.Contains(tt.objects, object) {
					t.Errorf("api.PostHeader() received incorrect object %s, expected to be one of %q", object, tt.objects)
				}
				value, _ := receivedContent.Load(object)
				receivedContent.Store(object, append(value.([]byte), header...))

				return nil
			}
			//nolint:nestif
			if tt.projectType == "default" {
				api.UploadObject = func(ctx context.Context, body io.Reader, rep, bucket, object string, segmentSize int64, metadata map[string]string) error {
					if rep != api.SDConnect {
						t.Errorf("api.UploadObject() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
					}
					if bucket != "test-bucket" {
						t.Errorf("api.UploadObject() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
					}
					if !slices.Contains(tt.objects, object) {
						t.Errorf("api.UploadObject() received incorrect object %s, expected to be one of %q", object, tt.objects)
					}
					if segmentSize != tt.segmentSize {
						t.Errorf("api.UploadObject() received incorrect segment size. Expected=%d, received=%d", tt.segmentSize, segmentSize)
					}

					bodyBytes, err := io.ReadAll(body)
					if err != nil {
						return fmt.Errorf("failed to read file body: %w", err)
					}
					value, _ := receivedContent.Load(object)
					receivedObjContent := append(value.([]byte), bodyBytes...)

					value, _ = content.Load(object)
					expectedObjContent := string(value.([]byte))

					c4ghr, err := streaming.NewCrypt4GHReader(bytes.NewReader(receivedObjContent), privateKey, nil)
					if err != nil {
						t.Errorf("Failed to create crypt4gh reader: %s", err.Error())
					} else if message, err := io.ReadAll(c4ghr); err != nil {
						t.Errorf("Failed to read from encrypted file: %s", err.Error())
					} else if string(message) != expectedObjContent {
						t.Errorf("Reader received incorrect message for object %s\nExpected=%s\nReceived=%s", object, expectedObjContent, string(message))
					}

					return nil
				}
			} else {
				api.UploadObject = func(ctx context.Context, body io.Reader, rep, bucket, object string, segmentSize int64, metadata map[string]string) error {
					if rep != api.SDConnect && rep != api.Findata {
						t.Fatalf("api.UploadObject() received incorrect repository %s", rep)
					}
					if bucket != "test-bucket" {
						t.Errorf("api.UploadObject() received incorrect %s bucket. Expected=test-bucket, received=%s", api.SDConnect, bucket)
					}
					if !slices.Contains(tt.objects, object) {
						t.Errorf("api.UploadObject() received incorrect object %s, expected to be one of %q", object, tt.objects)
					}
					if segmentSize != tt.segmentSize {
						t.Errorf("api.UploadObject() received incorrect segment size. Expected=%d, received=%d", tt.segmentSize, segmentSize)
					}

					bodyBytes, err := io.ReadAll(body)
					if err != nil {
						return fmt.Errorf("failed to read file body: %s", err.Error())
					}

					if rep == api.Findata {
						if !reflect.DeepEqual(metadata, tt.metadata) {
							t.Errorf("api.UploadObject() received incorrect metadata\nExpected=%v\nReceived=%v", tt.metadata, metadata)
						}

						origObject := strings.TrimPrefix(object, "test-bucket/")
						value, _ := content.Load(origObject)
						expectedObjContent := string(value.([]byte))
						if string(bodyBytes) != expectedObjContent {
							t.Fatalf("findata reader returned incorrect body for object %s.\nExpected=%s\nReceived=%s", object, expectedObjContent, string(bodyBytes))
						}

						return nil
					}

					value, _ := receivedContent.Load(object)
					receivedObjContent := append(value.([]byte), bodyBytes...)

					value, _ = content.Load(object)
					expectedObjContent := string(value.([]byte))

					c4ghr, err := streaming.NewCrypt4GHReader(bytes.NewReader(receivedObjContent), privateKey, nil)
					if err != nil {
						t.Fatalf("failed to create crypt4gh reader: %s", err.Error())
					} else if message, err := io.ReadAll(c4ghr); err != nil {
						t.Errorf("Failed to read from encrypted file: %s", err.Error())
					} else if string(message) != expectedObjContent {
						t.Errorf("Reader received incorrect message for object %s\nExpected=%s\nReceived=%s", object, expectedObjContent, string(message))
					}

					return nil
				}
			}
			api.DeleteObject = func(rep, bucket, object string) error {
				t.Error("Should not call api.DeleteObject()")

				return nil
			}

			set := UploadSet{
				Bucket:  tt.bucket,
				Files:   tt.files,
				Objects: tt.objects,
			}
			if err := Upload(set, tt.metadata); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			}
		})
	}
}

func TestUpload_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr, projectType string
		fileSize                      int64
		header                        bool
		keyErr, uploadErr             error
	}{
		{
			"FAIL_KEY", "failed to get project public key: " + errExpected.Error(), "default",
			500, false, errExpected, nil,
		},
		{
			"FAIL_UPLOAD", "upload interrupted due to errors", "default",
			456, true, nil, errExpected,
		},
	}

	origGetPublicKey := api.GetPublicKey
	origGetFileDetails := getFileDetails
	origGetProjectType := api.GetProjectType
	origUploadAllas := uploadAllas
	origDeleteObject := api.DeleteObject
	origPublicKey := ai.publicKey
	origError := logs.Error
	origErrorf := logs.Errorf
	defer func() {
		api.GetPublicKey = origGetPublicKey
		getFileDetails = origGetFileDetails
		api.GetProjectType = origGetProjectType
		uploadAllas = origUploadAllas
		api.DeleteObject = origDeleteObject
		ai.publicKey = origPublicKey
		logs.Error = origError
		logs.Errorf = origErrorf
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			publicKey, _, err := keys.GenerateKeyPair()
			if err != nil {
				t.Fatalf("Could not generate key pair: %s", err.Error())
			}

			api.GetPublicKey = func() ([32]byte, error) {
				return publicKey, tt.keyErr
			}
			getFileDetails = func(filename string) (io.ReadCloser, int64, error) {
				rc := io.NopCloser(strings.NewReader("I am content"))

				return rc, 100, nil
			}
			api.GetProjectType = func() string {
				return "default"
			}
			uploadAllas = func(ctx context.Context, pr io.Reader, bucket, object string, segmentSize int64) error {
				if tt.uploadErr != nil {
					if object == "subfolder/test-file.txt.c4gh" {
						time.Sleep(10 * time.Millisecond) // We have to be sure other goroutines have called the function
						_, cancel := context.WithCancel(ctx)
						cancel()
					}
					time.Sleep(20 * time.Millisecond) // Make sure cancel() has been called
				}
				_, _ = io.ReadAll(pr)

				return tt.uploadErr
			}
			api.DeleteObject = func(rep, bucket, object string) error {
				return nil
			}

			errs := make([]string, 0)
			errc := make(chan error)
			wait := make(chan any)

			go func() {
				for e := range errc {
					errs = append(errs, e.Error())
				}

				wait <- nil
			}()

			logs.Error = func(err error) {
				errc <- err
			}
			logs.Errorf = func(format string, args ...any) {
				errc <- fmt.Errorf(format, args...)
			}

			set := UploadSet{
				Bucket: "test-bucket",
				Files: []string{
					"test-file.txt",
					"subfolder/test-file.txt",
					"another-file.txt",
					"subfolder/another-file.txt",
					"dir/cover.out",
					"script.py",
					"run.sh",
					"log.txt",
				},
				Objects: []string{
					"test-file.txt.c4gh",
					"subfolder/test-file.txt.c4gh",
					"another-file.txt.c4gh",
					"subfolder/another-file.txt.c4gh",
					"dir/cover.out.c4gh",
					"script.py.c4gh",
					"run.sh.c4gh",
					"log.txt.c4gh",
				},
			}
			if err = Upload(set, nil); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
			close(errc)
			<-wait

			expectedLogs := []string{
				"uploading file test-file.txt failed",
				"uploading file subfolder/test-file.txt failed",
				"uploading file another-file.txt failed",
				"uploading file subfolder/another-file.txt failed",
			}
			expectedLogs = append(expectedLogs, slices.Repeat([]string{errExpected.Error()}, numRoutines)...)
			slices.Sort(expectedLogs)
			slices.Sort(errs)

			if tt.uploadErr != nil && !reflect.DeepEqual(expectedLogs, errs) {
				t.Errorf("Function logged incorrect errors\nExpected=%q\nReceived=%q", expectedLogs, errs)
			}
		})
	}
}

type badReader struct {
	buf       *bytes.Buffer
	readSoFar int
	limit     int
}

func (br *badReader) Read(p []byte) (int, error) {
	remaining := br.limit - br.readSoFar
	p = p[:min(remaining, len(p))]

	n, err := br.buf.Read(p)
	br.readSoFar += n

	if err != nil {
		return n, err
	}
	if br.readSoFar >= br.limit {
		return n, errExpected
	}

	return n, nil
}

func (br *badReader) Close() error {
	return nil
}

func TestUploadObject_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr, projectType               string
		loggedErrors                                []string
		fileSize                                    int64
		header, badCopy                             bool
		detailsErr, headerErr, uploadErr, deleteErr error
	}{
		{
			"FAIL_DETAILS", "failed to get details for file test-file.txt: " + errExpected.Error(), "default",
			[]string{},
			65986, false, false,
			errExpected, nil, nil, nil,
		},
		{
			"FAIL_HEADER", "uploading file test-file.txt failed", "default",
			[]string{
				"Streaming file test-file.txt failed: failed to create crypt4gh writer: crypto/ecdh: bad X25519 remote ECDH input: low order point",
				"failed to extract header from encrypted file: EOF",
			},
			2375680, false, false,
			nil, nil, nil, nil,
		},
		{
			"FAIL_SIZE", "file test-file.txt is too large (5497558139316 bytes)", "default",
			[]string{},
			5*(1<<40) + 560, true, false,
			nil, nil, nil, nil,
		},
		{
			"FAIL_POST_HEADER", "uploading file test-file.txt failed", "default",
			[]string{"failed to upload header to vault: " + errExpected.Error()},
			94567376, true, false,
			nil, errExpected, nil, nil,
		},
		{
			"FAIL_UPLOAD", "uploading file test-file.txt failed", "default",
			[]string{errExpected.Error()},
			456, true, false,
			nil, nil, errExpected, nil,
		},
		{
			"FAIL_CRYPT", "uploading file test-file.txt failed", "findata",
			[]string{
				"failed to create crypt4gh writer: crypto/ecdh: bad X25519 remote ECDH input: low order point",
				"failed to extract header from encrypted file: failed to create crypt4gh writer: crypto/ecdh: bad X25519 remote ECDH input: low order point",
			},
			74869, true, false,
			nil, nil, nil, nil,
		},
		{
			"FAIL_FINDATA", "uploading file test-file.txt failed", "findata",
			[]string{
				"failed to upload " + api.Findata + " object: " + errExpected.Error(),
				"failed to read file body: failed to upload " + api.Findata + " object: " + errExpected.Error(),
			},
			98, true, false,
			nil, nil, errExpected, nil,
		},
		{
			"FAIL_COPY_1", "uploading file test-file.txt failed", "default",
			[]string{"Streaming file test-file.txt failed: " + errExpected.Error()},
			456, true, true,
			nil, nil, nil, nil,
		},
		{
			"FAIL_COPY_2", "uploading file test-file.txt failed", "findata",
			[]string{
				"failed to upload Findata object: failed to read file body: " + errExpected.Error(),
				"failed to read file body: failed to upload Findata object: failed to read file body: " + errExpected.Error(),
			},
			456, true, true,
			nil, nil, nil, nil,
		},
		{
			"FAIL_DELETE", "uploading file test-file.txt failed", "default",
			[]string{"Streaming file test-file.txt failed: " + errExpected.Error()},
			456, true, true,
			nil, nil, nil, errExpected,
		},
	}

	origGetFileDetails := getFileDetails
	origGetProjectType := api.GetProjectType
	origPostHeader := api.PostHeader
	origUploadObject := api.UploadObject
	origDeleteObject := api.DeleteObject
	origPublicKey := ai.publicKey
	origError := logs.Error
	origErrorf := logs.Errorf
	defer func() {
		getFileDetails = origGetFileDetails
		api.GetProjectType = origGetProjectType
		api.PostHeader = origPostHeader
		api.UploadObject = origUploadObject
		api.DeleteObject = origDeleteObject
		ai.publicKey = origPublicKey
		logs.Error = origError
		logs.Errorf = origErrorf
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			publicKey, _, err := keys.GenerateKeyPair()
			if err != nil {
				t.Fatalf("Could not generate key pair: %s", err.Error())
			}
			ai.publicKey = publicKey
			if tt.testname == "FAIL_HEADER" || tt.testname == "FAIL_CRYPT" {
				ai.publicKey = [32]byte{}
			}

			api.GetProjectType = func() string {
				return tt.projectType
			}
			getFileDetails = func(filename string) (io.ReadCloser, int64, error) {
				if !tt.header {
					rc := io.NopCloser(strings.NewReader(""))

					return rc, tt.fileSize, tt.detailsErr
				}

				content := test.GenerateRandomText(100)
				buffer := bytes.NewBuffer(content)

				if tt.badCopy {
					return &badReader{buffer, 0, 50}, tt.fileSize, nil
				}

				return io.NopCloser(buffer), tt.fileSize, tt.detailsErr
			}
			api.PostHeader = func(header []byte, bucket, object string) error {
				return tt.headerErr
			}
			api.UploadObject = func(ctx context.Context, body io.Reader, rep, bucket, object string, segmentSize int64, metadata map[string]string) error {
				_, err := io.ReadAll(body)
				if err != nil {
					return fmt.Errorf("failed to read file body: %s", err.Error())
				}

				return tt.uploadErr
			}

			deleted := false
			api.DeleteObject = func(rep, bucket, object string) error {
				if rep != api.SDConnect {
					t.Errorf("api.DeleteObject() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != "test-bucket" {
					t.Errorf("api.DeleteObject() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
				}
				if object != "test-file.txt.c4gh" {
					t.Errorf("api.DeleteObject() received incorrect object. Expected=test-file.txt.c4gh, received=%s", object)
				}
				deleted = true

				return tt.deleteErr
			}

			errs := make([]string, 0)
			logs.Error = func(err error) {
				errs = append(errs, err.Error())
			}
			logs.Errorf = func(format string, args ...any) {
				errs = append(errs, fmt.Errorf(format, args...).Error())
			}
			slices.Sort(errs)

			if err = UploadObject(context.Background(), "test-file.txt", "test-file.txt.c4gh", "test-bucket", nil); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
			if slices.Compare(tt.loggedErrors, errs) != 0 {
				t.Errorf("Function logged incorrect errors\nExpected=%q\nReceived=%q", tt.loggedErrors, errs)
			}
			if tt.badCopy && tt.projectType == "default" && !deleted {
				t.Error("Object was not deleted")
			}
		})
	}
}
