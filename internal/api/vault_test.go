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
	origMakeRequest := MakeRequest
	origKeyName := ai.vi.keyName
	defer func() {
		whitelistKey = origWhitelistKey
		deleteWhitelistedKey = origDeleteWhitelistedKey
		MakeRequest = origMakeRequest
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
			whitelistKey = func(rep string) error {
				if rep != "some repo" {
					t.Errorf("whitelistKey() received invalis repository. Expected=some repo, received=%v", rep)
				}
				whitelisted = true

				return tt.whitelistErr
			}
			deleteWhitelistedKey = func(rep string) error {
				if rep != "some repo" {
					t.Errorf("deleteWhitelistedKey() received invalis repository. Expected=some repo, received=%v", rep)
				}
				if !whitelisted {
					t.Errorf("deleteWhitelistedKey() was called before key was whitelisted")
				}
				whitelisted = false

				return tt.deleteErr
			}
			MakeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
				if method != "GET" {
					return fmt.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				if path != "/desktop/file-headers" {
					return fmt.Errorf("Request has incorrect path\nExpected=/desktop/fileheaders\nReceived=%v", path)
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
