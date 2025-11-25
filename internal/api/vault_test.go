package api

import (
	"fmt"
	"io"
	"maps"
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

	expectedData := make(BatchHeaders)
	expectedData["bucket_1"] = make(map[string]VaultHeaderVersions)
	expectedData["shared-bucket"] = make(map[string]VaultHeaderVersions)

	expectedData["bucket_1"]["kansio/file_1"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"1": {Header: "yvdyviditybf"},
		},
		LatestVersion: 1,
	}
	expectedData["bucket_1"]["kansio/file_2"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"3": {Header: "hbfyucdtkyv"},
			"4": {Header: "hubftiuvfti"},
		},
		LatestVersion: 4,
	}
	expectedData["shared-bucket"]["dir/obj.c4gh"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"1": {Header: "gyutvrtivru"},
		},
		LatestVersion: 1,
	}

	batch := make(map[string]BatchHeaders)
	batch["my-project"] = make(BatchHeaders)
	batch["sharing-project-1"] = make(BatchHeaders)
	batch["sharing-project-2"] = make(BatchHeaders)

	batch["my-project"]["bucket_1"] = expectedData["bucket_1"]
	batch["sharing-project-1"]["shared-bucket"] = expectedData["shared-bucket"]

	expectedBody := map[string]string{
		"my-project": `{
		"batch": "eyJidWNrZXRfMSI6WyIqKiJdfQ==",
		"service": "data-gateway",
		"key": "some-key"
	}`,
		"sharing-project-1": `{
		"batch": "eyJzaGFyZWQtYnVja2V0IjpbIioqIl0sInNoYXJlZC1idWNrZXQtMiI6WyIqKiJdfQ==",
		"service": "data-gateway",
		"key": "some-key"
	}`,
		"sharing-project-2": `{
		"batch": "eyJhbm90aGVyLXNoYXJlZC1idWNrZXQiOlsiKioiXX0=",
		"service": "data-gateway",
		"key": "some-key"
	}`,
	}

	warnings := map[string][]string{
		"my-project":        nil,
		"sharing-project-1": {"No matches found for bucket invisible-bucket", "Bad bucket warning"},
	}

	ai.vi.keyName = "some-key"
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, errStr, errProject string
		whitelistErr, deleteErr      error
	}{
		{
			"OK", "", "", nil, nil,
		},
		{
			"FAIL_WHITELIST",
			"failed to get headers for SD Connect: failed to whitelist public key: " + errExpected.Error(),
			"", errExpected, nil,
		},
		{
			"FAIL_DELETE", "", "", nil, errExpected,
		},
		{
			"FAIL_REQUEST",
			"failed to get headers for SD Connect: request failed: " + errExpected.Error(),
			"my-project", nil, nil,
		},
		{
			"FAIL_REQUEST_SHARED", "", "sharing-project-1", nil, nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			whitelisted := false
			whitelistKey = func(query map[string]string) error {
				whitelisted = true

				return tt.whitelistErr
			}
			deleteWhitelistedKey = func(query map[string]string) error {
				if !whitelisted {
					t.Errorf("deleteWhitelistedKey() was called before key was whitelisted")
				}
				whitelisted = false

				return tt.deleteErr
			}
			makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
				if method != "GET" {
					t.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				if path != "/headers-endpoint" {
					t.Errorf("Request has incorrect path\nExpected=/headers-endpoint\nReceived=%v", path)
				}
				body, err := io.ReadAll(reqBody)
				if err != nil {
					return fmt.Errorf("failed to read body: %w", err)
				}

				if !whitelisted {
					t.Errorf("Key is not whitelisted")
				}
				owner, ok := query["owner"]
				if !ok {
					owner = "my-project"
				}
				if tt.errProject == owner {
					return errExpected
				}

				if string(body) != expectedBody[owner] {
					t.Errorf("Request has incorrect body for project %s\nExpected=%s\nReceived=%s", owner, expectedBody[owner], string(body))
				}

				switch v := ret.(type) {
				case *vaultResponse:
					v.Data = make(BatchHeaders)
					maps.Copy(v.Data, batch[owner])
					v.Warnings = warnings[owner]

					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *vaultResponse", reflect.TypeOf(v))
				}
			}

			data, err := GetHeaders(
				SDConnect,
				[]Metadata{{Name: "bucket_1"}, {Name: "bucket_1_segments"}},
				map[string]SharedBucketsMeta{
					"sharing-project-1": {"shared-bucket", "shared-bucket-2"},
					"sharing-project-2": {"another-shared-bucket"},
					"sharing-project-3": {},
				},
			)

			if tt.testname == "FAIL_REQUEST_SHARED" {
				delete(expectedData, "shared-bucket")
			}

			switch {
			case tt.errStr != "":
				if err == nil {
					t.Errorf("Function did not return error")
				} else if err.Error() != tt.errStr {
					t.Errorf("Function returned incorrect error\nExpected=%q\nReceived=%q", tt.errStr, err.Error())
				}
			case err != nil:
				t.Errorf("Function returned unexpected error: %s", err.Error())
			case !reflect.DeepEqual(expectedData, data):
				t.Errorf("Function returned incorrect headers\nExpected=%v\nReceived=%v", expectedData, data)
			}
		})
	}
}

func TestGetHeaders_SDApply(t *testing.T) {
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

	expectedData := make(BatchHeaders)
	expectedData["dataset-1"] = make(map[string]VaultHeaderVersions)
	expectedData["another-dataset"] = make(map[string]VaultHeaderVersions)

	expectedData["dataset-1"]["kansio/file_1"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"1": {Header: "yvdyviditybf"},
		},
		LatestVersion: 1,
	}
	expectedData["dataset-1"]["kansio/file_2"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"3": {Header: "hbfyucdtkyv"},
			"4": {Header: "hubftiuvfti"},
		},
		LatestVersion: 4,
	}
	expectedData["another-dataset"]["dir/obj.c4gh"] = VaultHeaderVersions{
		Headers: map[string]VaultHeader{
			"1": {Header: "gyutvrtivru"},
		},
		LatestVersion: 1,
	}

	batch := make(map[string]BatchHeaders)
	batch["fega"] = make(BatchHeaders)
	batch["bp"] = make(BatchHeaders)

	batch["fega"]["dataset-1"] = expectedData["dataset-1"]
	batch["bp"]["another-dataset"] = expectedData["another-dataset"]

	expectedBody := map[string][]string{
		"fega": {`{
		"batch": "WyJkYXRhc2V0LTEiLCJkYXRhc2V0LTIiXQ==",
		"service": "data-gateway",
		"key": "some-key"
	}`, `{
		"batch": "WyJkYXRhc2V0LTIiLCJkYXRhc2V0LTEiXQ==",
		"service": "data-gateway",
		"key": "some-key"
	}`},
		"bp": {`{
		"batch": "WyJhbm90aGVyLWRhdGFzZXQiXQ==",
		"service": "data-gateway",
		"key": "some-key"
	}`},
	}

	ai.vi.keyName = "some-key"
	ai.hi.endpoints = testConfig

	whitelisted := false
	whitelistKey = func(query map[string]string) error {
		whitelisted = true

		return nil
	}
	deleteWhitelistedKey = func(query map[string]string) error {
		if !whitelisted {
			t.Errorf("d() was cale key was whitelisted")
		}
		whitelisted = false

		return nil
	}
	makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if method != "GET" {
			t.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", method)
		}
		if path != "/headers-endpoint" {
			t.Errorf("Request has incorrect path\nExpected=/headers-endpoint\nReceived=%v", path)
		}
		body, err := io.ReadAll(reqBody)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}

		if !whitelisted {
			t.Errorf("Key is not whitelisted")
		}
		owner, ok := query["owner"]
		if !ok {
			return fmt.Errorf("request missing query parameter 'owner'")
		}

		found := false
		for _, eb := range expectedBody[owner] {
			if string(body) == eb {
				found = true
			} else {
				t.Logf("Expected=%s", eb)
			}
		}
		if !found {
			t.Errorf("Request has incorrect body for project %s\nReceived=%s", owner, string(body))
		}

		switch v := ret.(type) {
		case *vaultResponse:
			v.Data = batch[owner]

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *vaultResponse", reflect.TypeOf(v))
		}
	}

	data, err := GetHeaders(
		SDApply, []Metadata{},
		map[string]SharedBucketsMeta{
			"fega": {"dataset-1", "dataset-2"},
			"bp":   {"another-dataset"},
		},
	)

	switch {
	case err != nil:
		t.Errorf("Function returned unexpected error: %s", err.Error())
	case !reflect.DeepEqual(expectedData, data):
		t.Errorf("Function returned incorrect headers\nExpected=%v\nReceived=%v", expectedData, data)
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
