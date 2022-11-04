package airlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
)

var errExpected = errors.New("Expected error for test")

func TestMain(m *testing.M) {
	logs.SetSignal(func(i int, s []string) {})
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
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\nR29vZCBtb3JuaW5nIHN1bnNoaW5lIQ==\n-----END CRYPT4GH PUBLIC KEY-----",
			"Invalid length of decoded public key (22)",
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
		testname, key, decoded_key string
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
			} else if tt.decoded_key != string(ai.publicKey[:]) {
				t.Errorf("Function saved incorrect public key\nExpected=%s\nReceived=%s", tt.decoded_key, ai.publicKey)
			}
		})
	}
}

/*func TestUpload_Encrypt_Error(t *testing.T) {
	origEncrypt := encrypt
	defer func() { encrypt = origEncrypt }()

	encrypt = func(in_filename, out_filename string) error {
		return errExpected
	}

	errStr := fmt.Sprintf("Failed to encrypt file orig: %s", errExpected.Error())
	if err := Upload("orig", "enc", "container", "", 4000, true); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUpload_FileDetails_Error(t *testing.T) {
	var tests = []struct {
		testname, failOnFile string
	}{
		{"FAIL_1", "enc"},
		{"FAIL_2", "orig"},
	}

	origEncrypt := encrypt
	origGetFileDetails := getFileDetails
	defer func() {
		encrypt = origEncrypt
		getFileDetails = origGetFileDetails
	}()

	encrypt = func(in_filename, out_filename string) error {
		return errors.New("Should not have called function")
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			getFileDetails = func(filename string) (string, int64, error) {
				if filename == tt.failOnFile {
					return "", 0, errExpected
				}
				return "smthsmth", 40, nil
			}

			errStr := fmt.Sprintf("Failed to get details for file %s: %s", tt.failOnFile, errExpected.Error())
			if err := Upload("orig", "enc", "container", "", 400, false); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
			}
		})
	}
}

func TestUpload_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	origEncrypt := encrypt
	origGetFileDetails := getFileDetails
	defer func() {
		encrypt = origEncrypt
		getFileDetails = origGetFileDetails
	}()

	encrypt = func(in_filename, out_filename string) error {
		return nil
	}
	getFileDetails = func(filename string) (string, int64, error) {
		return "smthsmth", 40, nil
	}

	errStr := fmt.Sprintf("Cannot open encypted file: open %s: no such file or directory", file.Name())
	if err := Upload("orig", file.Name(), "bucket473", "", 500, true); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUpload(t *testing.T) {
	var tests = []struct {
		testname, checksum, container  string
		journal_number, origFile, file string
		size, total                    int64
		query                          map[string]string
	}{
		{
			"OK_1", "gyov7vclytc6g7x", "bucket784/", "", "", "../../test/sample.txt.enc", 958, 1,
			map[string]string{"filename": "sample.txt.enc", "bucket": "bucket784"},
		},
		{
			"OK_2", "n7cpo5oviuogv78o", "bucket937/dir/subdir/", "",
			"../../test/sample.txt", "../../test/sample.txt.enc", 114857600, 2,
			map[string]string{"filename": "dir/subdir/sample.txt.enc", "bucket": "bucket937", "encfilesize": "114857600",
				"encchecksum": "n7cpo5oviuogv78o", "filesize": "114857800", "checksum": "n7cpo5oviuogv78ohmmhmm"},
		},
		{
			"OK_3", "i8vgyuo8cr7o", "bucket790/subdir", "9", "", "../../test/sample.txt", 930485, 1,
			map[string]string{"filename": "subdir/sample.txt", "bucket": "bucket790", "journal": "9"},
		},
	}

	origEncrypt := encrypt
	origGetFileDetails := getFileDetails
	origPut := put
	origCurrentTime := currentTime
	defer func() {
		encrypt = origEncrypt
		getFileDetails = origGetFileDetails
		put = origPut
		currentTime = origCurrentTime
	}()

	encrypt = func(in_filename, out_filename string) error {
		return nil
	}

	for i, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			testTime := time.Now()
			currentTime = func() time.Time {
				return testTime
			}
			getFileDetails = func(filename string) (string, int64, error) {
				if filename == tt.file {
					return tt.checksum, tt.size, nil
				}
				return tt.checksum + strings.Repeat("hmm", i+1), tt.size*int64(i) + 200, nil
			}

			count := 1
			total := int(tt.total)
			put = func(url, container string, segment_nro, segment_total int, upload_dir string, upload_data io.Reader, query map[string]string) error {
				if container != tt.query["bucket"] {
					t.Errorf("Function received incorrect container. Expected=%s, received=%s", tt.query["bucket"], container)
				}
				if (segment_nro == -1 || segment_total == -1) && tt.total == 1 {
					t.Error("Segment number and segment total should not be -1")
				}
				if segment_nro != count {
					t.Errorf("Function received incorrect segment number. Expected=%d, received=%d", count, segment_nro)
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

			if err := Upload(tt.origFile, tt.file, tt.container, tt.journal_number, 100, true); err != nil {
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
		{"FAIL_1", "Uploading file failed: ", 1, 8469},
		{"FAIL_2", "Uploading file failed: ", 2, 114857600},
		{"FAIL_3", "Uploading manifest file failed: ", 3, 164789600},
	}

	origEncrypt := encrypt
	origGetFileDetails := getFileDetails
	origPut := put
	defer func() {
		encrypt = origEncrypt
		getFileDetails = origGetFileDetails
		put = origPut
	}()

	encrypt = func(in_filename, out_filename string) error {
		return nil
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			getFileDetails = func(filename string) (string, int64, error) {
				return "", int64(tt.size), nil
			}

			count := 1
			put = func(url, container string, segment_nro, segment_total int, upload_dir string, upload_data io.Reader, query map[string]string) error {
				if count == tt.count {
					return errExpected
				}
				count++
				return nil
			}

			errStr := tt.errStr + errExpected.Error()
			if err := Upload("", "../../test/sample.txt.enc", "bucket684", "", 100, false); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
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

	origEncrypt := encrypt
	origGetFileDetails := getFileDetails
	origPut := put
	origMiniumSegmentSize := minimumSegmentSize
	defer func() {
		encrypt = origEncrypt
		getFileDetails = origGetFileDetails
		put = origPut
		minimumSegmentSize = origMiniumSegmentSize
	}()

	minimumSegmentSize = 10
	encrypt = func(in_filename, out_filename string) error {
		return nil
	}

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

			getFileDetails = func(filename string) (string, int64, error) {
				return "", int64(len(tt.content)), nil
			}

			buf := &bytes.Buffer{}
			put = func(url, container string, segment_nro, segment_total int, upload_dir string, upload_data io.Reader, query map[string]string) error {
				if segment_nro != -1 {
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

func TestFileDetails(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	_, err = file.WriteString("secret_content\n")
	if err != nil {
		t.Fatalf("Failed to write content to file: %s", err.Error())
	}

	checksum, size, err := getFileDetails(file.Name())
	if err != nil {
		t.Fatal("Function returned unexpected error")
	}
	if size != 15 {
		t.Fatalf("Returned incorrect size. Expected=%d, received=%d", 15, size)
	}
	if checksum != "eae319fc2c45359a335451ba6e2fabe5" {
		t.Fatalf("Returned incorrect checksum\nExpected=%s\nReceived=%s", "eae319fc2c45359a335451ba6e2fabe5", checksum)
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

func TestCheckEncryption_Not_Encrypted(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(file.Name())

	if encrypted, err := CheckEncryption(file.Name()); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	} else if encrypted {
		t.Fatal("Function mistakenly determined file to be encrypted")
	}
}

func TestCheckEncryption_Encrypted(t *testing.T) {
	if encrypted, err := CheckEncryption("../../test/sample.txt.enc"); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	} else if !encrypted {
		t.Fatal("Function mistakenly determined file to be not encrypted")
	}
}

func TestEncrypt_File_Error(t *testing.T) {
	in_file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(in_file.Name())

	out_file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(out_file.Name())

	var tests = []struct {
		testname, errStr  string
		in_perm, out_perm fs.FileMode
	}{
		{"NO_IN_FILE_PERMISSION", fmt.Sprintf("open %s: permission denied", in_file.Name()), 0333, 0666},
		{"NO_OUT_FILE_PEEMISSION", fmt.Sprintf("open %s: permission denied", out_file.Name()), 0666, 0333},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			if err = os.Chmod(in_file.Name(), os.FileMode(tt.in_perm)); err != nil {
				t.Errorf("Changing permission bits failed: %s", err.Error())
			}
			if err = os.Chmod(out_file.Name(), os.FileMode(tt.out_perm)); err != nil {
				t.Errorf("Changing permission bits failed: %s", err.Error())
			}

			if err := encrypt(in_file.Name(), out_file.Name()); err == nil {
				t.Error("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestEncrypt_Fail(t *testing.T) {
	out_file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(out_file.Name())

	origPublicLey := ai.publicKey
	defer func() { ai.publicKey = origPublicLey }()

	ai.publicKey = [32]byte{}
	if err := encrypt("../../test/sample.txt", out_file.Name()); err == nil {
		t.Error("Function did not return error")
	}
}

func TestEncrypt(t *testing.T) {
	out_file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	defer os.RemoveAll(out_file.Name())

	origPublicLey := ai.publicKey
	defer func() { ai.publicKey = origPublicLey }()

	publicKey, privateKey, err := keys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Could not generate key pair: %s", err.Error())
	}

	ai.publicKey = publicKey
	file := "../../test/sample.txt"
	if err := encrypt(file, out_file.Name()); err != nil {
		t.Fatal("Function did not return error")
	}
	if encrypted, _ := CheckEncryption(out_file.Name()); !encrypted {
		t.Fatal("Function failed to encrypt file")
	}

	c4ghr, err := streaming.NewCrypt4GHReader(out_file, privateKey, nil)
	if err != nil {
		t.Fatalf("Failed to create crypt4gh reader: %s", err.Error())
	}

	encBytes, err := io.ReadAll(c4ghr)
	if err != nil {
		t.Fatalf("Failed to read from encrypted file: %s", err.Error())
	}

	in_file, err := os.Open(file)
	if err != nil {
		t.Fatalf("Failed to open file: %s", err.Error())
	}
	defer in_file.Close()

	decBytes, err := io.ReadAll(in_file)
	if err != nil {
		t.Fatalf("Failed to read from original file: %s", err.Error())
	}

	if !reflect.DeepEqual(encBytes, decBytes) {
		t.Error("Could not recreate decrypted file from encrypted file")
	}
}

func TestGetFileDetailsAndEncrypt(t *testing.T) {
	origPublicLey := ai.publicKey
	defer func() { ai.publicKey = origPublicLey }()

	publicKey, privateKey, err := keys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Could not generate key pair: %s", err.Error())
	}

	ai.publicKey = publicKey
	file := "../../test/sample.txt"
	if _, _, _, err := getFileDetailsAndEncrypt("../../test/sample.txt"); err != nil {
		t.Errorf("Function returned unexpected error: %s", err.Error())
	}
}*/

func TestGetFileDetailsAndEncrypt_C4ghWriterError(t *testing.T) {
	ai.publicKey = [32]byte{}
	if _, _, _, err := getFileDetailsAndEncrypt("../../test/sample.txt"); err == nil {
		t.Errorf("Function did not return error")
	}
}

func TestGetFileDetailsAndEncrypt_NoFile(t *testing.T) {
	file, err := os.CreateTemp("", "file")
	if err != nil {
		t.Fatalf("Failed to create file: %s", err.Error())
	}
	os.RemoveAll(file.Name())

	errStr := fmt.Sprintf("open %s: no such file or directory", file.Name())
	if _, _, _, err := getFileDetailsAndEncrypt(file.Name()); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
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

	testUrl := "https://example.com"

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetSDSToken = func() string {
				return tt.token
			}
			api.MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				if url != testUrl {
					t.Errorf("Function received incorrect url\nExpected=%s\nReceived=%s", testUrl, url)
				}
				if !reflect.DeepEqual(query, tt.query) {
					t.Errorf("Function received incorrect query\nExpected=%q\nReceived=%q", tt.query, query)
				}
				if !reflect.DeepEqual(headers, tt.headers) {
					t.Errorf("Function received incorrect headers\nExpected=%q\nReceived=%q", tt.headers, headers)
				}
				return nil
			}

			err := put(testUrl, "bucket/dir", tt.segNro, tt.segTotal, nil, tt.query)
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
