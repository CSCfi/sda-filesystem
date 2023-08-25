package airlock

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/neicnordic/crypt4gh/keys"
	"github.com/neicnordic/crypt4gh/streaming"
)

var errExpected = errors.New("Expected error for test")

func TestMain(m *testing.M) {
	logs.SetSignal(func(string, []string) {})
	os.Exit(m.Run())
}

func TestGetProxy(t *testing.T) {
	var tests = []struct {
		testname, proxy string
		envErr          error
	}{
		{"OK_1", "test_url", nil},
		{"OK_2", "another_url", nil},
		{"FAIL", "", errExpected},
	}

	origGetEnv := api.GetEnv
	defer func() { api.GetEnv = origGetEnv }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetEnv = func(name string, verifyURL bool) (string, error) {
				return tt.proxy, tt.envErr
			}

			if err := GetProxy(); err == nil {
				if tt.envErr != nil {
					t.Error("Function did not return error")
				}
			} else if tt.envErr == nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if err.Error() != tt.envErr.Error() {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.envErr.Error(), err.Error())
			}
		})
	}
}

func TestIsProjectManager_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	origInfoFile := infoFile
	defer func() { infoFile = origInfoFile }()
	infoFile = file.Name()

	errStr := fmt.Sprintf("Could not find user info: open %s: no such file or directory", file.Name())
	if _, err := IsProjectManager(""); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestIsProjectManager_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		err              error
		data             map[string]string
	}{
		{
			"EMPTY_FILE", "Could not find user info: unexpected end of JSON input", nil, nil,
		},
		{
			"NO_ENDPOINT",
			"Could not determine endpoint for user info: Config file did not contain key 'userinfo_endpoint'",
			nil, map[string]string{"test": "Text"},
		},
		{
			"NO_PROJECT",
			"Could not determine to which project this Desktop belongs: Config file did not contain key 'login_aud'",
			nil, map[string]string{"userinfo_endpoint": "test_point"},
		},
		{
			"REQUEST_FAIL_1", errExpected.Error(), errExpected,
			map[string]string{"userinfo_endpoint": "test_point", "login_aud": "test_aud"},
		},
		{
			"REQUEST_FAIL_2", "Invalid token", &api.RequestError{StatusCode: 400},
			map[string]string{"userinfo_endpoint": "test_point", "login_aud": "test_aud"},
		},
		{
			"REQUEST_FAIL_3", "Response body did not contain key 'projectPI'", nil,
			map[string]string{"userinfo_endpoint": "test_point", "login_aud": "test_aud"},
		},
	}

	origInfoFile := infoFile
	defer func() { infoFile = origInfoFile }()

	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			if err = file.Truncate(0); err != nil {
				t.Errorf("Could not truncate file: %s", err.Error())
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Errorf("Could not change file offset: %s", err.Error())
			}

			if tt.data != nil {
				data := tt.data
				encoder := json.NewEncoder(file)
				if err = encoder.Encode(data); err != nil {
					t.Errorf("Failed to encode data to json file: %s", err.Error())
				}
			}

			infoFile = file.Name()
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				return tt.err
			}

			if _, err := IsProjectManager(""); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestIsProjectManager(t *testing.T) {
	var tests = []struct {
		testname, data, project, projectParam string
		isManager, overridden                 bool
	}{
		{
			"OK_1", "6036 7843 6947",
			"", "", false, false,
		},
		{
			"OK_2", "1394 4726 9362",
			"project_4726", "", true, false,
		},
		{
			"OK_3", "4726 0837 7295",
			"project_7295", "project_7295", true, true,
		},
		{
			"OK_3", "0593 9274 2735",
			"project_9274", "project_9274", true, true,
		},
	}

	origInfoFile := infoFile
	origMakeRequest := api.MakeRequest
	origProject := ai.project
	defer func() {
		infoFile = origInfoFile
		api.MakeRequest = origMakeRequest
		ai.project = origProject
	}()

	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	data := map[string]string{"userinfo_endpoint": "test_point2", "login_aud": "4726"}
	encoder := json.NewEncoder(file)
	if err = encoder.Encode(data); err != nil {
		t.Fatalf("Failed to encode data to json file: %s", err.Error())
	}

	infoFile = file.Name()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				switch v := ret.(type) {
				case *map[string]any:
					(*v)["projectPI"] = tt.data

					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *map[string]any", reflect.TypeOf(v))
				}
			}

			switch isManager, err := IsProjectManager(tt.projectParam); {
			case err != nil:
				t.Errorf("Function returned unexpected error: %s", err.Error())
			case tt.isManager != isManager:
				t.Errorf("Function returned incorrect manager status. Expected=%v, received=%v", tt.isManager, isManager)
			case isManager && ai.project != tt.project:
				t.Errorf("Field 'project' not defined correctly. Expected=%v, received=%v", tt.project, ai.project)
			case ai.overridden != tt.overridden:
				t.Errorf("Field 'overridden' not defined correctly. Expected=%v, received=%v", tt.overridden, ai.overridden)
			}
		})
	}
}

func TestExtractKey_Invalid_Key(t *testing.T) {
	var tests = []struct {
		testname, key, errStr string
	}{
		{
			"PREFIX_FAIL",
			"-----bad header-----\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBiZ2lyeXM0OWM=\n-----END CRYPT4GH PUBLIC KEY-----",
			"Invalid public key format \"-----bad header-----\\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBiZ2lyeXM0OWM=\\n-----END CRYPT4GH PUBLIC KEY-----\"",
		},
		{
			"SUFFIX_FAIL",
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBiZ2lyeXM0OWM=\n-----bad suffix-----",
			"Invalid public key format \"-----BEGIN CRYPT4GH PUBLIC KEY-----\\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBiZ2lyeXM0OWM=\\n-----bad suffix-----\"",
		},
		{
			"FORMAT_FAIL",
			"-----BEGiuäbhuöo m\nuöyv",
			"Invalid public key format \"-----BEGiuäbhuöo m\\nuöyv\"",
		},
		{
			"KEY_DECODE_LEN_FAIL",
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\nSGVsbG8sIHdvcmxkIQ==\n-----END CRYPT4GH PUBLIC KEY-----",
			"Invalid length of decoded public key (13)",
		},
		{
			"KEY_DECODE_FAIL",
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\ngibberishh\n-----END CRYPT4GH PUBLIC KEY-----",
			"Could not decode public key: illegal base64 data at input byte 8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			if _, err := extractKey([]byte(tt.key)); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestExtractKey(t *testing.T) {
	var tests = []struct {
		testname, key, decodedKey string
	}{
		{
			"OK_1",
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBiZ2lyeXM0OWM=\n-----END CRYPT4GH PUBLIC KEY-----",
			"Good morning sunshine! bgirys49c",
		},
		{
			"OK_2",
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBnaDZ4Mzl4dGs=\n-----END CRYPT4GH PUBLIC KEY-----\n",
			"Good morning sunshine! gh6x39xtk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			if key, err := extractKey([]byte(tt.key)); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if tt.decodedKey != string(key[:]) {
				t.Errorf("Function saved incorrect public key\nExpected=%s\nReceived=%s", []byte(tt.decodedKey), key)
			}
		})
	}
}

func TestPublicKey_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		flags            []string
		keyErr, reqErr   error
	}{
		{
			"REQUEST_FAIL",
			fmt.Sprintf("Failed to get public key for Airlock: %s", errExpected.Error()),
			[]string{}, nil, errExpected,
		},
		{
			"EXTRACT_FAIL",
			fmt.Sprintf("Failed to get public key for Airlock: %s", errExpected.Error()),
			nil, errExpected, nil,
		},
		{
			"READ_FILE_FAIL",
			"Could not use key test-key.pub: open test-key.pub: no such file or directory",
			[]string{"test-key.pub"}, errExpected, nil,
		},
		{
			"EXTRACT_FAIL2",
			fmt.Sprintf("Could not use key ../../test/test-key.pub: %s", errExpected.Error()),
			[]string{"../../test/test-key.pub"}, errExpected, nil,
		},
	}

	origExtractKey := extractKey
	origMakeRequest := api.MakeRequest
	defer func() {
		extractKey = origExtractKey
		api.MakeRequest = origMakeRequest
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			extractKey = func(keySlice []byte) ([32]byte, error) {
				return [32]byte{}, tt.keyErr
			}
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				return tt.reqErr
			}

			if err := GetPublicKey(tt.flags); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestPublicKey(t *testing.T) {
	var tests = []struct {
		testname, key     string
		decodedKey, flags []string
	}{
		{
			"OK_1",
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBiZ2lyeXM0OWM=\n-----END CRYPT4GH PUBLIC KEY-----",
			[]string{"Good morning sunshine! bgirys49c"}, nil,
		},
		{
			"OK_2",
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBnaDZ4Mzl4dGs=\n-----END CRYPT4GH PUBLIC KEY-----\n",
			[]string{"Good morning sunshine! gh6x39xtk"}, nil,
		},
		{
			"OK_3", "", []string{"Good morning sunshine! gh6x39xtk"},
			[]string{"../../test/test-key2.pub"},
		},
		{
			"OK_4", "", []string{"Good morning sunshine! bgirys49c", "Good morning sunshine! gh6x39xtk"},
			[]string{"../../test/test-key.pub", "../../test/test-key2.pub"},
		},
	}

	origExtractKey := extractKey
	origMakeRequest := api.MakeRequest
	defer func() {
		extractKey = origExtractKey
		api.MakeRequest = origMakeRequest
	}()

	var keyStr string
	api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		if keyStr == "" {
			return fmt.Errorf("Should not call api.MakeRequest()")
		}
		switch v := ret.(type) {
		case *[]byte:
			(*v) = []byte(keyStr)

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *byte[]", reflect.TypeOf(v))
		}
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			keyStr = tt.key
			dKeys := make([][32]byte, len(tt.decodedKey))
			for i := range tt.decodedKey {
				copy(dKeys[i][:], []byte(tt.decodedKey[i]))
			}

			if err := GetPublicKey(tt.flags); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(dKeys, ai.publicKeys) {
				t.Errorf("Function saved incorrect public key\nExpected=%s\nReceived=%s", dKeys, ai.publicKeys)
			}
		})
	}
}

func TestUpload_Stat_Error(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("stat %s: no such file or directory", file.Name())
	if err := Upload(file.Name(), "container", 30, "", false); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUpload_File(t *testing.T) {
	var tests = []struct {
		testname              string
		encError, uploadError error
	}{
		{"OK", nil, nil},
		{"FAIL_ENCRYPTION", errExpected, nil},
		{"FAIL_UPLOAD_FILE", nil, errExpected},
	}

	origCheckEncryption := CheckEncryption
	origUploadFile := UploadFile
	defer func() {
		CheckEncryption = origCheckEncryption
		UploadFile = origUploadFile
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			CheckEncryption = func(filename string) (bool, error) {
				if filename != "../../test/sample.txt" {
					t.Errorf("CheckEncryption() was called with incorrect file name %s", filename)
				}

				return false, tt.encError
			}

			count := 0
			UploadFile = func(filename, container string, segmentSizeMb uint64, journalNumber string, useOriginal, encrypted bool) error {
				if filename != "../../test/sample.txt" {
					t.Errorf("UploadFile() was called with incorrect file name %s", filename)
				}
				count++
				if count > 1 {
					t.Errorf("UploadFile() was called too many times")
				}

				return tt.uploadError
			}

			err := Upload("../../test/sample.txt", "container", 30, "", false)
			if tt.testname == "OK" {
				if err != nil {
					t.Errorf("Function returned unexpected error: %s", err.Error())
				}
			} else if err == nil {
				t.Error("Function did not return error")
			}
		})
	}
}

func TestUpload_Folder(t *testing.T) {
	origCheckEncryption := CheckEncryption
	origUploadFile := UploadFile
	defer func() {
		CheckEncryption = origCheckEncryption
		UploadFile = origUploadFile
	}()

	dir, err := os.MkdirTemp("../../test/", "testdir")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(dir)

	file1 := filepath.Join(dir, "tmpfile")
	if err := os.WriteFile(file1, []byte("content"), 0600); err != nil {
		t.Fatalf("Failed to write to file: %s", err.Error())
	}
	file2 := filepath.Join(dir, "tmpfile2")
	if err := os.WriteFile(file2, []byte("more content"), 0600); err != nil {
		t.Fatalf("Failed to write to file: %s", err.Error())
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}

	file3 := filepath.Join(subdir, "file")
	if err := os.WriteFile(file3, []byte("blaa blaa blaa"), 0600); err != nil {
		t.Fatalf("Failed to write to file: %s", err.Error())
	}
	file4 := filepath.Join(subdir, "file2")
	if err := os.WriteFile(file4, []byte("hello there"), 0600); err != nil {
		t.Fatalf("Failed to write to file: %s", err.Error())
	}

	CheckEncryption = func(filename string) (bool, error) {
		if filename == file2 {
			return false, errExpected
		}

		return false, nil
	}

	checkNames := map[string]bool{file1: true, file3: true, file4: true}

	UploadFile = func(filename, container string, segmentSizeMb uint64, journalNumber string, useOriginal, encrypted bool) error {
		valid, ok := checkNames[filename]
		if !ok {
			t.Errorf("UploadFile() was called with incorrect file name %s", filename)

			return nil
		}
		if !valid {
			t.Errorf("UploadFile() was called too many times with file %s", filename)

			return nil
		}

		checkNames[filename] = false

		return nil
	}

	err = Upload(dir, "container", 300, "", false)
	if err != nil {
		t.Errorf("Function returned unexpected error: %s", err.Error())
	}

	for key := range checkNames {
		if checkNames[key] {
			t.Errorf("UploadFile was not called for file %s", key)
		}
	}
}

func TestUploadFile_FileDetails_Error(t *testing.T) {
	origGetFileDetails := getFileDetails
	defer func() { getFileDetails = origGetFileDetails }()

	getFileDetails = func(filename string, performEncryption bool) (*readCloser, int64, error) {
		if filename == "test-file" {
			return nil, 0, errExpected
		}

		return nil, 0, nil
	}

	errStr := fmt.Sprintf("Failed to get details for file test-file: %s", errExpected.Error())
	if err := UploadFile("test-file", "container", 400, "", false, false); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUploadFile_OriginalFileDetails_Error(t *testing.T) {
	origGetOriginalFileDetails := getOriginalFileDetails
	defer func() { getOriginalFileDetails = origGetOriginalFileDetails }()

	getOriginalFileDetails = func(filename string) (string, int64, error) {
		return "", 0, errExpected
	}

	errStr := fmt.Sprintf("Failed to get details for file ../../test/sample.txt: %s", errExpected.Error())
	if err := UploadFile("../../test/sample.txt.c4gh", "container", 500, "", true, true); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUploadFile(t *testing.T) {
	var tests = []struct {
		testname, container, file string
		journalNumber, manifest   string
		size                      int64
		encrypted, useOrig        bool
		query, manifestQuery      map[string]string
	}{
		{
			"OK_1", "bucket937/dir/subdir/", "../../test/sample.txt.c4gh",
			"", "bucket937_segments/dir/subdir/sample.txt.c4gh/",
			65688, true, true,
			map[string]string{"filename": "dir/subdir/sample.txt.c4gh", "bucket": "bucket937"},
			map[string]string{"filename": "dir/subdir/sample.txt.c4gh", "bucket": "bucket937",
				"encfilesize": "65688", "encchecksum": "da385d93ae510bc91c9c8af7e670ac6f",
				"filesize": "70224", "checksum": "a63d88b82003d96e1659070b253f891a"},
		},
		{
			"OK_2", "bucket790/subdir", "../../test/sample.txt",
			"9", "bucket790_segments/subdir/sample.txt/",
			70224, true, false,
			map[string]string{"filename": "subdir/sample.txt", "bucket": "bucket790", "journal": "9"},
			map[string]string{"filename": "subdir/sample.txt", "bucket": "bucket790", "journal": "9"},
		},
		{
			"OK_3", "bucket784/", "../../test/sample.txt.c4gh",
			"", "bucket784_segments/sample.txt.c4gh.c4gh/",
			65688, false, false,
			map[string]string{"filename": "sample.txt.c4gh.c4gh", "bucket": "bucket784"},
			map[string]string{"filename": "sample.txt.c4gh.c4gh", "bucket": "bucket784"},
		},
	}

	origPut := put
	defer func() { put = origPut }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			count, total := 1, 1
			testTime := time.Now()
			put = func(manifest string, segmentNro, segmentTotal int, uploadData io.Reader, query map[string]string) error {
				if manifest != tt.manifest {
					t.Errorf("Function received incorrect manifest. Expected=%s, received=%s", tt.manifest, manifest)
				}
				if segmentNro != count {
					t.Errorf("Function received incorrect segment number. Expected=%d, received=%d", count, segmentNro)
				}
				if segmentTotal != total {
					t.Errorf("Function received incorrect segment total. Expected=%d, received=%d", total, segmentTotal)
				}

				count, total = -1, -1
				tt.query["timestamp"] = testTime.Format(time.RFC3339)
				tt.manifestQuery["timestamp"] = testTime.Format(time.RFC3339)

				if segmentNro == -1 {
					if !reflect.DeepEqual(query, tt.manifestQuery) {
						t.Errorf("Function received incorrect manifest query\nExpected=%v\nReceived=%v", tt.manifestQuery, query)
					}
				} else if !reflect.DeepEqual(query, tt.query) {
					t.Errorf("Function received incorrect query\nExpected=%v\nReceived=%v", tt.query, query)
				}

				if reflect.ValueOf(uploadData) != reflect.Zero(reflect.TypeOf(uploadData)) {
					if _, err := io.ReadAll(uploadData); err != nil {
						t.Fatalf("Reading file content failed: %s", err.Error())
					}
				}

				return nil
			}

			if err := UploadFile(tt.file, tt.container, 1, tt.journalNumber, tt.useOrig, tt.encrypted); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			}
		})
	}
}

func TestUploadFile_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		count, size      int
	}{
		{"FAIL_1", "Uploading file sample.txt.c4gh failed: ", 1, 8469},
		{"FAIL_2", "Uploading file sample.txt.c4gh failed: ", 2, 114857600},
		{"FAIL_3", "Uploading manifest file failed: ", 3, 164789600},
	}

	origGetFileDetails := getFileDetails
	origPut := put
	defer func() {
		getFileDetails = origGetFileDetails
		put = origPut
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			var file *os.File
			getFileDetails = func(filename string, performEncryption bool) (*readCloser, int64, error) {
				var err error
				file, err = os.Open(filename)
				if err != nil {
					return nil, 0, err
				}

				return &readCloser{file, file, nil, nil}, int64(tt.size), nil
			}

			count := 1
			put = func(manifest string, segmentNro, segmentTotal int, uploadData io.Reader, query map[string]string) error {
				if count == tt.count {
					return errExpected
				}
				count++

				return nil
			}

			errStr := tt.errStr + errExpected.Error()
			err := UploadFile("../../test/sample.txt.c4gh", "bucket684", 100, "", false, true)
			switch {
			case err == nil:
				t.Error("Function did not return error")
			case err.Error() != errStr:
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
			case file != nil:
				if err = file.Close(); err == nil {
					t.Error("File was not closed")
				}
			}
		})
	}
}

func TestUploadFile_FileContent(t *testing.T) {
	var tests = []struct {
		testname, content string
		encrypted         bool
	}{
		{"OK_1", "u89pct87", true},
		{"OK_2", "hiopctfylbkigo", false},
		{"OK_3", "jtxfulvghoi.g oi.rf lg o.fblhoo jihuimgk", true},
	}

	origGetFileDetails := getFileDetails
	origPut := put
	origMiniumSegmentSize := minimumSegmentSize
	defer func() {
		getFileDetails = origGetFileDetails
		put = origPut
		minimumSegmentSize = origMiniumSegmentSize
	}()

	minimumSegmentSize = 10

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			file, err := os.CreateTemp("", "file")
			if err != nil {
				t.Fatalf("Failed to create file: %s", err.Error())
			}

			if _, err := file.WriteString(tt.content); err != nil {
				os.RemoveAll(file.Name())
				t.Fatalf("Failed to write to file: %s", err.Error())
			}
			file.Close()

			getFileDetails = func(filename string, performEncryption bool) (*readCloser, int64, error) {
				file, err := os.Open(filename)
				if err != nil {
					return nil, 0, err
				}

				return &readCloser{file, file, nil, nil}, int64(len(tt.content)), nil
			}

			buf := &bytes.Buffer{}
			put = func(manifest string, segmentNro, segmentTotal int, uploadData io.Reader, query map[string]string) error {
				if segmentNro != -1 {
					if _, err := buf.ReadFrom(uploadData); err != nil {
						return err
					}
				}

				return nil
			}

			filename := file.Name()
			if err := UploadFile(filename, "bucket684", 1, "", false, tt.encrypted); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if tt.content != buf.String() {
				t.Errorf("put() read incorrect content\nExpected=%v\nReceived=%v", []byte(tt.content), buf.Bytes())
			}

			os.RemoveAll(file.Name())
		})
	}
}

func TestUploadFile_Channel_Error(t *testing.T) {
	origGetFileDetails := getFileDetails
	origPut := put
	defer func() {
		getFileDetails = origGetFileDetails
		put = origPut
	}()

	getFileDetails = func(filename string, performEncryption bool) (*readCloser, int64, error) {
		file, err := os.Open(filename)
		if err != nil {
			return nil, 0, err
		}
		errc := make(chan error, 1)
		errc <- errExpected

		return &readCloser{file, file, nil, errc}, 0, nil
	}
	put = func(manifest string, segmentNro, segmentTotal int, uploadData io.Reader, query map[string]string) error {
		return nil
	}

	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	message := "Some content that will never be read"
	if _, err := file.WriteString(message); err != nil {
		t.Fatalf("Failed to write to file: %s", err.Error())
	}

	errStr := "Streaming file failed: " + errExpected.Error()
	if err := UploadFile(file.Name(), "bucket407", 100, "", false, false); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestEncrypt_Error(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errc := make(chan error, 2)
	_, pw := io.Pipe()
	pw.Close()

	go encrypt(file, pw, errc)

	errStr := "io: read/write on closed pipe"
	if err = <-errc; err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}

	var pr *io.PipeReader
	pr, pw = io.Pipe()
	file.Close()

	go encrypt(file, pw, errc)

	if _, err := io.ReadAll(pr); err != nil {
		t.Fatalf("Reading file content failed: %s", err.Error())
	}

	errStr = fmt.Sprintf("read %s: file already closed", file.Name())
	if err = <-errc; err == nil {
		t.Error("Function did not return error, again")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestFileDetails_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	if _, _, err := getFileDetails(file.Name(), false); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestFileDetails_Open_Error(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	if _, _, err := getFileDetails(file.Name(), false); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestFileDetails(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	message := "more_secret_content\n"
	_, err = file.WriteString(message)
	if err != nil {
		t.Fatalf("Failed to write content to file: %s", err.Error())
	}

	testChecksum := "b0c25c08c7ca4496e3bd4b3750ab4fd5"
	rc, size, err := getFileDetails(file.Name(), false)
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if size != 20 {
		t.Errorf("Returned incorrect size. Expected=%d, received=%d", 15, size)
	}
	if bytes, err := io.ReadAll(rc); err != nil {
		t.Fatalf("Reading file content failed: %s", err.Error())
	} else if string(bytes) != message {
		t.Fatalf("Reader returned incorrect message\nExpected=%s\nReceived=%s", message, bytes)
	}
	checksum := hex.EncodeToString(rc.hash.Sum(nil))
	if checksum != testChecksum {
		t.Errorf("Returned incorrect checksum\nExpected=%s\nReceived=%s", testChecksum, checksum)
	}
	if err = rc.Close(); err != nil {
		t.Errorf("Closing file caused error: %s", err.Error())
	}
}

func TestGetFileDetails_Encrypt(t *testing.T) {
	var testBytes int64 = 70512

	publicKey, _, err := keys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Could not generate key pair: %s", err.Error())
	}

	ai.publicKeys = make([][32]byte, 2)
	ai.publicKeys[0] = publicKey
	ai.publicKeys[1] = publicKey

	hash := md5.New()
	rc, bytes, err := getFileDetails("../../test/sample.txt", true)
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	} else if bytes != testBytes {
		t.Errorf("Function returned incorrect bytes\nExpected=%d\nReceived=%d", testBytes, bytes)
	} else if _, err = io.Copy(hash, rc); err != nil {
		t.Fatalf("Error when copying from io.ReadCloser: %s", err.Error())
	}

	checksum := hex.EncodeToString(rc.hash.Sum(nil))
	testChecksum := hex.EncodeToString(hash.Sum(nil))
	if checksum != testChecksum {
		t.Errorf("Returned incorrect checksum\nExpected=%s\nReceived=%s", testChecksum, checksum)
	}
	if err = rc.Close(); err != nil {
		t.Errorf("Closing file caused error: %s", err.Error())
	}
}

func TestGetFileDetails_Crypt4GH(t *testing.T) {
	var tests = []struct {
		testname, message string
	}{
		{"OK_1", "pipe message"},
		{"OK_2", "another_message"},
	}

	publicKey, privateKey, err := keys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Could not generate key pair: %s", err.Error())
	}

	ai.publicKeys = make([][32]byte, 1)
	ai.publicKeys[0] = publicKey

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			file, err := os.CreateTemp("", "file")
			if err != nil {
				t.Fatalf("Failed to create file: %s", err.Error())
			}

			if _, err := file.WriteString(tt.message); err != nil {
				os.RemoveAll(file.Name())
				t.Fatalf("Failed to write to file: %s", err.Error())
			}
			file.Close()

			rc, _, err := getFileDetails(file.Name(), true)
			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else {
				c4ghr, err := streaming.NewCrypt4GHReader(rc, privateKey, nil)
				if err != nil {
					t.Errorf("Failed to create crypt4gh reader: %s", err.Error())
				} else if message, err := io.ReadAll(c4ghr); err != nil {
					t.Errorf("Failed to read from encrypted file: %s", err.Error())
				} else if tt.message != string(message) {
					t.Errorf("Reader received incorrect message\nExpected=%s\nReceived=%s", tt.message, message)
				}
				if err = rc.Close(); err != nil {
					t.Errorf("Closing file caused error: %s", err.Error())
				}
			}

			os.RemoveAll(file.Name())
		})
	}
}

func TestOriginalFileDetails_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	if _, _, err := getOriginalFileDetails(file.Name()); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

type Testfs struct {
	fuse.FileSystemBase

	filename string
}

func (t *Testfs) Getattr(path string, stat *fuse.Stat_t, _ uint64) (errc int) {
	switch path {
	case "/":
		stat.Mode = fuse.S_IFDIR | 0755

		return
	case "/" + t.filename:
		stat.Mode = fuse.S_IFREG | 0755
		stat.Size = int64(10)

		return
	default:
		return -fuse.ENOENT
	}
}

func (t *Testfs) Open(path string, _ int) (errc int, fh uint64) {
	switch path {
	case "/" + t.filename:
		return
	default:
		return -fuse.ENOENT, ^uint64(0)
	}
}

func (t *Testfs) Read(_ string, _ []byte, _ int64, _ uint64) int {
	return -fuse.EIO
}

func TestOriginalFileDetails_CopyFail(t *testing.T) {
	dir, err := os.MkdirTemp("../../test/", "filesystem")
	if err != nil {
		t.Fatalf("Failed to create folder: %s", err.Error())
	}
	defer os.RemoveAll(dir)

	file := "file"
	path := filepath.Join(dir, file)

	testfs := &Testfs{filename: file}
	host := fuse.NewFileSystemHost(testfs)
	go host.Mount(dir, nil)
	defer host.Unmount()

	time.Sleep(2 * time.Second)

	errStr := fmt.Sprintf("read %s: input/output error", path)
	if _, _, err := getOriginalFileDetails(path); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestOriginalFileDetails(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	message := "secret_content\n"
	_, err = file.WriteString(message)
	if err != nil {
		t.Fatalf("Failed to write content to file: %s", err.Error())
	}

	testChecksum := "eae319fc2c45359a335451ba6e2fabe5"
	checksum, size, err := getOriginalFileDetails(file.Name())
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if size != 15 {
		t.Errorf("Returned incorrect size. Expected=%d, received=%d", 15, size)
	}
	if checksum != testChecksum {
		t.Errorf("Returned incorrect checksum\nExpected=%s\nReceived=%s", testChecksum, checksum)
	}
}

func TestCheckEncryption_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("Failed to check if file is encrypted: open %s: no such file or directory", file.Name())
	if _, err := CheckEncryption(file.Name()); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestCheckEncryption(t *testing.T) {
	if encrypted, err := CheckEncryption("../../test/sample.txt.c4gh"); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	} else if !encrypted {
		t.Fatal("Function mistakenly determined file ../../test/sample.txt.enc to not be encrypted")
	}

	if encrypted, err := CheckEncryption("../../test/sample.txt"); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	} else if encrypted {
		t.Fatal("Function mistakenly determined file ../../test/sample.txt to be encrypted")
	}
}

func TestPut(t *testing.T) {
	var tests = []struct {
		testname, token, project string
		segNro, segTotal         int
		query, headers           map[string]string
	}{
		{
			"OK_1", "test_token", "project_9385", 385, 9563,
			map[string]string{"test": "hello", "world": "bye"},
			map[string]string{
				"SDS-Access-Token":  "test_token",
				"SDS-Segment":       "385",
				"SDS-Total-Segment": "9563",
				"Project-Name":      "project_9385",
			},
		},
		{
			"OK_2", "another_token", "", -1, 56,
			map[string]string{"test": "good bye", "parrot": "carrot"},
			map[string]string{
				"SDS-Access-Token":  "another_token",
				"SDS-Segment":       "-1",
				"SDS-Total-Segment": "56",
				"X-Object-Manifest": "bucket/dir",
			},
		},
	}

	origGetSDSToken := api.GetSDSToken
	origMakeRequest := api.MakeRequest
	origProxy := ai.proxy
	origOverridden := ai.overridden
	origProject := ai.project

	defer func() {
		api.GetSDSToken = origGetSDSToken
		api.MakeRequest = origMakeRequest
		ai.proxy = origProxy
		ai.overridden = origOverridden
		ai.project = origProject
	}()

	testURL := "https://example.com"

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetSDSToken = func() string {
				return tt.token
			}
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				if url != testURL+"/airlock" {
					t.Errorf("Function received incorrect url\nExpected=%s\nReceived=%s", testURL+"/airlock", url)
				}
				if !reflect.DeepEqual(query, tt.query) {
					t.Errorf("Function received incorrect query\nExpected=%q\nReceived=%q", tt.query, query)
				}
				if !reflect.DeepEqual(headers, tt.headers) {
					t.Errorf("Function received incorrect headers\nExpected=%q\nReceived=%q", tt.headers, headers)
				}

				return nil
			}

			ai.proxy = testURL
			ai.project = tt.project
			ai.overridden = tt.project != ""
			err := put("bucket/dir", tt.segNro, tt.segTotal, nil, tt.query)
			if err != nil {
				t.Errorf("Function returned error: %s", err.Error())
			}
		})
	}
}

func TestPut_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		err              error
	}{
		{"FAIL_1", errExpected.Error(), errExpected},
		{"FAIL_2", "request body error", &api.RequestError{StatusCode: 500}},
	}

	origGetSDSToken := api.GetSDSToken
	origMakeRequest := api.MakeRequest

	defer func() {
		api.GetSDSToken = origGetSDSToken
		api.MakeRequest = origMakeRequest
	}()

	api.GetSDSToken = func() string {
		return "token"
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				switch v := ret.(type) {
				case *[]byte:
					(*v) = []byte("request body error")

					return tt.err
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *byte[]", reflect.TypeOf(v))
				}
			}

			err := put("bucket6754/dir", 43, 2046, nil, nil)
			if err == nil {
				t.Error("Function did not return error")
			} else if tt.errStr != err.Error() {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}
