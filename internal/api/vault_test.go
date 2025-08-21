package api

import (
	"fmt"
	"io"
	"reflect"
	"testing"
)

func TestGetHeaders(t *testing.T) {
	origWhitelistKey := whitelistKey
	origDeleteWhitelistedKey := deleteWhitelistedKey
	origMakeRequest := makeRequest
	origKeyName := ai.vi.keyName
	defer func() {
		whitelistKey = origWhitelistKey
		deleteWhitelistedKey = origDeleteWhitelistedKey
		makeRequest = origMakeRequest
		ai.vi.keyName = origKeyName
	}()

	batch := make(BatchHeaders)
	batch["bucket_1"] = make(map[string]VaultHeaderVersions)

	batch["bucket_1"]["kansio/file_1"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"1": {Header: "yvdyviditybf"},
		},
		LatestVersion: 1,
	}
	batch["bucket_1"]["kansio/file_2"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"3": {Header: "hbfyucdtkyv"},
			"4": {Header: "hubftiuvfti"},
		},
		LatestVersion: 4,
	}

	expectedBody := `{
		"batch": "eyJoZWxsbyI6WyIqKiJdLCJodWxsbyI6WyIqKiJdfQ==",
		"service": "data-gateway",
		"key": "some-key"
	}`
	ai.vi.keyName = "some-key"
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, errStr                    string
		whitelistErr, deleteErr, requestErr error
	}{
		{
			"OK", "", nil, nil, nil,
		},
		{
			"FAIL_WHITELIST", "failed to whitelist public key: " + errExpected.Error(), errExpected, nil, nil,
		},
		{
			"FAIL_DELETE", "", nil, errExpected, nil,
		},
		{
			"FAIL_REQUEST", "request failed: " + errExpected.Error(), nil, nil, errExpected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			whitelisted := false
			whitelistKey = func() error {
				whitelisted = true

				return tt.whitelistErr
			}
			deleteWhitelistedKey = func() error {
				if !whitelisted {
					t.Errorf("deleteWhitelistedKey() was called before key was whitelisted")
				}
				whitelisted = false

				return tt.deleteErr
			}
			makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
				if method != "GET" {
					return fmt.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				if path != "/headers-endpoint" {
					return fmt.Errorf("Request has incorrect path\nExpected=/headers-endpoint\nReceived=%v", path)
				}
				body, err := io.ReadAll(reqBody)
				if err != nil {
					return fmt.Errorf("Failed to read body: %w", err)
				}
				if !whitelisted {
					t.Errorf("Key is not whitelisted")
				}

				if string(body) != expectedBody {
					return fmt.Errorf("Request has incorrect body\nExpected=%s\nReceived=%s", expectedBody, string(body))
				}

				switch v := ret.(type) {
				case *vaultResponse:
					v.Data = batch
					v.Warnings = []string{"No matches found for bucket invisible-bucket", "Bad bucket warning"}

					return tt.requestErr
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *vaultResponse", reflect.TypeOf(v))
				}
			}

			data, err := GetHeaders("some repo", []Metadata{{Name: "hello"}, {Name: "hullo"}})
			switch {
			case tt.errStr != "":
				if err == nil {
					t.Errorf("Function did not return error")
				} else if err.Error() != tt.errStr {
					t.Errorf("Function returned incorrect error\nExpected=%q\nReceived=%q", tt.errStr, err.Error())
				}
			case err != nil:
				t.Errorf("Function returned unexpected error: %s", err.Error())
			case !reflect.DeepEqual(batch, data):
				t.Errorf("Incorrect response body\nExpected=%v\nReceived=%v", batch, data)
			}
		})
	}
}

func TestPublicKey(t *testing.T) {
	var tests = []struct {
		testname, key64 string
		publicKey       [32]byte
	}{
		{
			"OK_1", "BzyzezEMAx5f38/zGzc/zD863j/nHFheRH9TM/eXIjs=",
			[32]byte{7, 60, 179, 123, 49, 12, 3, 30, 95, 223, 207, 243, 27, 55, 63, 204, 63, 58, 222, 63, 231, 28, 88, 94, 68, 127, 83, 51, 247, 151, 34, 59},
		},
		{
			"OK_3", "HtEvWvRi9e0QvKeWTfU/QTCRR5Wm5rSlc8v+jxjNRXU=",
			[32]byte{30, 209, 47, 90, 244, 98, 245, 237, 16, 188, 167, 150, 77, 245, 63, 65, 48, 145, 71, 149, 166, 230, 180, 165, 115, 203, 254, 143, 24, 205, 69, 117},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	ai.hi.endpoints = testConfig

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(method, path string, query, headers map[string]string, body io.Reader, ret any) error {
				if method != "GET" {
					return fmt.Errorf("request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				switch v := ret.(type) {
				case *keyResponse:
					switch path {
					case "/project-key-endpoint":
						v.Key64 = fmt.Sprintf("-----BEGIN CRYPT4GH PUBLIC KEY-----\n%s\n-----END CRYPT4GH PUBLIC KEY-----", tt.key64)
					default:
						return fmt.Errorf("request has incorrect path %v", path)
					}

					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *keyResponse", reflect.TypeOf(v))
				}
			}

			key, err := GetPublicKey()
			if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(key, tt.publicKey) {
				t.Errorf("Function saved incorrect public key\nExpected=%v\nReceived=%v", tt.publicKey, key)
			}
		})
	}
}

func TestGetPublicKey_InvalidKey(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	makeRequest = func(method, path string, query, headers map[string]string, body io.Reader, ret any) error {
		switch v := ret.(type) {
		case *keyResponse:
			v.Key64 = "-----BEGIN CRYPT4GH PUBLIC KEY-----\nSGVsbG8sIHdvcmxkIQ==\n-----END CRYPT4GH PUBLIC KEY-----"

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *keyResponse", reflect.TypeOf(v))
		}
	}

	errStr := "Unsupported key file format"
	if _, err := GetPublicKey(); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}
