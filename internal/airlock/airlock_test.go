package airlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"testing"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/neicnordic/crypt4gh/keys"
	"github.com/neicnordic/crypt4gh/streaming"
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
			"-----BEGIN CRYPT4GH PUBLIC KEY-----\n\nR29vZCBtb3JuaW5nIHN1bnNoaW5lISBnaDZ4Mzl4dGs=\n\n\n-----END CRYPT4GH PUBLIC KEY-----\n",
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

			err := put(testUrl, "bucket", tt.segNro, tt.segTotal, "dir", nil, tt.query)
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

			errStr := fmt.Sprintf("Failed to upload data to container bucket6754: %s", tt.errStr)
			err := put("google.com", "bucket6754", 43, 2046, "dir", nil, nil)
			if err == nil {
				t.Error("Function did not return error")
			} else if errStr != err.Error() {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
			}
		})
	}
}
