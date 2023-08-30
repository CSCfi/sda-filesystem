package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type mockConnecter struct {
	sTokens     map[string]sToken
	projects    []Metadata
	projectsErr error
	token       sToken
	tokenErr    error
}

func (c *mockConnecter) getProjects() ([]Metadata, error) {
	if c.projectsErr != nil {
		return nil, fmt.Errorf("getProjects error: %w", c.projectsErr)
	}

	return c.projects, nil
}

func (c *mockConnecter) getToken(string) (sToken, error) {
	if c.tokenErr != nil {
		return sToken{}, fmt.Errorf("getToken error: %w", c.tokenErr)
	}

	return c.token, nil
}

func (c *mockConnecter) getSTokens([]Metadata) map[string]sToken {
	return c.sTokens
}

func Test_SDConnect_GetProjects(t *testing.T) {
	var tests = []struct {
		testname, url, token string
		expectedMetaData     []Metadata
	}{
		{
			"OK_1", "google.com", "7ce5ic",
			[]Metadata{{234, "Jack"}, {2, "yur586bl"}, {7489, "rtu6u__78bgi"}},
		},
		{
			"OK_2", "example.com", "2cjv05fgi",
			[]Metadata{{740, "rtu6u__78boi"}, {83, "85cek6o"}},
		},
		{
			"OK_EMPTY", "hs.fi", "WHM6d.7k", []Metadata{},
		},
	}

	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				if token, ok := headers["X-Authorization"]; !ok || token != "Basic "+tt.token {
					return fmt.Errorf("Incorrect header 'X-Authorization'\nExpected=%s\nReceived=%s", "Bearer "+tt.token, token)
				}

				switch v := ret.(type) {
				case *[]Metadata:
					*v = tt.expectedMetaData

					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			}

			url := "https://data.csc.fi"
			c := connecter{url: &url, token: &tt.token}
			projects, err := c.getProjects()

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, projects) {
				t.Errorf("Projects incorrect. Expected=%v, received=%v", tt.expectedMetaData, projects)
			}
		})
	}
}

func Test_SDConnect_GetProjects_Error(t *testing.T) {
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		return errExpected
	}

	url := "url"
	token := "token"
	c := connecter{url: &url, token: &token}
	projects, err := c.getProjects()

	if err == nil {
		t.Error("Function should have returned error")
	} else if !errors.Is(err, errExpected) {
		t.Errorf("Function returned incorrect error: %s", err.Error())
	}

	if projects != nil {
		t.Errorf("Slice should have been empty, received=%v", projects)
	}
}

func Test_SDConnect_GetSTokens(t *testing.T) {
	var tests = []struct {
		testname string
		projects []Metadata
		override bool
		sTokens  map[string]sToken
	}{
		{
			"OK_1", []Metadata{{56, "project1"}, {67, "project2"}}, false,
			map[string]sToken{"project1": {"vhjk", "cud7"}, "project2": {"d6l", "88x6l"}},
		},
		{
			"OK_2", []Metadata{{23, "pr1568"}, {90, "pr2097"}}, true,
			map[string]sToken{"pr1568": {"6rxy", "7cli87t"}, "pr2097": {"7cek", "25c8"}},
		},
		{
			"FAIL_STOKENS", []Metadata{{496, "pr152"}, {271, "pr375"}, {12, "pr225"}}, false,
			map[string]sToken{"pr225": {"8vgicö", "xfd6"}},
		},
	}

	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				if _, ok := tt.sTokens[query["project"]]; !ok {
					return fmt.Errorf("Error occurred")
				}
				if token, ok := headers["X-Authorization"]; !ok || token != "Basic token" {
					t.Errorf("X-Authorization header incorrect.")
				}

				switch project, ok := headers["X-Project-Name"]; {
				case tt.override && (!ok || project != query["project"]):
					t.Errorf("X-Project-Name header incorrect.")
				case !tt.override && ok:
					t.Errorf("Header should not contain X-Project-Name")
				}

				switch v := ret.(type) {
				case *sToken:
					*v = tt.sTokens[query["project"]]
				default:
					t.Errorf("ret has incorrect type %v, expected *SToken", reflect.TypeOf(v))
				}

				return nil
			}

			url := "url"
			token := "token"
			overridden := tt.override
			c := connecter{url: &url, token: &token, overriden: &overridden}
			newSTokens := c.getSTokens(tt.projects)

			if !reflect.DeepEqual(newSTokens, tt.sTokens) {
				t.Errorf("sTokens incorrect.\nExpected=%s\nReceived=%s", tt.sTokens, newSTokens)
			}
		})
	}
}

func Test_SDConnect_GetEnvs(t *testing.T) {
	var tests = []struct {
		testname      string
		expectedURL   string
		expectedError error
		mockGetEnv    func(string, bool) (string, error)
		mockTestURL   func(string) error
	}{
		{
			testname:      "OK",
			expectedURL:   "https://data.csc.fi",
			expectedError: nil,
			mockGetEnv: func(s string, b bool) (string, error) {
				if s != "FS_SD_CONNECT_API" {
					return "https://metadata.csc.fi", nil
				}

				return "https://data.csc.fi", nil

			},
			mockTestURL: func(s string) error {
				return nil
			},
		},
		{
			testname:      "FAIL_API_ENV",
			expectedURL:   "",
			expectedError: errors.New("some error"),
			mockGetEnv: func(s string, b bool) (string, error) {
				return "", errors.New("some error")
			},
			mockTestURL: nil,
		},
		{
			testname:      "FAIL_API_VALIDATE",
			expectedURL:   "https://metadata.csc.fi",
			expectedError: errors.New("Cannot connect to SD Connect API: bad url"),
			mockGetEnv: func(s string, b bool) (string, error) {
				return "https://metadata.csc.fi", nil
			},
			mockTestURL: func(s string) error {
				return errors.New("bad url")
			},
		},
	}

	origGetEnvs := GetEnv
	origTestURL := testURL
	defer func() {
		GetEnv = origGetEnvs
		testURL = origTestURL
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			// Place mocks
			sd := &sdConnectInfo{}
			GetEnv = tt.mockGetEnv
			testURL = tt.mockTestURL

			// Invoke function
			err := sd.getEnvs()

			// Test results
			switch {
			case tt.testname == "OK":
				if err != nil {
					t.Errorf("Unexpected error: %s", err.Error())
				}
			case err == nil:
				t.Error("Function did not return error")
			case err.Error() != tt.expectedError.Error():
				t.Errorf("Function failed\nExpected=%s\nReceived=%s", tt.expectedError.Error(), err.Error())
			}

			if sd.url != tt.expectedURL {
				t.Errorf("URL incorrect. Expected=%v, received=%v", tt.expectedURL, sd.url)
			}
		})
	}
}

func Test_SDConnect_ValidateLogin_OK(t *testing.T) {
	projects := []Metadata{{56, "pr1"}, {45, "pr56"}, {8, "pr88"}}
	mockC := &mockConnecter{sTokens: map[string]sToken{"s1": {"sToken", "proj1"}}, projects: projects}
	sd := &sdConnectInfo{connectable: mockC}

	err := sd.authenticate("dXNlcjpwYXNz", "")
	if err != nil {
		t.Fatalf("Function failed, expected no error, received=%v", err)
	}
	if sd.token != "dXNlcjpwYXNz" {
		t.Errorf("Token incorrect. Expected=dXNlcjpwYXNz, received=%s", sd.token)
	}
	if sd.overriden {
		t.Error("Projects should not have been overridden")
	}
	if st := sd.sTokens["s1"].Token; st != "sToken" {
		t.Errorf("sToken incorrect for project 's1'. Expected=sToken, received=%s", st)
	}
	if pi := sd.sTokens["s1"].ProjectID; pi != "proj1" {
		t.Errorf("ProjectID incorrect for project 's1'. expected=proj1, received=%s", pi)
	}
	if !reflect.DeepEqual(sd.projects, projects) {
		t.Errorf("Projects incorrect\nExpected=%v\nReceived=%v", projects, sd.projects)
	}
}

func Test_SDConnect_ValidateLogin_Override_OK(t *testing.T) {
	project := "project_7376"
	projects := []Metadata{{-1, project}}
	mockC := &mockConnecter{
		projectsErr: errors.New("should not have fetched projects"),
		token:       sToken{"sToken", "projectID"},
	}
	sd := &sdConnectInfo{connectable: mockC}

	err := sd.authenticate("vfylxr7pckgh", project)
	if err != nil {
		t.Fatalf("Function failed, expected no error, received=%v", err)
	}
	if sd.token != "vfylxr7pckgh" {
		t.Errorf("Token incorrect. Expected=vfylxr7pckgh, received=%s", sd.token)
	}
	if !sd.overriden {
		t.Error("Projects should have been overridden.")
	}
	if st := sd.sTokens[project].Token; st != "sToken" {
		t.Errorf("sToken incorrect for project '%s'. Expected=sToken, received=%s", project, st)
	}
	if pi := sd.sTokens[project].ProjectID; pi != "projectID" {
		t.Errorf("ProjectID incorrect for project '%s'. expected=projectID, received=%s", project, pi)
	}
	if !reflect.DeepEqual(sd.projects, projects) {
		t.Errorf("Projects incorrect\nExpected=%v\nReceived=%v", projects, sd.projects)
	}
}

func Test_SDConnect_ValidateLogin_Fail_GetProjects(t *testing.T) {
	mockC := &mockConnecter{projectsErr: errors.New("Error occurred")}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "getProjects error: Error occurred"
	err := sd.authenticate("u7c9Cstlv7", "")
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != expectedError {
		t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}
}

func Test_SDConnect_ValidateLogin_Fail_GetToken(t *testing.T) {
	mockC := &mockConnecter{tokenErr: errors.New("Error occurred")}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "getToken error: Error occurred"
	err := sd.authenticate("vhy9pcr7til", "project")
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != expectedError {
		t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}
}

func Test_SDConnect_ValidateLogin_Fail_Parameters(t *testing.T) {
	sd := &sdConnectInfo{}
	expectedError := "validateLogin() called with too few parameters"

	err := sd.authenticate("one-param")
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != expectedError {
		t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}
}

func Test_SDConnect_ValidateLogin_No_Projects(t *testing.T) {
	mockC := &mockConnecter{projects: nil}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "No projects found for SD Connect"
	err := sd.authenticate("f60ovguTit7", "")
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != expectedError {
		t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}
}

func Test_SDConnect_ValidateLogin_401_Error(t *testing.T) {
	mockC := &mockConnecter{projectsErr: &RequestError{StatusCode: 401}}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "SD Connect login failed: getProjects error: API responded with status 401 Unauthorized"
	err := sd.authenticate("69vdtulvf6", "")
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != expectedError {
		t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}
}

func Test_SDConnect_ValidateLogin_500_Error(t *testing.T) {
	mockC := &mockConnecter{projectsErr: &RequestError{StatusCode: 500}}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "SD Connect is not available, please contact CSC servicedesk: getProjects error: API responded with status 500 Internal Server Error"
	err := sd.authenticate("7vr6lvgil", "")
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != expectedError {
		t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}
}

func Test_SDConnect_GetNthLevel_Projects(t *testing.T) {
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	mockC := &mockConnecter{}
	projects := []Metadata{{34, "Pr3"}, {90, "Pr56"}, {123, "Pr7"}, {4, "Pr12"}}
	sd := &sdConnectInfo{connectable: mockC, projects: projects}

	meta, err := sd.getNthLevel("")
	if err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	} else if !reflect.DeepEqual(meta, projects) {
		t.Errorf("Returned incorrect metadata. Expected=%v, received=%v", projects, meta)
	}
}

func Test_SDConnect_LevelCount(t *testing.T) {
	sd := sdConnectInfo{}
	if lc := sd.levelCount(); lc != 3 {
		t.Errorf("Function failed, expected=3, received=%d", lc)
	}
}

func Test_SDConnect_CalculateDecryptedSize(t *testing.T) {
	// Fail min size
	s := calculateDecryptedSize(5, 5)
	if s != -1 {
		t.Errorf("Function failed, expected=-1, received=%d", s)
	}

	// OK
	s = calculateDecryptedSize(500, 128)
	if s != 344 {
		t.Errorf("Function failed, expected=344, received=%d", s)
	}

	// OK, test remainder
	s = calculateDecryptedSize(65690, 100)
	if s != 65562 {
		t.Errorf("Function failed, expected=65562, received=%d", s)
	}
}

func Test_SDConnect_GetNthLevel_Fail_NoNodes(t *testing.T) {
	md := []Metadata{{Bytes: 10, Name: "project1"}}
	sd := &sdConnectInfo{projects: md}
	metadata, err := sd.getNthLevel("fspath")
	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
	}
	if metadata[0].Name != "project1" {
		t.Errorf("Function failed, expected=project1, received=%s", metadata[0].Name)
	}
}

func Test_SDConnect_GetNthLevel_Fail_Path(t *testing.T) {
	sd := &sdConnectInfo{}
	metadata, err := sd.getNthLevel("fspath", "1", "2", "3")
	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
	}
	if metadata != nil {
		t.Errorf("Function failed, expected=nil, received=%v", metadata)
	}
}

func Test_SDConnect_GetNthLevel_Fail_Request(t *testing.T) {
	// Mock
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()
	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		return errors.New("some error")
	}
	sd := &sdConnectInfo{}

	// Test
	expectedError := "Failed to retrieve metadata for fspath: some error"
	_, err := sd.getNthLevel("fspath", "1", "2")
	if err.Error() != expectedError {
		t.Errorf("Function failed, expected=%s, received=%v", expectedError, err)
	}
}

func Test_SDConnect_GetNthLevel_Pass_1Node(t *testing.T) {
	// Mock
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	count := 0
	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		_ = json.NewDecoder(bytes.NewReader([]byte(`[{"bytes":100,"name":"thingy1"}]`))).Decode(ret)
		if count == 0 {
			count++

			return nil
		}

		return fmt.Errorf("MakeRequest() called too many times")
	}
	sd := &sdConnectInfo{}

	// Test
	meta, err := sd.getNthLevel("fspath", "1")
	if err != nil {
		t.Fatalf("Function failed, expected no error, received=%v", err)
	}
	if meta[0].Bytes != 100 {
		t.Errorf("Function failed, expected=%d, received=%d", 100, meta[0].Bytes)
	}
	if meta[0].Name != "thingy1" {
		t.Errorf("Function failed, expected=%s, received=%s", "thingy1", meta[0].Name)
	}
}

func Test_SDConnect_GetNthLevel_Pass_2Node(t *testing.T) {
	// Mock
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	count := 0
	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		switch count {
		case 0:
			_ = json.NewDecoder(bytes.NewReader([]byte(`[{"bytes":100,"name":"thingy2"}]`))).Decode(ret)
		case 1:
			_ = json.NewDecoder(bytes.NewReader([]byte(`[{"bytes":674,"name":"thingy3"}]`))).Decode(ret)
		case 2:
		default:
			return fmt.Errorf("MakeRequest() called too many times")
		}
		count++

		return nil
	}
	sd := &sdConnectInfo{}
	objects := []Metadata{{100, "thingy2"}, {674, "thingy3"}}

	// Test
	meta, err := sd.getNthLevel("fspath", "1", "2")
	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
	}
	if !reflect.DeepEqual(meta, objects) {
		t.Errorf("Function failed, expected=%v, received=%v", objects, meta)
	}
}

func Test_SDConnect_GetNthLevel_Pass_TokenExpired(t *testing.T) {
	// Mock
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	count := 0
	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		if token, ok := headers["X-Authorization"]; ok && token == "Bearer freshToken" {
			if count == 0 {
				_ = json.NewDecoder(bytes.NewReader([]byte(`[{"bytes":100,"name":"thingy3"}]`))).Decode(ret)
			}
			if count > 1 {
				return fmt.Errorf("MakeRequest() called too many times")
			}
			count++

			return nil
		}

		return &RequestError{http.StatusUnauthorized}
	}
	mockC := &mockConnecter{sTokens: map[string]sToken{"project": {"freshToken", "projectID"}}}
	sd := &sdConnectInfo{
		connectable: mockC,
		sTokens:     map[string]sToken{"project": {"expiredToken", "project"}},
		projects:    []Metadata{},
	}

	// Test
	meta, err := sd.getNthLevel("sdconnect", "project", "container")
	if err != nil {
		t.Fatalf("Function failed, expected no error, received=%v", err)
	}
	if len(meta) == 0 {
		t.Fatalf("Function failed, returned metadata empty")
	}
	if meta[0].Bytes != 100 {
		t.Errorf("Function failed, expected=%d, received=%d", 100, meta[0].Bytes)
	}
	if meta[0].Name != "thingy3" {
		t.Errorf("Function failed, expected=%s, received=%s", "thingy3", meta[0].Name)
	}
}

func Test_SDConnect_GetNthLevel_Fail_TokenExpired(t *testing.T) {
	// Mock
	expectedError := "Failed to retrieve metadata for sdconnect: API responded with status 401 Unauthorized"
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()
	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		return &RequestError{http.StatusUnauthorized}
	}
	mockC := &mockConnecter{sTokens: map[string]sToken{"project": {"freshToken", "projectID"}}}
	sd := &sdConnectInfo{
		connectable: mockC,
		sTokens:     map[string]sToken{"project": {"expiredToken", "projectID"}},
		projects:    []Metadata{},
	}

	// Test
	_, err := sd.getNthLevel("sdconnect", "project", "container")
	if err.Error() != expectedError {
		t.Errorf("Function failed, expected=%s, received=%v", expectedError, err)
	}
}

func Test_SDConnect_UpdateAttributes(t *testing.T) {
	var tests = []struct {
		testname                                 string
		segmentedObjectSize, initSize, finalSize int64
		decrypted                                bool
	}{
		{"OK_SEGMENTED_DERYPTED", 30, 75, 23, true},
		{"OK_SEGMENTED_NOT_DECRYPTED", 67, 6, 67, false},
		{"OK_NOT_SEGMENTED_DECRYPTED", -1, 6, 6, true},
		{"OK_NOT_SEGMENTED_NOT_DECRYPTED", -1, 34, 34, false},
	}

	origMakeRequest := MakeRequest
	origCalculateDecryptedSize := calculateDecryptedSize

	defer func() {
		MakeRequest = origMakeRequest
		calculateDecryptedSize = origCalculateDecryptedSize
	}()

	calculateDecryptedSize = func(fileSize, headerSize int64) int64 {
		return fileSize - 7
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				switch v := ret.(type) {
				case *SpecialHeaders:
					v.Decrypted = tt.decrypted
					v.SegmentedObjectSize = tt.segmentedObjectSize
					v.HeaderSize = 0

					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *SpecialHeaders", reflect.TypeOf(v))
				}
			}

			var size = tt.initSize
			sd := &sdConnectInfo{}
			err := sd.updateAttributes([]string{"path", "to", "file"}, "path/to/file", &size)

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if size != tt.finalSize {
				t.Errorf("Final size was incorrect. Expected=%d, received=%d", tt.finalSize, size)
			}
		})
	}
}

func Test_SDConnect_UpdateAttributes_Error(t *testing.T) {
	var tests = []struct {
		testname, errText string
		nodes             []string
		requestErr        error
		value             any
	}{
		{
			"TOO_FEW_NODES", "Cannot update attributes for path Folder/file",
			[]string{"Folder", "file"}, nil, "test",
		},
		{
			"WRONG_DATA_TYPE",
			"SD Connect updateAttributes() was called with incorrect attribute. Expected type *int64, received *string",
			[]string{"Folder", "dir", "file"}, nil, "test",
		},
		{
			"FAIL_DOWNLOAD", errExpected.Error(),
			[]string{"Folder", "dir", "file"}, errExpected, int64(10),
		},
	}

	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
				return tt.requestErr
			}

			var err error
			sd := &sdConnectInfo{}
			switch v := tt.value.(type) {
			case int64:
				err = sd.updateAttributes(tt.nodes, strings.Join(tt.nodes, "/"), &v)
			case string:
				err = sd.updateAttributes(tt.nodes, strings.Join(tt.nodes, "/"), &v)
			}

			if err == nil {
				t.Error("Function should have returned error")
			} else if err.Error() != tt.errText {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%v", tt.errText, err)
			}
		})
	}
}

func Test_SDConnect_DownloadData_Pass(t *testing.T) {
	// Mock
	expectedBody := []byte("hellothere")
	expectedHeaders := map[string]string{
		"Range":           "bytes=0-9",
		"X-Authorization": "Bearer token",
		"X-Project-ID":    "project",
	}
	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()
	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		// Test that headers were computed properly
		if !reflect.DeepEqual(headers, expectedHeaders) {
			t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedHeaders, headers)
		}
		_, _ = io.ReadFull(bytes.NewReader(expectedBody), ret.([]byte))

		return nil
	}
	sd := &sdConnectInfo{sTokens: map[string]sToken{"project": {"token", "project"}}}

	// Test
	buf := make([]byte, 10)
	err := sd.downloadData([]string{"project", "container", "object"}, buf, 0, 10)

	if err != nil {
		t.Fatalf("Function failed, expected no error, received=%v", err)
	}
	if !bytes.Equal(buf, expectedBody) {
		t.Errorf("Function failed\nExpected=%s\nReceived=%s", string(expectedBody), string(buf))
	}
}

func Test_SDConnect_DownloadData_Pass_TokenExpired(t *testing.T) {
	// Mock
	expectedBody := []byte("hellothere")
	expectedHeaders := map[string]string{
		"Range":           "bytes=0-9",
		"X-Authorization": "Bearer freshToken",
		"X-Project-ID":    "projectID",
		"X-Project-Name":  "project",
	}

	origMakeRequest := MakeRequest
	defer func() { MakeRequest = origMakeRequest }()

	MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
		if token, ok := headers["X-Authorization"]; ok && token == "Bearer freshToken" {
			// Test that headers were computed properly
			if !reflect.DeepEqual(headers, expectedHeaders) {
				t.Errorf("Function failed\nExpected=%s\nReceived=%s", expectedHeaders, headers)
			}
			_, _ = io.ReadFull(bytes.NewReader(expectedBody), ret.([]byte))

			return nil
		}

		return &RequestError{http.StatusUnauthorized}
	}
	mockC := &mockConnecter{sTokens: map[string]sToken{"project": {"freshToken", "projectID"}}}
	sd := &sdConnectInfo{
		connectable: mockC,
		sTokens:     map[string]sToken{"project": {"expiredToken", "projectID"}},
		projects:    []Metadata{},
		overriden:   true,
	}

	// Test
	buf := make([]byte, 10)
	err := sd.downloadData([]string{"project", "container", "object"}, buf, 0, 10)

	if err != nil {
		t.Fatalf("Function failed, expected no error, received=%v", err)
	}
	if !bytes.Equal(buf, expectedBody) {
		t.Errorf("Function failed, expected=%s, received=%s", string(expectedBody), string(buf))
	}
}
