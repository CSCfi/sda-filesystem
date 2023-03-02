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
	if _, err := IsProjectManager(); err == nil {
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

			if _, err := IsProjectManager(); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestIsProjectManager(t *testing.T) {
	var tests = []struct {
		testname  string
		data      string
		isManager bool
	}{
		{"OK_1", "wrong_project another_project pr726", false},
		{"OK_2", "wrong_project project_4726 cupcake", true},
	}

	origInfoFile := infoFile
	origMakeRequest := api.MakeRequest
	defer func() {
		infoFile = origInfoFile
		api.MakeRequest = origMakeRequest
	}()

	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	data := map[string]string{"userinfo_endpoint": "test_point2", "login_aud": "project_4726"}
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

			if isManager, err := IsProjectManager(); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if tt.isManager != isManager {
				t.Errorf("Function returned incorrect manager status. Expected=%v, received=%v", tt.isManager, isManager)
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
		encrypt              bool
	}{
		{"FAIL_1", "enc", false},
		{"FAIL_2", "orig", true},
		{"FAIL_3", "orig", false},
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
			getFileDetailsEncrypt = func(filename string) (*readCloser, string, int64, error) {
				if filename == tt.failOnFile {
					return nil, "", 0, errExpected
				}

				return nil, "", 0, nil
			}

			errStr := fmt.Sprintf("Failed to get details for file %s: %s", tt.failOnFile, errExpected.Error())
			if err := Upload("orig", "enc", "container", "", 400, tt.encrypt); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
			}
		})
	}
}

func TestUpload(t *testing.T) {
	var tests = []struct {
		testname, checksum, container, manifest string
		journalNumber, origFile, file           string
		size, total                             int64
		encrypt                                 bool
		query                                   map[string]string
	}{
		{
			"OK_1", "gyov7vclytc6g7x", "bucket784/", "",
			"", "", "../../test/sample.txt.enc", 958, 1, false,
			map[string]string{"filename": "sample.txt.enc", "bucket": "bucket784"},
		},
		{
			"OK_2", "n7cpo5oviuogv78o", "bucket937/dir/subdir/", "bucket937/.segments/dir/subdir/sample.txt.enc/",
			"", "../../test/sample.txt", "../../test/sample.txt.enc", 114857600, 2, true,
			map[string]string{"filename": "dir/subdir/sample.txt.enc", "bucket": "bucket937", "encfilesize": "114858600",
				"encchecksum": "n7cpo5oviuogv78o5050", "filesize": "114858000", "checksum": "n7cpo5oviuogv78o7878"},
		},
		{
			"OK_3", "i8vgyuo8cr7o", "bucket790/subdir", "bucket790/.segments/subdir/sample.txt/",
			"9", "", "../../test/sample.txt", 249715200, 3, false,
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
			testTime := time.Now()
			var file1, file2 *os.File
			getFileDetails = func(filename string) (*os.File, string, int64, error) {
				var err error
				file1, err = os.Open(filename)
				if err != nil {
					return nil, "", 0, err
				}

				return file1, tt.checksum + strings.Repeat("78", i+1), tt.size + 200*int64(i+1), nil
			}
			getFileDetailsEncrypt = func(filename string) (*readCloser, string, int64, error) {
				if !tt.encrypt {
					return nil, "", 0, errors.New("Should not have called getFileDetailsEncrypt()")
				}
				var err error
				file2, err = os.Open(filename)
				if err != nil {
					return nil, "", 0, err
				}

				return &readCloser{file2, file2, nil}, tt.checksum + strings.Repeat("50", i+1), tt.size + 500*int64(i+1), nil
			}

			count := 1
			total := int(tt.total)
			put = func(url, manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
				if manifest != tt.manifest {
					t.Errorf("Function received incorrect manifest. Expected=%s, received=%s", tt.manifest, manifest)
				}
				if (segmentNro == -1 || segment_total == -1) && tt.total == 1 {
					t.Error("Segment number and segment total should not be -1")
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

			if err := Upload(tt.origFile, tt.file, tt.container, tt.journalNumber, 100, tt.encrypt); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else {
				if file1 != nil {
					if err = file1.Close(); err == nil {
						t.Error("File was not closed")
					}
				}
				if file2 != nil {
					if err = file2.Close(); err == nil {
						t.Error("Unencrypted file was not closed")
					}
				}
			}
		})
	}
}

func TestUpload_Error(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		count, size      int
	}{
		{"FAIL_1", "Uploading file failed: ", 1, 8469},
		{"FAIL_2", "Uploading file failed: ", 2, 114857600},
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
			getFileDetailsEncrypt = func(filename string) (*readCloser, string, int64, error) {
				return nil, "", 0, errors.New("Should not have called getFileDetailsEncrypt()")
			}

			count := 1
			put = func(url, manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
				if count == tt.count {
					return errExpected
				}
				count++

				return nil
			}

			errStr := tt.errStr + errExpected.Error()
			err := Upload("", "../../test/sample.txt.enc", "bucket684", "", 100, false)
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
	}{
		{"OK_1", "u89pct87"},
		{"OK_2", "hiopctfylbkigo"},
		{"OK_3", "jtxfulvghoi.g oi.rf lg o.fblhoo jihuimgk"},
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
			if _, err := file.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to file: %s", err.Error())
			}

			getFileDetails = func(filename string) (*os.File, string, int64, error) {
				file, err := os.Open(filename)
				if err != nil {
					return nil, "", 0, err
				}

				return file, "", int64(len(tt.content)), nil
			}

			buf := &bytes.Buffer{}
			put = func(url, manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
				if segmentNro != -1 {
					if _, err := buf.ReadFrom(upload_data); err != nil {
						return err
					}
				}

				return nil
			}

			if err := Upload("", file.Name(), "bucket684", "", 1, false); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if tt.content != buf.String() {
				t.Errorf("put() read incorrect content\nExpected=%v\nReceived=%v", []byte(tt.content), buf.Bytes())
			}
		})
	}
}

func TestUpload_Channel_Error(t *testing.T) {
	origGetFileDetails := getFileDetails
	origGetFileDetailsEncrypt := getFileDetailsEncrypt
	origPut := put
	defer func() {
		getFileDetails = origGetFileDetails
		getFileDetailsEncrypt = origGetFileDetailsEncrypt
		put = origPut
	}()

	getFileDetailsEncrypt = func(filename string) (*readCloser, string, int64, error) {
		file, err := os.Open(filename)
		if err != nil {
			return nil, "", 0, err
		}
		errc := make(chan error, 1)
		errc <- errExpected

		return &readCloser{file, file, errc}, "", 0, nil
	}
	getFileDetails = func(filename string) (*os.File, string, int64, error) {
		file, err := os.Open(filename)
		if err != nil {
			return nil, "", 0, err
		}

		return file, "", 0, nil
	}
	put = func(url, manifest string, segmentNro, segment_total int, upload_data io.Reader, query map[string]string) error {
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
	if err := Upload(file.Name(), file.Name()+".c4gh", "bucket407", "", 100, true); err == nil {
		t.Error("Function did not return error")
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
	writeErr error
	closeErr error
	data     []byte
}

func (wc *mockWriteCloser) Write(data []byte) (n int, err error) {
	wc.data = data

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

			if _, _, _, err := getFileDetailsEncrypt("../../test/sample.txt"); err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != errExpected.Error() {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errExpected.Error(), err.Error())
			}
		})
	}
}

func TestGetFileDetailsEncrypt_Hash_Error(t *testing.T) {
	origEncrypt := encrypt
	defer func() { encrypt = origEncrypt }()

	encrypt = func(file *os.File, pw *io.PipeWriter, errc chan error) {
		pw.CloseWithError(errExpected)
		errc <- nil
	}

	if _, _, _, err := getFileDetailsEncrypt("../../test/sample.txt"); err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errExpected.Error() {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errExpected.Error(), err.Error())
	}
}

func TestGetFileDetailsEncrypt_Seek_Error(t *testing.T) {
	origEncrypt := encrypt
	defer func() { encrypt = origEncrypt }()

	encrypt = func(file *os.File, pw *io.PipeWriter, errc chan error) {
		pw.Close()
		file.Close()
		errc <- nil
	}

	errStr := "seek ../../test/sample.txt: file already closed"
	if _, _, _, err := getFileDetailsEncrypt("../../test/sample.txt"); err == nil {
		t.Errorf("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestGetFileDetailsEncrypt_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	if _, _, _, err := getFileDetailsEncrypt(file.Name()); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
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

	origEncrypt := encrypt
	defer func() { encrypt = origEncrypt }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			encrypt = func(file *os.File, pw *io.PipeWriter, errc chan error) {
				_, err := pw.Write([]byte(tt.message))
				pw.Close()
				errc <- err
			}

			if rc, checksum, bytes, err := getFileDetailsEncrypt("../../test/sample.txt"); err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if checksum != tt.checksum {
				t.Errorf("Function returned incorrect checksum\nExpected=%s\nReceived=%s", tt.checksum, checksum)
			} else if bytes != tt.bytes {
				t.Errorf("Function returned incorrect checksum\nExpected=%d\nReceived=%d", tt.bytes, bytes)
			} else if data, err := io.ReadAll(rc); err != nil {
				t.Errorf("Error when read from io.ReadCloser: %s", err.Error())
			} else if string(data) != tt.message {
				t.Errorf("io.ReadCloser had incorrect content\nExpected=%s\nReceived=%s", tt.message, data)
			} else if err = <-rc.errc; err != nil {
				t.Errorf("Channel returned unexpected error: %s", err.Error())
			} else if err = rc.Close(); err != nil {
				t.Errorf("Closing file caused error: %s", err.Error())
			}
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

	origEncrypt := encrypt
	origPublicLey := ai.publicKey
	defer func() {
		encrypt = origEncrypt
		ai.publicKey = origPublicLey
	}()

	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	publicKey, privateKey, err := keys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Could not generate key pair: %s", err.Error())
	}

	ai.publicKey = publicKey

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			if err = file.Truncate(0); err != nil {
				t.Errorf("Could not truncate file: %s", err.Error())
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Errorf("Could not change file offset: %s", err.Error())
			}
			if _, err := file.WriteString(tt.message); err != nil {
				t.Fatalf("Failed to write to file: %s", err.Error())
			}

			if rc, _, _, err := getFileDetailsEncrypt(file.Name()); err != nil {
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
		})
	}
}

func TestPut(t *testing.T) {
	var tests = []struct {
		testname, token  string
		segNro, segTotal int
		query, headers   map[string]string
	}{
		{
			"OK_1", "test_token", 385, 9563,
			map[string]string{"test": "hello", "world": "bye"},
			map[string]string{"SDS-Access-Token": "test_token", "SDS-Segment": "385", "SDS-Total-Segment": "9563"},
		},
		{
			"OK_2", "another_token", -1, 56,
			map[string]string{"test": "good bye", "parrot": "carrot"},
			map[string]string{"SDS-Access-Token": "another_token", "SDS-Segment": "-1", "SDS-Total-Segment": "56", "X-Object-Manifest": "bucket/dir"},
		},
	}

	origGetSDSToken := api.GetSDSToken
	origMakeRequest := api.MakeRequest

	defer func() {
		api.GetSDSToken = origGetSDSToken
		api.MakeRequest = origMakeRequest
	}()

	testURL := "https://example.com"

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetSDSToken = func() string {
				return tt.token
			}
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				if url != testURL {
					t.Errorf("Function received incorrect url\nExpected=%s\nReceived=%s", testURL, url)
				}
				if !reflect.DeepEqual(query, tt.query) {
					t.Errorf("Function received incorrect query\nExpected=%q\nReceived=%q", tt.query, query)
				}
				if !reflect.DeepEqual(headers, tt.headers) {
					t.Errorf("Function received incorrect headers\nExpected=%q\nReceived=%q", tt.headers, headers)
				}

				return nil
			}

			err := put(testURL, "bucket/dir", tt.segNro, tt.segTotal, nil, tt.query)
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

			err := put("google.com", "bucket6754/dir", 43, 2046, nil, nil)
			if err == nil {
				t.Error("Function did not return error")
			} else if tt.errStr != err.Error() {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}
