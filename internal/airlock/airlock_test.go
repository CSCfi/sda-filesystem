package airlock

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

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

func TestGetFileDetails(t *testing.T) {
	var tests = []struct {
		testname, message string
		bytes             int64
	}{
		{"OK_1", "pipe message", 164},
		{"OK_2", "another_message", 167},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			file, err := os.CreateTemp("", "file")
			if err != nil {
				t.Fatalf("Failed to create file: %s", err.Error())
			}
			t.Cleanup(func() { os.RemoveAll(file.Name()) })

			if _, err := file.WriteString(tt.message); err != nil {
				t.Fatalf("Failed to write to file: %s", err.Error())
			}
			file.Close()

			rc, bytes, err := getFileDetails(file.Name())
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
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	if _, _, err := getFileDetails(file.Name()); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestCheckObjectExistence_NoBucket(t *testing.T) {
	origBucketExists := api.BucketExists
	defer func() { api.BucketExists = origBucketExists }()

	api.BucketExists = func(rep, bucket string) (bool, error) {
		if rep != api.SDConnect {
			t.Errorf("api.BucketExists() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
		}
		if bucket != "bucket" {
			t.Errorf("api.BucketExists() received incorrect bucket. Expected=bucket, received=%s", bucket)
		}

		return false, nil
	}

	err := CheckObjectExistence("file.txt", "bucket/subfolder/another-dir", strings.NewReader(""))
	if err != nil {
		t.Errorf("Function returned unexpected error: %s", err.Error())
	}
}

func TestCheckObjectExistence_UserInput(t *testing.T) {
	var tests = []struct {
		testname, userInput, errStr string
		objects                     []api.Metadata
	}{
		{
			"OK_1", "", "",
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-object"},
			},
		},
		{
			"OK_2", "yes\n", "",
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/dir/file.txt"},
				{Name: "subfolder/another-dir/file.txt.c4gh"},
				{Name: "subfolder/another-object"},
			},
		},
		{
			"OK_3", "y\nno\n", "",
			[]api.Metadata{
				{Name: "subfolder/another-dir/file.txt.c4gh"},
				{Name: "some-object"},
				{Name: "subfolder/another-object"},
			},
		},
		{
			"FAIL_NO", "ye\nno\n", "not permitted to override data",
			[]api.Metadata{
				{Name: "some-object"},
				{Name: "subfolder/another-dir/file.txt.c4gh"},
			},
		},
		{
			"FAIL_INPUT", "blaa", "failed to read user input: EOF",
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

				return tt.objects, nil
			}

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			err := CheckObjectExistence("file.txt", "bucket/subfolder/another-dir", strings.NewReader(tt.userInput))

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

func TestCheckObjectExistence_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr     string
		bucketErr, objectErr error
	}{
		{
			"FAIL_1", errExpected.Error(), errExpected, nil,
		},
		{
			"FAIL_2", "could not determine if export will override data: " + errExpected.Error(),
			nil, errExpected,
		},
	}

	origBucketExists := api.BucketExists
	origGetObjects := api.GetObjects
	defer func() {
		api.BucketExists = origBucketExists
		api.GetObjects = origGetObjects
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.BucketExists = func(rep, bucket string) (bool, error) {
				return true, tt.bucketErr
			}
			api.GetObjects = func(rep, bucket, path string, prefix ...string) ([]api.Metadata, error) {
				return nil, tt.objectErr
			}

			err := CheckObjectExistence("file.txt", "bucket/subfolder/another-dir", strings.NewReader(""))
			if err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestUpload(t *testing.T) {
	var tests = []struct {
		testname, projectType, bucket, filename, object string
		createBucket                                    bool
		fileSize, segmentSize                           int64
	}{
		{
			"OK_1", "default", "test-bucket", "test-file.txt", "test-file.txt.c4gh",
			false, 9463686, 1 << 27,
		},
		{
			"OK_2", "default", "test-bucket/subfolder", "test-file.txt", "subfolder/test-file.txt.c4gh",
			true, (1 << 40) * 1.5, 1 << 28,
		},
		{
			"OK_3", "findata", "test-bucket", "test-file.txt", "test-file.txt.c4gh",
			false, 9463686, 1 << 27,
		},
		{
			"OK_4", "findata", "test-bucket/subfolder", "test-file.txt", "subfolder/test-file.txt.c4gh",
			true, (1 << 40) * 1.5, 1 << 28,
		},
	}

	origGetPublicKey := api.GetPublicKey
	origBucketExists := api.BucketExists
	origCreateBucket := api.CreateBucket
	origGetFileDetails := getFileDetails
	origProjectName := api.GetProjectName
	origGetProjectType := api.GetProjectType
	origPostHeader := api.PostHeader
	origUploadObject := api.UploadObject
	origDeleteObject := api.DeleteObject
	origPublicKey := ai.publicKey
	defer func() {
		api.GetPublicKey = origGetPublicKey
		api.BucketExists = origBucketExists
		api.CreateBucket = origCreateBucket
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
			content := test.GenerateRandomText(100)

			api.GetPublicKey = func() ([32]byte, error) {
				return publicKey, nil
			}
			api.GetProjectType = func() string {
				return tt.projectType
			}
			api.BucketExists = func(rep, bucket string) (bool, error) {
				if rep != api.SDConnect {
					t.Errorf("api.BucketExists() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != "test-bucket" {
					t.Errorf("api.BucketExists() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
				}

				return !tt.createBucket, nil
			}
			api.CreateBucket = func(rep, bucket string) error {
				if rep != api.SDConnect {
					t.Errorf("api.CreateBucket() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != "test-bucket" {
					t.Errorf("api.CreateBucket() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
				}

				return nil
			}
			getFileDetails = func(filename string) (io.ReadCloser, int64, error) {
				rc := io.NopCloser(bytes.NewReader(content))

				return rc, tt.fileSize, nil
			}

			receivedContent := make([]byte, 0)
			api.PostHeader = func(header []byte, bucket, object string) error {
				if bucket != "test-bucket" {
					t.Errorf("api.PostHeader() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
				}
				if object != tt.object {
					t.Errorf("api.PostHeader() received incorrect object. Expected=%s, received=%s", tt.object, object)
				}
				receivedContent = append(receivedContent, header...)

				return nil
			}
			//nolint:nestif
			if tt.projectType == "default" {
				api.UploadObject = func(body io.Reader, rep, bucket, object string, segmentSize int64) error {
					if rep != api.SDConnect {
						t.Errorf("api.UploadObject() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
					}
					if bucket != "test-bucket" {
						t.Errorf("api.UploadObject() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
					}
					if object != tt.object {
						t.Errorf("api.UploadObject() received incorrect object. Expected=%s, received=%s", tt.object, object)
					}
					if segmentSize != tt.segmentSize {
						t.Errorf("api.UploadObject() received incorrect segment size. Expected=%d, received=%d", tt.segmentSize, segmentSize)
					}

					bodyBytes, err := io.ReadAll(body)
					if err != nil {
						return fmt.Errorf("failed to read file body: %w", err)
					}
					receivedContent = append(receivedContent, bodyBytes...)

					c4ghr, err := streaming.NewCrypt4GHReader(bytes.NewReader(receivedContent), privateKey, nil)
					if err != nil {
						t.Errorf("Failed to create crypt4gh reader: %s", err.Error())
					} else if message, err := io.ReadAll(c4ghr); err != nil {
						t.Errorf("Failed to read from encrypted file: %s", err.Error())
					} else if string(message) != string(content) {
						t.Errorf("Reader received incorrect message\nExpected=%s\nReceived=%s", string(content), string(message))
					}

					return nil
				}
			} else {
				api.UploadObject = func(body io.Reader, rep, bucket, object string, segmentSize int64) error {
					switch rep {
					case api.SDConnect:
						if bucket != "test-bucket" {
							t.Errorf("api.UploadObject() received incorrect %s bucket. Expected=test-bucket, received=%s", api.SDConnect, bucket)
						}
						if object != tt.object {
							t.Errorf("api.UploadObject() received incorrect %s object. Expected=%s, received=%s", api.SDConnect, tt.object, object)
						}
					case api.Findata:
						if bucket != "" {
							t.Errorf("api.UploadObject() received incorrect %s bucket. Expected=, received=%s", api.Findata, bucket)
						}
						if object != "findata-project/test-bucket/"+tt.object {
							t.Errorf("api.UploadObject() received incorrect %s object\nExpected=findata-project/test-bucket/%s\nReceived=%s", api.Findata, tt.object, object)
						}
					default:
						t.Fatalf("api.UploadObject() received incorrect repository %s", rep)
					}

					if segmentSize != tt.segmentSize {
						t.Errorf("api.UploadObject() received incorrect segment size. Expected=%d, received=%d", tt.segmentSize, segmentSize)
					}

					bodyBytes, err := io.ReadAll(body)
					if err != nil {
						return fmt.Errorf("failed to read file body: %s", err.Error())
					}

					if rep == api.Findata {
						if !reflect.DeepEqual(bodyBytes, content) {
							t.Fatalf("findata reader returned incorrect body.\nExpected=%s\nReceived=%s", string(content), string(bodyBytes))
						}

						return nil
					}

					receivedContent = append(receivedContent, bodyBytes...)

					c4ghr, err := streaming.NewCrypt4GHReader(bytes.NewReader(receivedContent), privateKey, nil)
					if err != nil {
						t.Fatalf("failed to create crypt4gh reader: %s", err.Error())
					} else if message, err := io.ReadAll(c4ghr); err != nil {
						t.Errorf("Failed to read from encrypted file: %s", err.Error())
					} else if string(message) != string(content) {
						t.Errorf("Reader received incorrect message\nExpected=%s\nReceived=%s", string(content), string(message))
					}

					return nil
				}
			}
			api.DeleteObject = func(rep, bucket, object string) error {
				t.Error("Should not call api.DeleteObject()")

				return nil
			}

			if err := Upload(tt.filename, tt.bucket); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
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

func TestUpload_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr, projectType                                             string
		loggedErrors                                                              []string
		fileSize                                                                  int64
		header, badCopy                                                           bool
		keyErr, existsErr, createErr, detailsErr, headerErr, uploadErr, deleteErr error
	}{
		{
			"FAIL_KEY", "failed to get project public key: " + errExpected.Error(), "default",
			[]string{},
			500, false, false,
			errExpected, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_EXISTS", errExpected.Error(), "default",
			[]string{},
			458478, false, false,
			nil, errExpected, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_CREATE", errExpected.Error(), "default",
			[]string{},
			234, false, false,
			nil, nil, errExpected, nil, nil, nil, nil,
		},
		{
			"FAIL_DETAILS", "failed to get details for file test-file.txt: " + errExpected.Error(), "default",
			[]string{},
			65986, false, false,
			nil, nil, nil, errExpected, nil, nil, nil,
		},
		{
			"FAIL_HEADER", "uploading file test-file.txt failed", "default",
			[]string{
				"Streaming file test-file.txt failed: failed to create crypt4gh writer: crypto/ecdh: bad X25519 remote ECDH input: low order point",
				"failed to extract header from encrypted file: EOF",
			},
			2375680, false, false,
			nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_SIZE", "file test-file.txt is too large (5497558139316 bytes)", "default",
			[]string{},
			5*(1<<40) + 560, true, false,
			nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_POST_HEADER", "uploading file test-file.txt failed", "default",
			[]string{"failed to upload header to vault: " + errExpected.Error()},
			94567376, true, false,
			nil, nil, nil, nil, errExpected, nil, nil,
		},
		{
			"FAIL_UPLOAD", "uploading file test-file.txt failed", "default",
			[]string{errExpected.Error()},
			456, true, false,
			nil, nil, nil, nil, nil, errExpected, nil,
		},
		{
			"FAIL_CRYPT", "uploading file test-file.txt failed", "findata",
			[]string{
				"failed to create crypt4gh writer: crypto/ecdh: bad X25519 remote ECDH input: low order point",
				"failed to extract header from encrypted file: failed to create crypt4gh writer: crypto/ecdh: bad X25519 remote ECDH input: low order point",
			},
			74869, true, false,
			nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_FINDATA", "uploading file test-file.txt failed", "findata",
			[]string{
				"failed to upload " + api.Findata + " object: " + errExpected.Error(),
				"failed to read file body: failed to upload " + api.Findata + " object: " + errExpected.Error(),
			},
			98, true, false,
			nil, nil, nil, nil, nil, errExpected, nil,
		},
		{
			"FAIL_COPY_1", "uploading file test-file.txt failed", "default",
			[]string{"Streaming file test-file.txt failed: " + errExpected.Error()},
			456, true, true,
			nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_COPY_2", "uploading file test-file.txt failed", "findata",
			[]string{
				"failed to upload Findata object: failed to read file body: " + errExpected.Error(),
				"failed to read file body: failed to upload Findata object: failed to read file body: " + errExpected.Error(),
			},
			456, true, true,
			nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_DELETE", "uploading file test-file.txt failed", "default",
			[]string{"Streaming file test-file.txt failed: " + errExpected.Error()},
			456, true, true,
			nil, nil, nil, nil, nil, nil, errExpected,
		},
	}

	origGetPublicKey := api.GetPublicKey
	origBucketExists := api.BucketExists
	origCreateBucket := api.CreateBucket
	origGetFileDetails := getFileDetails
	origGetProjectType := api.GetProjectType
	origPostHeader := api.PostHeader
	origUploadObject := api.UploadObject
	origDeleteObject := api.DeleteObject
	origPublicKey := ai.publicKey
	origNewCrypt4GHWriter := newCrypt4GHWriter
	origError := logs.Error
	origErrorf := logs.Errorf
	defer func() {
		api.GetPublicKey = origGetPublicKey
		api.BucketExists = origBucketExists
		api.CreateBucket = origCreateBucket
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

			api.GetPublicKey = func() ([32]byte, error) {
				if tt.testname == "FAIL_HEADER" || tt.testname == "FAIL_CRYPT" {
					return [32]byte{}, nil
				}

				return publicKey, tt.keyErr
			}
			api.BucketExists = func(rep, bucket string) (bool, error) {
				return false, tt.existsErr
			}
			api.CreateBucket = func(rep, bucket string) error {
				return tt.createErr
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
			api.UploadObject = func(body io.Reader, rep, bucket, object string, segmentSize int64) error {
				_, err := io.ReadAll(body)
				if err != nil {
					return fmt.Errorf("failed to read file body: %s", err.Error())
				}

				return tt.uploadErr
			}
			if tt.createErr != nil {
				newCrypt4GHWriter = func(wr io.Writer) (*streaming.Crypt4GHWriter, error) {
					return nil, tt.createErr
				}
				t.Cleanup(func() { newCrypt4GHWriter = origNewCrypt4GHWriter })
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

			if err = Upload("test-file.txt", "test-bucket"); err == nil {
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
