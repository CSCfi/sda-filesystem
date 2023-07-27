package airlock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/neicnordic/crypt4gh/keys"
	"github.com/neicnordic/crypt4gh/streaming"
)

var errExpected = errors.New("Expected error for test")

func TestMain(m *testing.M) {
	logs.SetSignal(func(string, []string) {})
	os.Exit(m.Run())
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

func TestPublicKey_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		envErr, reqErr   error
	}{
		{"NO_PROXY", errExpected.Error(), errExpected, nil},
		{
			"REQUEST_FAIL", fmt.Sprintf("Failed to get public key for Airlock: %s", errExpected.Error()),
			nil, errExpected,
		},
	}

	origGetEnv := api.GetEnv
	origMakeRequest := api.MakeRequest
	defer func() {
		api.GetEnv = origGetEnv
		api.MakeRequest = origMakeRequest
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetEnv = func(name string, verifyURL bool) (string, error) {
				return "test_url", tt.envErr
			}
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				return tt.reqErr
			}

			if err := GetPublicKey(); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestPublicKey_Invalid_Key(t *testing.T) {
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

	origGetEnv := api.GetEnv
	origMakeRequest := api.MakeRequest
	defer func() {
		api.GetEnv = origGetEnv
		api.MakeRequest = origMakeRequest
	}()

	var keyStr string
	api.GetEnv = func(name string, verifyURL bool) (string, error) {
		return "proxy_url", nil
	}
	api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
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
			if err := GetPublicKey(); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestPublicKey(t *testing.T) {
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

	origGetEnv := api.GetEnv
	origMakeRequest := api.MakeRequest
	defer func() {
		api.GetEnv = origGetEnv
		api.MakeRequest = origMakeRequest
	}()

	var keyStr string
	api.GetEnv = func(name string, verifyURL bool) (string, error) {
		return "proxy_url", nil
	}
	api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
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
			if err := GetPublicKey(); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if tt.decodedKey != string(ai.publicKey[:]) {
				t.Errorf("Function saved incorrect public key\nExpected=%s\nReceived=%s", tt.decodedKey, ai.publicKey)
			}
		})
	}
}

func TestUpload_FileDetails_Error(t *testing.T) {
	var tests = []struct {
		testname, failOnFile string
		encrypted            bool
	}{
		{"FAIL_1", "enc", true},
		{"FAIL_2", "enc", false},
		{"FAIL_3", "orig", true},
	}

	origGetFileDetails := getFileDetails
	origGetFileDetailsEncrypt := getFileDetailsEncrypt
	defer func() {
		getFileDetails = origGetFileDetails
		getFileDetailsEncrypt = origGetFileDetailsEncrypt
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			getFileDetails = func(filename string) (*os.File, string, int64, error) {
				if filename == tt.failOnFile {
					return nil, "", 0, errExpected
				}

				return nil, "", 0, nil
			}
			getFileDetailsEncrypt = func(filename string) (*os.File, string, int64, error) {
				if filename == tt.failOnFile {
					return nil, "", 0, errExpected
				}

				return nil, "", 0, nil
			}

			errStr := fmt.Sprintf("Failed to get details for file %s: %s", tt.failOnFile, errExpected.Error())
			if err := Upload("enc", "container", 400, "", "orig", tt.encrypted); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
			}
		})
	}
}

func TestUpload(t *testing.T) {
	origGetFileDetails := getFileDetails
	origGetFileDetailsEncrypt := getFileDetailsEncrypt
	origPut := put
	defer func() {
		getFileDetails = origGetFileDetails
		getFileDetailsEncrypt = origGetFileDetailsEncrypt
		put = origPut
	}()

	testContainer := "bucket784/"
	testFile := "../../test/sample.txt.enc"
	testQuery := map[string]string{"filename": "sample.txt.enc.c4gh", "bucket": "bucket784"}

	tempFile, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}

	testTime := time.Now()
	getFileDetails = func(filename string) (*os.File, string, int64, error) {
		return nil, "", 0, errors.New("Should not have called getFileDetails()")
	}
	getFileDetailsEncrypt = func(filename string) (*os.File, string, int64, error) {
		file, err := os.Open(filename)
		if err != nil {
			return tempFile, "", 0, err
		}
		defer file.Close()

		_, err = io.Copy(tempFile, file)
		if err != nil {
			return tempFile, "", 0, err
		}
		if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
			return tempFile, "", 0, err
		}

		return tempFile, "gyov7vclytc6g7x", 958, nil
	}

	count, total := 1, 1
	put = func(manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
		if manifest != "" {
			t.Errorf("Function received manifest %q", manifest)
		}
		if segmentNro == -1 || segment_total == -1 {
			t.Error("Segment number and segment total should not be -1")
		}
		if segmentNro != count {
			t.Errorf("Function received incorrect segment number. Expected=%d, received=%d", count, segmentNro)
		}
		if segment_total != total {
			t.Errorf("Function received incorrect segment total. Expected=%d, received=%d", total, segment_total)
		}

		if count == 1 {
			count, total = -1, -1
		} else {
			count++
		}
		testQuery["timestamp"] = testTime.Format(time.RFC3339)

		if !reflect.DeepEqual(query, testQuery) {
			t.Errorf("Function received incorrect query\nExpected=%v\nReceived=%v", testQuery, query)
		}

		return nil
	}

	if err := Upload(testFile, testContainer, 100, "", "", false); err != nil {
		t.Errorf("Function returned unexpected error: %s", err.Error())
	} else if _, err := os.Stat(tempFile.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("File should not exist")
	}
}

func TestUpload_Encrypted(t *testing.T) {
	var tests = []struct {
		testname, checksum, container, file string
		journalNumber, origFile, manifest   string
		size, total                         int64
		query                               map[string]string
	}{
		{
			"OK_1", "n7cpo5oviuogv78o", "bucket937/dir/subdir/", "../../test/sample.txt.enc",
			"", "../../test/sample.txt", "bucket937/.segments/dir/subdir/sample.txt.enc/", 114857600, 2,
			map[string]string{"filename": "dir/subdir/sample.txt.enc", "bucket": "bucket937", "encfilesize": "114857800",
				"encchecksum": "n7cpo5oviuogv78o78", "filesize": "114858000", "checksum": "n7cpo5oviuogv78o7878"},
		},
		{
			"OK_2", "i8vgyuo8cr7o", "bucket790/subdir", "../../test/sample.txt",
			"9", "", "bucket790/.segments/subdir/sample.txt/", 249715200, 3,
			map[string]string{"filename": "subdir/sample.txt", "bucket": "bucket790", "journal": "9"},
		},
	}

	origGetFileDetails := getFileDetails
	origGetFileDetailsEncrypt := getFileDetailsEncrypt
	origPut := put
	defer func() {
		getFileDetails = origGetFileDetails
		getFileDetailsEncrypt = origGetFileDetailsEncrypt
		put = origPut
	}()

	for i, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			j := i
			getFileDetails = func(filename string) (*os.File, string, int64, error) {
				file, err := os.Open(filename)
				if err != nil {
					return nil, "", 0, err
				}
				j++

				return file, tt.checksum + strings.Repeat("78", j), tt.size + 200*int64(j), nil
			}
			getFileDetailsEncrypt = func(filename string) (*os.File, string, int64, error) {
				return nil, "", 0, errors.New("Should not have called getFileDetailsEncrypt()")
			}

			count := 1
			total := int(tt.total)
			testTime := time.Now()
			put = func(manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
				if manifest != tt.manifest {
					t.Errorf("Function received incorrect manifest. Expected=%s, received=%s", tt.manifest, manifest)
				}
				if segmentNro != count {
					t.Errorf("Function received incorrect segment number. Expected=%d, received=%d", count, segmentNro)
				}
				if segment_total != total {
					t.Errorf("Function received incorrect segment total. Expected=%d, received=%d", total, segment_total)
				}

				if count == int(tt.total) {
					count, total = -1, -1
				} else {
					count++
				}
				tt.query["timestamp"] = testTime.Format(time.RFC3339)

				if !reflect.DeepEqual(query, tt.query) {
					t.Errorf("Function received incorrect query\nExpected=%v\nReceived=%v", tt.query, query)
				}

				return nil
			}

			if err := Upload(tt.file, tt.container, 100, tt.journalNumber, tt.origFile, true); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			}
		})
	}
}

func TestUpload_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		count, size      int
	}{
		{"FAIL_1", "Uploading file sample.txt.enc failed: ", 1, 8469},
		{"FAIL_2", "Uploading file sample.txt.enc failed: ", 2, 114857600},
		{"FAIL_3", "Uploading manifest file failed: ", 3, 164789600},
	}

	origGetFileDetails := getFileDetails
	origGetFileDetailsEncrypt := getFileDetailsEncrypt
	origPut := put
	defer func() {
		getFileDetails = origGetFileDetails
		getFileDetailsEncrypt = origGetFileDetailsEncrypt
		put = origPut
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			var file *os.File
			getFileDetails = func(filename string) (*os.File, string, int64, error) {
				var err error
				file, err = os.Open(filename)
				if err != nil {
					return nil, "", 0, err
				}

				return file, "", int64(tt.size), nil
			}
			getFileDetailsEncrypt = func(filename string) (*os.File, string, int64, error) {
				return nil, "", 0, errors.New("Should not have called getFileDetailsEncrypt()")
			}

			count := 1
			put = func(manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
				if count == tt.count {
					return errExpected
				}
				count++

				return nil
			}

			errStr := tt.errStr + errExpected.Error()
			err := Upload("../../test/sample.txt.enc", "bucket684", 100, "", "", true)
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

func TestUpload_FileContent(t *testing.T) {
	var tests = []struct {
		testname, content string
		encrypted         bool
	}{
		{"OK_1", "u89pct87", true},
		{"OK_2", "hiopctfylbkigo", false},
		{"OK_3", "jtxfulvghoi.g oi.rf lg o.fblhoo jihuimgk", true},
	}

	origGetFileDetails := getFileDetails
	origGetFileDetailsEncrypt := getFileDetailsEncrypt
	origPut := put
	origMiniumSegmentSize := minimumSegmentSize
	defer func() {
		getFileDetails = origGetFileDetails
		getFileDetailsEncrypt = origGetFileDetailsEncrypt
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

			getFileDetails = func(filename string) (*os.File, string, int64, error) {
				file, err := os.Open(filename)
				if err != nil {
					return nil, "", 0, err
				}

				return file, "", int64(len(tt.content)), nil
			}
			getFileDetailsEncrypt = func(filename string) (*os.File, string, int64, error) {
				file, err := os.Open(filename)
				if err != nil {
					return nil, "", 0, err
				}

				return file, "", int64(len(tt.content)), nil
			}

			buf := &bytes.Buffer{}
			put = func(manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
				if segmentNro != -1 {
					if _, err := buf.ReadFrom(upload_data); err != nil {
						return err
					}
				}

				return nil
			}

			filename := file.Name()
			if err := Upload(filename, "bucket684", 1, "", "", tt.encrypted); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if tt.content != buf.String() {
				t.Errorf("put() read incorrect content\nExpected=%v\nReceived=%v", []byte(tt.content), buf.Bytes())
			} else if !tt.encrypted {
				if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("File should not exist")
				}
			}

			os.RemoveAll(filename)
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
	if _, _, _, err := getFileDetails(file.Name()); err == nil {
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

	message := "secret_content\n"
	_, err = file.WriteString(message)
	if err != nil {
		t.Fatalf("Failed to write content to file: %s", err.Error())
	}

	testChecksum := "eae319fc2c45359a335451ba6e2fabe5"
	rc, checksum, size, err := getFileDetails(file.Name())
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if size != 15 {
		t.Errorf("Returned incorrect size. Expected=%d, received=%d", 15, size)
	}
	if checksum != testChecksum {
		t.Errorf("Returned incorrect checksum\nExpected=%s\nReceived=%s", testChecksum, checksum)
	}
	if bytes, err := io.ReadAll(rc); err != nil {
		t.Fatalf("Reading file content failed: %s", err.Error())
	} else if string(bytes) != message {
		t.Fatalf("Reader returned incorrect message\nExpected=%s\nReceived=%s", message, bytes)
	}
}

func TestCheckEncryption_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	if _, err := CheckEncryption(file.Name()); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestCheckEncryption(t *testing.T) {
	if encrypted, err := CheckEncryption("../../test/sample.txt.enc"); err != nil {
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

type mockWriteCloser struct {
	writer io.Writer

	writeErr error
	closeErr error
	data     []byte
}

func (wc *mockWriteCloser) Write(data []byte) (int, error) {
	wc.data = data
	if wc.writer != nil {
		_, err := wc.writer.Write(data)
		if err != nil {
			return 0, err
		}
	}

	return len(data), wc.writeErr
}

func (wc *mockWriteCloser) Close() error {
	return wc.closeErr
}

func TestGetFileDetailsEncrypt_C4ghWriter_Error(t *testing.T) {
	var tests = []struct {
		testname                string
		err, writeErr, closeErr error
	}{
		{"FAIL_1", errExpected, nil, nil},
		{"FAIL_2", nil, errExpected, nil},
		{"FAIL_3", nil, nil, errExpected},
	}

	origNewWriter := newCrypt4GHWriter
	defer func() { newCrypt4GHWriter = origNewWriter }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			newCrypt4GHWriter = func(w io.Writer) (io.WriteCloser, error) {
				return &mockWriteCloser{writeErr: tt.writeErr, closeErr: tt.closeErr}, tt.err
			}

			f, _, _, err := getFileDetailsEncrypt("../../test/sample.txt")
			if err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != errExpected.Error() {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errExpected.Error(), err.Error())
			}
			if f != nil {
				os.RemoveAll(f.Name())
			}
		})
	}
}

func TestGetFileDetailsEncrypt_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	f, _, _, err := getFileDetailsEncrypt(file.Name())
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
	if f != nil {
		os.RemoveAll(f.Name())
	}
}

func TestGetFileDetailsEncrypt(t *testing.T) {
	var tests = []struct {
		testname, message, checksum string
		bytes                       int64
	}{
		{"OK_1", "pipe message", "c4c9d148f5d14dfd9e7a31b4e6ab2f43", 12},
		{"OK_2", "another_message", "5c13b34276b82d5fb39a4e9a99ad182b", 15},
	}

	origNewCrypt4GHWriter := newCrypt4GHWriter
	defer func() { newCrypt4GHWriter = origNewCrypt4GHWriter }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			newCrypt4GHWriter = func(w io.Writer) (io.WriteCloser, error) {
				return &mockWriteCloser{writer: w}, nil
			}

			file, err := os.CreateTemp("", "file")
			if err != nil {
				t.Fatalf("Failed to create file: %s", err.Error())
			}

			if _, err := file.WriteString(tt.message); err != nil {
				os.RemoveAll(file.Name())
				t.Fatalf("Failed to write to file: %s", err.Error())
			}
			file.Close()

			f, checksum, bytes, err := getFileDetailsEncrypt(file.Name())
			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if checksum != tt.checksum {
				t.Errorf("Function returned incorrect checksum\nExpected=%s\nReceived=%s", tt.checksum, checksum)
			} else if bytes != tt.bytes {
				t.Errorf("Function returned incorrect bytes\nExpected=%d\nReceived=%d", tt.bytes, bytes)
			} else if data, err := io.ReadAll(f); err != nil {
				t.Errorf("Error when read from io.ReadCloser: %s", err.Error())
			} else if string(data) != tt.message {
				t.Errorf("io.ReadCloser had incorrect content\nExpected=%s\nReceived=%s", tt.message, data)
			}

			if f == nil {
				t.Error("Function returned nil file")
			} else {
				if err = f.Close(); err != nil {
					t.Errorf("Closing file caused error: %s", err.Error())
				}
				os.RemoveAll(f.Name())
			}

			os.RemoveAll(file.Name())
		})
	}
}

func TestGetFileDetailsEncrypt_Crypt4GH(t *testing.T) {
	var tests = []struct {
		testname, message string
	}{
		{"OK_1", "pipe message"},
		{"OK_2", "another_message"},
	}

	origPublicKey := ai.publicKey
	defer func() { ai.publicKey = origPublicKey }()

	publicKey, privateKey, err := keys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Could not generate key pair: %s", err.Error())
	}

	ai.publicKey = publicKey

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

			f, _, _, err := getFileDetailsEncrypt(file.Name())
			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else {
				c4ghr, err := streaming.NewCrypt4GHReader(f, privateKey, nil)
				if err != nil {
					t.Errorf("Failed to create crypt4gh reader: %s", err.Error())
				} else if message, err := io.ReadAll(c4ghr); err != nil {
					t.Errorf("Failed to read from encrypted file: %s", err.Error())
				} else if tt.message != string(message) {
					t.Errorf("Reader received incorrect message\nExpected=%s\nReceived=%s", tt.message, message)
				}
			}

			if f == nil {
				t.Error("Function returned nil file")
			} else {
				if err = f.Close(); err != nil {
					t.Errorf("Closing file caused error: %s", err.Error())
				}
				os.RemoveAll(f.Name())
			}

			os.RemoveAll(file.Name())
		})
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
