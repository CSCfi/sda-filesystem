package airlock

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
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

func TestPublicKey(t *testing.T) {
	var tests = []struct {
		testname, projectType string
		publicKeys            [][32]byte
	}{
		{
			"OK_1", "default",
			[][32]byte{
				{7, 60, 179, 123, 49, 12, 3, 30, 95, 223, 207, 243, 27, 55, 63, 204, 63, 58, 222, 63, 231, 28, 88, 94, 68, 127, 83, 51, 247, 151, 34, 59},
			},
		},
		{
			"OK_2", "registry",
			[][32]byte{
				{7, 60, 179, 123, 49, 12, 3, 30, 95, 223, 207, 243, 27, 55, 63, 204, 63, 58, 222, 63, 231, 28, 88, 94, 68, 127, 83, 51, 247, 151, 34, 59},
				{30, 209, 47, 90, 244, 98, 245, 237, 16, 188, 167, 150, 77, 245, 63, 65, 48, 145, 71, 149, 166, 230, 180, 165, 115, 203, 254, 143, 24, 205, 69, 117},
			},
		},
		{
			"OK_3", "findata",
			[][32]byte{
				{30, 209, 47, 90, 244, 98, 245, 237, 16, 188, 167, 150, 77, 245, 63, 65, 48, 145, 71, 149, 166, 230, 180, 165, 115, 203, 254, 143, 24, 205, 69, 117},
			},
		},
	}

	origGetProjectType := api.GetProjectType
	origMakeRequest := api.MakeRequest
	origPublicKeys := ai.publicKeys
	defer func() {
		api.GetProjectType = origGetProjectType
		api.MakeRequest = origMakeRequest
		ai.publicKeys = origPublicKeys
	}()

	api.MakeRequest = func(method, path string, query, headers map[string]string, body io.Reader, ret any) error {
		if method != "GET" {
			return fmt.Errorf("request has incorrect method\nExpected=GET\nReceived=%v", method)
		}
		switch v := ret.(type) {
		case *keyResponse:
			switch path {
			case "/desktop/project-key":
				v.Key64 = "-----BEGIN CRYPT4GH PUBLIC KEY-----\nBzyzezEMAx5f38/zGzc/zD863j/nHFheRH9TM/eXIjs=\n-----END CRYPT4GH PUBLIC KEY-----"
			case "/public-key":
				v.Key64 = "-----BEGIN CRYPT4GH PUBLIC KEY-----\nHtEvWvRi9e0QvKeWTfU/QTCRR5Wm5rSlc8v+jxjNRXU=\n-----END CRYPT4GH PUBLIC KEY-----"
			default:
				return fmt.Errorf("request has incorrect path %v", path)
			}

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *keyResponse", reflect.TypeOf(v))
		}
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetProjectType = func() string {
				return tt.projectType
			}

			if err := getPublicKeys(); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(ai.publicKeys, tt.publicKeys) {
				t.Errorf("Function saved incorrect public key\nExpected=%v\nReceived=%v", tt.publicKeys, ai.publicKeys)
			}
		})
	}
}

func TestPublicKey_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		err1, err2       error
	}{
		{"OK_1", "failed to get project public key: " + errExpected.Error(), errExpected, nil},
		{"OK_2", "failed to get findata public key: " + errExpected.Error(), nil, errExpected},
	}

	origGetProjectType := api.GetProjectType
	origMakeRequest := api.MakeRequest
	origPublicKeys := ai.publicKeys
	defer func() {
		api.GetProjectType = origGetProjectType
		api.MakeRequest = origMakeRequest
		ai.publicKeys = origPublicKeys
	}()

	api.GetProjectType = func() string {
		return "registry"
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.MakeRequest = func(method, path string, query, headers map[string]string, body io.Reader, ret any) error {
				if method != "GET" {
					return fmt.Errorf("request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				switch v := ret.(type) {
				case *keyResponse:
					v.Key64 = "-----BEGIN CRYPT4GH PUBLIC KEY-----\nBzyzezEMAx5f38/zGzc/zD863j/nHFheRH9TM/eXIjs=\n-----END CRYPT4GH PUBLIC KEY-----"
					switch path {
					case "/desktop/project-key":
						return tt.err1
					case "/public-key":
						return tt.err2
					default:
						return fmt.Errorf("request has incorrect path %v", path)
					}
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *keyResponse", reflect.TypeOf(v))
				}
			}

			if err := getPublicKeys(); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestPublicKey_InvalidKey(t *testing.T) {
	origMakeRequest := api.MakeRequest
	origPublicKeys := ai.publicKeys
	defer func() {
		api.MakeRequest = origMakeRequest
		ai.publicKeys = origPublicKeys
	}()

	api.MakeRequest = func(method, path string, query, headers map[string]string, body io.Reader, ret any) error {
		switch v := ret.(type) {
		case *keyResponse:
			v.Key64 = "-----BEGIN CRYPT4GH PUBLIC KEY-----\nSGVsbG8sIHdvcmxkIQ==\n-----END CRYPT4GH PUBLIC KEY-----"

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *keyResponse", reflect.TypeOf(v))
		}
	}

	errStr := "failed to get project public key: Unsupported key file format"
	if err := getPublicKeys(); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestGetFileDetails(t *testing.T) {
	var tests = []struct {
		testname, message string
		bytes             int64
	}{
		{"OK_1", "pipe message", 164},
		{"OK_2", "another_message", 275},
	}

	origPublicKeys := ai.publicKeys
	defer func() { ai.publicKeys = origPublicKeys }()

	for i, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			ai.publicKeys = make([][32]byte, i+1)
			for j := range i {
				publicKey, _, err := keys.GenerateKeyPair()
				if err != nil {
					t.Fatalf("Could not generate key pair: %s", err.Error())
				}
				ai.publicKeys[j] = publicKey
			}
			publicKey, privateKey, err := keys.GenerateKeyPair()
			if err != nil {
				t.Fatalf("Could not generate key pair: %s", err.Error())
			}
			ai.publicKeys[i] = publicKey

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
				t.Fatal("Function returned nil readCloser")
			}

			c4ghr, err := streaming.NewCrypt4GHReader(rc, privateKey, nil)
			if err != nil {
				t.Errorf("Failed to create crypt4gh reader: %s", err.Error())
			} else if message, err := io.ReadAll(c4ghr); err != nil {
				t.Errorf("Failed to read from encrypted file: %s", err.Error())
			} else if tt.message != string(message) {
				t.Errorf("Reader received incorrect message\nExpected=%s\nReceived=%s", tt.message, string(message))
			}
			if err = <-rc.errc; err != nil {
				t.Errorf("Channel returned an error: %s", err.Error())
			}
			if err = rc.Close(); err != nil {
				t.Errorf("Closing file caused error: %s", err.Error())
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

func TestFileDetails_CopyError(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	t.Cleanup(func() { os.RemoveAll(file.Name()) })

	rc, _, err := getFileDetails(file.Name())
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if rc == nil {
		t.Fatal("Function returned nil readCloser")
	}

	rc.Close()

	_, _ = io.ReadAll(rc)
	if err = <-rc.errc; err == nil {
		t.Fatalf("Channel did not return error")
	}
	errStr := fmt.Sprintf("read %s: file already closed", file.Name())
	if err.Error() != errStr {
		t.Fatalf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
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
		testname, bucket, filename, object string
		createBucket                       bool
		fileSize, segmentSize              int64
	}{
		{
			"OK_1", "test-bucket", "test-file.txt", "test-file.txt.c4gh",
			false, 9463686, 1 << 27,
		},
		{
			"OK_2", "test-bucket/subfolder", "test-file.txt", "subfolder/test-file.txt.c4gh",
			false, (1 << 40) * 1.5, 1 << 28,
		},
	}

	origGetPublicKeys := getPublicKeys
	origBucketExists := api.BucketExists
	origCreateBucket := api.CreateBucket
	origGetFileDetails := getFileDetails
	origPostHeader := api.PostHeader
	origUploadObject := api.UploadObject
	origDeleteObject := api.DeleteObject
	defer func() {
		getPublicKeys = origGetPublicKeys
		api.BucketExists = origBucketExists
		api.CreateBucket = origCreateBucket
		getFileDetails = origGetFileDetails
		api.PostHeader = origPostHeader
		api.UploadObject = origUploadObject
		api.DeleteObject = origDeleteObject
	}()

	getPublicKeys = func() error {
		return nil
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			content := test.GenerateRandomText(100)
			headerBytes, encryptedContent, _ := test.EncryptData(t, content)

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
			getFileDetails = func(filename string) (*readCloser, int64, error) {
				errc := make(chan error, 1)
				errc <- nil
				rc := io.NopCloser(bytes.NewReader(append(headerBytes, encryptedContent...)))

				return &readCloser{Reader: rc, Closer: rc, errc: errc}, tt.fileSize, nil
			}
			api.PostHeader = func(header []byte, bucket, object string) error {
				if bucket != "test-bucket" {
					t.Errorf("api.PostHeader() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
				}
				if object != tt.object {
					t.Errorf("api.PostHeader() received incorrect object. Expected=%s, received=%s", tt.object, object)
				}
				if !reflect.DeepEqual(header, headerBytes) {
					t.Errorf("api.PostHeader() received incorrect header\nExpected=%v\nReceived=%v", headerBytes, header)
				}

				return nil
			}
			api.UploadObject = func(encryptedBody io.Reader, rep, bucket, object string, segmentSize int64) error {
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

				body, err := io.ReadAll(encryptedBody)
				if err != nil {
					return fmt.Errorf("failed to read file body: %w", err)
				}
				if !reflect.DeepEqual(body, encryptedContent) {
					t.Errorf("api.UploadObject() received incorrect body\nExpected=%v\nReceived=%v", encryptedContent, body)
				}

				return nil
			}
			api.DeleteObject = func(rep, bucket, object string) error {
				if rep != api.SDConnect {
					t.Errorf("api.DeleteObject() received incorrect repository. Expected=%s, received=%s", api.SDConnect, rep)
				}
				if bucket != "test-bucket" {
					t.Errorf("api.DeleteObject() received incorrect bucket. Expected=test-bucket, received=%s", bucket)
				}
				if object != tt.object {
					t.Errorf("api.DeleteObject() received incorrect object. Expected=%s, received=%s", tt.object, object)
				}

				return nil
			}

			if err := Upload(tt.filename, tt.bucket); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			}
		})
	}
}

func TestUpload_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr                                                                 string
		fileSize                                                                         int64
		header                                                                           bool
		keyErr, existsErr, createErr, detailsErr, headerErr, uploadErr, deleteErr, chErr error
	}{
		{
			"FAIL_1", errExpected.Error(), 500, false,
			errExpected, nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_2", errExpected.Error(), 458478, false,
			nil, errExpected, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_3", errExpected.Error(), 234, false,
			nil, nil, errExpected, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_4", "failed to get details for file test-file.txt: " + errExpected.Error(), 65986, false,
			nil, nil, nil, errExpected, nil, nil, nil, nil,
		},
		{
			"FAIL_5", "failed to extract header from encrypted file: EOF", 2375680, false,
			nil, nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_6", "file test-file.txt is too large (5497558139316 bytes)", 5*(1<<40) + 560, true,
			nil, nil, nil, nil, nil, nil, nil, nil,
		},
		{
			"FAIL_7", "failed to upload header to vault: " + errExpected.Error(), 94567376, true,
			nil, nil, nil, nil, errExpected, nil, nil, nil,
		},
		{
			"FAIL_8", errExpected.Error(), 456, true,
			nil, nil, nil, nil, nil, errExpected, nil, nil,
		},
		{
			"FAIL_9", "streaming file test-file.txt failed: " + errExpected.Error(), 24856, true,
			nil, nil, nil, nil, nil, nil, nil, errExpected,
		},
		{
			"FAIL_10", "streaming file test-file.txt failed: " + errExpected.Error(), 78, true,
			nil, nil, nil, nil, nil, nil, errExpected, errExpected,
		},
	}

	origGetPublicKeys := getPublicKeys
	origBucketExists := api.BucketExists
	origCreateBucket := api.CreateBucket
	origGetFileDetails := getFileDetails
	origPostHeader := api.PostHeader
	origUploadObject := api.UploadObject
	origDeleteObject := api.DeleteObject
	defer func() {
		getPublicKeys = origGetPublicKeys
		api.BucketExists = origBucketExists
		api.CreateBucket = origCreateBucket
		getFileDetails = origGetFileDetails
		api.PostHeader = origPostHeader
		api.UploadObject = origUploadObject
		api.DeleteObject = origDeleteObject
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			getPublicKeys = func() error {
				return tt.keyErr
			}
			api.BucketExists = func(rep, bucket string) (bool, error) {
				return false, tt.existsErr
			}
			api.CreateBucket = func(rep, bucket string) error {
				return tt.createErr
			}
			getFileDetails = func(filename string) (*readCloser, int64, error) {
				errc := make(chan error, 1)
				errc <- tt.chErr
				if !tt.header {
					rc := io.NopCloser(strings.NewReader(""))

					return &readCloser{Reader: rc, Closer: rc, errc: errc}, tt.fileSize, tt.detailsErr
				}

				content := test.GenerateRandomText(100)
				headerBytes, _, _ := test.EncryptData(t, content)
				rc := io.NopCloser(bytes.NewReader(headerBytes))

				return &readCloser{Reader: rc, Closer: rc, errc: errc}, tt.fileSize, tt.detailsErr
			}
			api.PostHeader = func(header []byte, bucket, object string) error {
				return tt.headerErr
			}
			api.UploadObject = func(encryptedBody io.Reader, rep, bucket, object string, segmentSize int64) error {
				return tt.uploadErr
			}
			api.DeleteObject = func(rep, bucket, object string) error {
				return tt.deleteErr
			}

			err := Upload("test-file.txt", "test-bucket")
			if err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}
