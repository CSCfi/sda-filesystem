package api

import (
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"testing"
)

func TestGetHeaderVersions(t *testing.T) {
	origMakeRequest := makeRequest
	origKeyName := ai.vi.keyName
	defer func() {
		makeRequest = origMakeRequest
		ai.vi.keyName = origKeyName
	}()

	expectedData := make(BatchHeaderVersions)
	expectedData["bucket_1"] = make(map[string]int)
	expectedData["shared-bucket"] = make(map[string]int)

	expectedData["bucket_1"]["kansio/file_1"] = 1
	expectedData["bucket_1"]["kansio/file_2"] = 4
	expectedData["shared-bucket"]["dir/obj.c4gh"] = 1

	batch := make(map[string]BatchHeaderVersions)
	batch["my-project"] = make(BatchHeaderVersions)
	batch["sharing-project-1"] = make(BatchHeaderVersions)
	batch["sharing-project-2"] = make(BatchHeaderVersions)

	batch["my-project"]["bucket_1"] = expectedData["bucket_1"]
	batch["sharing-project-1"]["shared-bucket"] = expectedData["shared-bucket"]

	expectedBody := map[string]string{
		"my-project":        `{"batch": "eyJidWNrZXRfMSI6WyIqKiJdfQ==", "versions": true}`,
		"sharing-project-1": `{"batch": "eyJzaGFyZWQtYnVja2V0IjpbIioqIl0sInNoYXJlZC1idWNrZXQtMiI6WyIqKiJdfQ==", "versions": true}`,
		"sharing-project-2": `{"batch": "eyJhbm90aGVyLXNoYXJlZC1idWNrZXQiOlsiKioiXX0=", "versions": true}`,
	}

	warnings := map[string][]string{
		"my-project":        nil,
		"sharing-project-1": {"No matches found for bucket invisible-bucket", "Bad bucket warning"},
	}

	ai.vi.keyName = "some-key"
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, errStr, errProject string
	}{
		{
			"OK", "", "",
		},
		{
			"FAIL_REQUEST",
			"failed to get header versions for SD Connect: request failed: " + errExpected.Error(),
			"my-project",
		},
		{
			"FAIL_REQUEST_SHARED", "", "sharing-project-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(method string, ep endpoint, query, headers map[string]string, reqBody io.Reader, ret any) error {
				if method != "GET" {
					t.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				if ep.path != "/headers-endpoint" {
					t.Errorf("Request has incorrect path\nExpected=/headers-endpoint\nReceived=%v", ep.path)
				}
				body, err := io.ReadAll(reqBody)
				if err != nil {
					return fmt.Errorf("failed to read body: %w", err)
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
				case *vaultBatchResponse:
					v.Data = make(BatchHeaderVersions)
					maps.Copy(v.Data, batch[owner])
					v.Warnings = warnings[owner]

					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *vaultResponse", reflect.TypeOf(v))
				}
			}

			data, err := GetHeaderVersions(
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
				t.Errorf("Function returned incorrect header versions\nExpected=%v\nReceived=%v", expectedData, data)
			}
		})
	}
}

func TestGetHeaderVersions_SDApply(t *testing.T) {
	origMakeRequest := makeRequest
	origKeyName := ai.vi.keyName
	defer func() {
		makeRequest = origMakeRequest
		ai.vi.keyName = origKeyName
	}()

	expectedData := make(BatchHeaderVersions)
	expectedData["dataset-1"] = make(map[string]int)
	expectedData["another-dataset"] = make(map[string]int)

	expectedData["dataset-1"]["kansio/file_1"] = 1
	expectedData["dataset-1"]["kansio/file_2"] = 4
	expectedData["another-dataset"]["dir/obj.c4gh"] = 1

	batch := make(map[string]BatchHeaderVersions)
	batch["fega"] = make(BatchHeaderVersions)
	batch["bp"] = make(BatchHeaderVersions)

	batch["fega"]["dataset-1"] = expectedData["dataset-1"]
	batch["bp"]["another-dataset"] = expectedData["another-dataset"]

	expectedBody := map[string][]string{
		"fega": {`{"batch": "WyJkYXRhc2V0LTEiLCJkYXRhc2V0LTIiXQ==", "versions": true}`, `{"batch": "WyJkYXRhc2V0LTIiLCJkYXRhc2V0LTEiXQ==", "versions": true}`},
		"bp":   {`{"batch": "WyJhbm90aGVyLWRhdGFzZXQiXQ==", "versions": true}`},
	}

	ai.vi.keyName = "some-key"
	ai.hi.endpoints = testConfig

	makeRequest = func(method string, ep endpoint, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if method != "GET" {
			t.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", method)
		}
		if ep.path != "/headers-endpoint" {
			t.Errorf("Request has incorrect path\nExpected=/headers-endpoint\nReceived=%v", ep.path)
		}
		body, err := io.ReadAll(reqBody)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
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
		case *vaultBatchResponse:
			v.Data = batch[owner]

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *vaultResponse", reflect.TypeOf(v))
		}
	}

	data, err := GetHeaderVersions(
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
		t.Errorf("Function returned incorrect header versions\nExpected=%v\nReceived=%v", expectedData, data)
	}
}

func TestGetFileHeader(t *testing.T) {
	origWhitelistKey := whitelistKey
	origMakeRequest := makeRequest
	origKeyName := ai.vi.keyName
	origWhitelistedProjects := whitelistedProjects
	defer func() {
		whitelistKey = origWhitelistKey
		makeRequest = origMakeRequest
		ai.vi.keyName = origKeyName
		whitelistedProjects = origWhitelistedProjects
	}()

	ai.vi.keyName = "some-key"
	ai.hi.endpoints = testConfig

	response := VaultHeaders{
		Headers: map[string]VaultHeader{
			"1": {
				Header: "fvutyubgyugbuy",
			},
			"2": {
				Header: "hubitiutituyvu",
			},
			"3": {
				Header: "qutevdfuyvoybgi",
			},
		},
		LatestVersion: 3,
	}

	var tests = []struct {
		testname, owner, expectedHeader, errStr string
		errRequest, errWhitelist                error
		version                                 int
		whitelist                               bool
	}{
		{
			"OK_1", "", "fvutyubgyugbuy", "", nil, nil, 1, true,
		},
		{
			"OK_2", "project_1234567", "qutevdfuyvoybgi", "", nil, nil, 3, false,
		},
		{
			"FAIL_REQUEST", "", "", errExpected.Error(), errExpected, nil, 2, false,
		},
		{
			"FAIL_WHITELIST_1", "", "",
			"failed to whitelist public key for SD Connect: " + errExpected.Error(),
			nil, errExpected, 1, true,
		},
		{
			"FAIL_WHITELIST_2", "project_20001234", "",
			"failed to whitelist public key for SD Connect shared project (project_20001234): " + errExpected.Error(),
			nil, errExpected, 1, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(method string, ep endpoint, query, headers map[string]string, reqBody io.Reader, ret any) error {
				if method != "GET" {
					t.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				if ep.path != "/headers-endpoint/my-bucket" {
					t.Errorf("Request has incorrect path\nExpected=/headers-endpoint/my-bucket\nReceived=%v", ep.path)
				}

				owner, ok := query["owner"]
				if ok && tt.owner == "" {
					t.Errorf("Request should not have owner, received %v", owner)
				}
				if owner != tt.owner {
					t.Errorf("Request has incorrect owner\nExpected=%v\nReceived=%v", tt.owner, owner)
				}
				svc, ok := query["service"]
				if svc != vaultService {
					t.Errorf("Request has incorrect service\nExpected=%s\nReceived=%v", vaultService, svc)
				}
				key := query["key"]
				if key != "some-key" {
					t.Errorf("Request has incorrect key\nExpected=some-key\nReceived=%v", key)
				}
				object := query["object"]
				if object != "my-object" {
					t.Errorf("Request has incorrect key\nExpected=my-object\nReceived=%v", object)
				}

				switch v := ret.(type) {
				case *VaultHeaders:
					v.Headers = response.Headers
					v.LatestVersion = response.LatestVersion

					return tt.errRequest
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *vaultResponse", reflect.TypeOf(v))
				}
			}
			whitelisted := false
			whitelistKey = func(query map[string]string) error {
				whitelisted = true

				return tt.errWhitelist
			}

			whitelistedProjects = make([]string, 0)
			if !tt.whitelist {
				whitelistedProjects = append(whitelistedProjects, tt.owner)
			}

			header, err := GetFileHeader(SDConnect, "my-bucket", "my-object", tt.owner, tt.version, "")

			switch {
			case tt.errStr != "":
				if err == nil {
					t.Errorf("Function did not return error")
				} else if err.Error() != tt.errStr {
					t.Errorf("Function returned incorrect error\nExpected=%q\nReceived=%q", tt.errStr, err.Error())
				}
			case err != nil:
				t.Errorf("Function returned unexpected error: %s", err.Error())
			case tt.whitelist != whitelisted:
				t.Errorf("Functions resulsted in unexpected whitelist status\nExpected=%t\nReceived=%t", tt.whitelist, whitelisted)
			case header != tt.expectedHeader:
				t.Errorf("Function returned incorrect header\nExpected=%v\nReceived=%v", tt.expectedHeader, header)
			}
		})
	}
}

func TestDeleteWhitelistedKeys(t *testing.T) {
	origMakeRequest := makeRequest
	origKeyName := ai.vi.keyName
	origWhitelistedProjects := whitelistedProjects
	defer func() {
		makeRequest = origMakeRequest
		ai.vi.keyName = origKeyName
		whitelistedProjects = origWhitelistedProjects
	}()

	ai.vi.keyName = "some-key"
	ai.hi.endpoints = testConfig

	whitelistedProjects = []string{"", "project-1", "project-2", "chicken"}
	whitelistedProjectsCopy := slices.Clone(whitelistedProjects)

	makeRequest = func(method string, ep endpoint, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if method != "DELETE" {
			t.Errorf("Request has incorrect method\nExpected=DELETE\nReceived=%v", method)
		}
		expectedPath := "/whitelist-endpoint/" + vaultService + "/some-key"
		if ep.path != expectedPath {
			t.Errorf("Request has incorrect path\nExpected=%v\nReceived=%v", expectedPath, ep.path)
		}

		owner, ok := query["owner"]
		if !ok {
			owner = ""
		}

		if !slices.Contains(whitelistedProjects, owner) {
			t.Errorf("Owner %q is not in the slice of whitelisted projects: %q", owner, whitelistedProjects)
		}
		whitelistedProjectsCopy = slices.DeleteFunc(whitelistedProjectsCopy, func(pr string) bool {
			return pr == owner
		})

		return nil
	}

	DeleteWhitelistedKeys()

	if len(whitelistedProjectsCopy) > 0 {
		t.Errorf("The slice of whitelisted projects is not empty after delete: %q", whitelistedProjectsCopy)
	}
}

func TestGetPublicKey(t *testing.T) {
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
			makeRequest = func(method string, ep endpoint, query, headers map[string]string, body io.Reader, ret any) error {
				if method != "GET" {
					return fmt.Errorf("request has incorrect method\nExpected=GET\nReceived=%v", method)
				}
				switch v := ret.(type) {
				case *keyResponse:
					switch ep.path {
					case "/project-key-endpoint":
						v.Key64 = fmt.Sprintf("-----BEGIN CRYPT4GH PUBLIC KEY-----\n%s\n-----END CRYPT4GH PUBLIC KEY-----", tt.key64)
					default:
						return fmt.Errorf("request has incorrect path %v", ep.path)
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

	makeRequest = func(method string, ep endpoint, query, headers map[string]string, body io.Reader, ret any) error {
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
