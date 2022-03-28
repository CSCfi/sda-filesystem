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

const envMetaUrl = "FS_SD_CONNECT_METADATA_API"
const metaUrl = "http://metadata.csc.fi"

type mockTokenator struct {
	uToken  string
	sTokens map[string]sToken
}

func (t *mockTokenator) getUToken(string) (string, error) {
	if t.uToken == "" {
		return "", errors.New("uToken error")
	}
	return t.uToken, nil
}

func (t *mockTokenator) getSToken(string, pr string) (sToken, error) {
	if token, ok := t.sTokens[pr]; !ok {
		return sToken{}, errors.New("sToken not found")
	} else if token == (sToken{}) {
		return sToken{}, errors.New("sToken error")
	}
	return t.sTokens[pr], nil
}

func (t *mockTokenator) keys() (ret []Metadata) {
	for key := range t.sTokens {
		ret = append(ret, Metadata{Name: key})
	}
	return
}

type mockConnecter struct {
	tokenable
	uToken      string
	sTokenKey   string
	sTokenValue sToken
	projects    []Metadata
	projectsErr error
}

func (c *mockConnecter) getProjects(string) ([]Metadata, error) {
	if c.uToken == "" {
		return nil, fmt.Errorf("getProjects error: %w", c.projectsErr)
	}
	return c.projects, nil
}

func (c *mockConnecter) fetchTokens(bool, []Metadata) (string, map[string]sToken) {
	m := make(map[string]sToken)
	m[c.sTokenKey] = c.sTokenValue
	return c.uToken, m
}

func Test_SDConnect_GetUToken(t *testing.T) {
	var tests = []struct {
		testname, url, expectedToken string
	}{
		{"OK_1", "github.com", "myveryowntoken"},
		{"OK_2", "google.com", "9765rty5678"},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(url string, token string, repository string, query map[string]string, headers map[string]string, ret interface{}) error {
				if url != tt.url+"/token" {
					return fmt.Errorf("makeRequest() was called with incorrect URL. Expected=%s, received=%s", tt.url+"/token", url)
				}

				switch v := ret.(type) {
				case *uToken:
					*v = uToken{tt.expectedToken}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *UToken", reflect.TypeOf(v))
				}
			}

			tr := tokenator{}
			token, err := tr.getUToken(tt.url)

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if tt.expectedToken != token {
				t.Errorf("Unscoped token is incorrect. Expected=%s, received=%s", tt.expectedToken, token)
			}
		})
	}
}

func Test_SDConnect_GetUToken_Error(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	makeRequest = func(url string, token string, repository string, query map[string]string, headers map[string]string, ret interface{}) error {
		return errExpected
	}

	tr := tokenator{}
	token, err := tr.getUToken("")

	if err == nil {
		t.Error("Function should have returned error")
	} else if !errors.Is(err, errExpected) {
		t.Errorf("Function returned incorrect error: %s", err.Error())
	}

	if token != "" {
		t.Errorf("Unscoped token should have been empty, received=%s", token)
	}
}

func Test_SDConnect_GetSToken(t *testing.T) {
	var tests = []struct {
		testname, project, url, expectedToken, expectedID string
	}{
		{"OK_1", "project007", "google.com", "myveryowntoken", "jbowegxf72nfbof"},
		{"OK_2", "projectID", "github.com", "9765rty5678", "ug8392nzdipqz9210z"},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(url string, token string, repository string, query map[string]string, headers map[string]string, ret interface{}) error {
				if url != tt.url+"/token" {
					return fmt.Errorf("makeRequest() was called with incorrect url. Expected=%s, reveived=%s", tt.url+"/token", url)
				}
				if query["project"] != tt.project {
					return fmt.Errorf("makeRequest() was called with incorrect query. Expected key 'project' to have value %s, received=%s",
						tt.project, query["project"])
				}

				switch v := ret.(type) {
				case *sToken:
					*v = sToken{tt.expectedToken, tt.expectedID}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *SToken", reflect.TypeOf(v))
				}
			}

			tr := tokenator{}
			token, err := tr.getSToken(tt.url, tt.project)

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if tt.expectedToken != token.Token {
				t.Errorf("Scoped token is incorrect. Expected=%s, received=%q", tt.expectedToken, token.Token)
			} else if tt.expectedID != token.ProjectID {
				t.Errorf("Project ID is incorrect. Expected=%s, received=%s", tt.expectedID, token.ProjectID)
			}
		})
	}
}

func Test_SDConnect_GetSToken_Error(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	makeRequest = func(url string, token string, repository string, query map[string]string, headers map[string]string, ret interface{}) error {
		return errExpected
	}

	tr := tokenator{}
	token, err := tr.getSToken("", "")

	if err == nil {
		t.Error("Function should have returned error")
	} else if !errors.Is(err, errExpected) {
		t.Errorf("Function returned incorrect error: %s", err.Error())
	}

	if token != (sToken{}) {
		t.Errorf("Scoped token should have been empty, received=%s", token)
	}
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

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(url string, token string, repository string, query map[string]string, headers map[string]string, ret interface{}) error {
				if url != tt.url+"/projects" {
					return fmt.Errorf("makeRequest() was called with incorrect url. Expected=%s, received=%s", tt.url+"/projects", url)
				}
				if token != tt.token {
					return fmt.Errorf("makeRequest() was called with incorrect token. Expected=%s, received=%s", tt.token, token)
				}

				switch v := ret.(type) {
				case *[]Metadata:
					*v = tt.expectedMetaData
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			}

			c := connecter{url: &tt.url}
			projects, err := c.getProjects(tt.token)

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, projects) {
				t.Errorf("Projects incorrect. Expected=%v, received=%v", tt.expectedMetaData, projects)
			}
		})
	}
}

func Test_SDConnect_GetProjects_Error(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	makeRequest = func(url string, token string, repository string, query map[string]string, headers map[string]string, ret interface{}) error {
		return errExpected
	}

	dummy := ""
	c := connecter{url: &dummy}
	projects, err := c.getProjects("")

	if err == nil {
		t.Error("Function should have returned error")
	} else if !errors.Is(err, errExpected) {
		t.Errorf("Function returned incorrect error: %s", err.Error())
	}

	if projects != nil {
		t.Errorf("Slice should have been empty, received=%v", projects)
	}
}

func Test_SDConnect_FetchTokens(t *testing.T) {
	var tests = []struct {
		testname, uToken, mockUToken string
		skip                         bool
		sTokens, mockSTokens         map[string]sToken
	}{
		{
			"OK_1", "unscoped token", "unscoped token", false,
			map[string]sToken{"project1": {"vhjk", "cud7"}, "project2": {"d6l", "88x6l"}},
			map[string]sToken{"project1": {"vhjk", "cud7"}, "project2": {"d6l", "88x6l"}},
		},
		{
			"OK_2", "", "garbage", true,
			map[string]sToken{"pr1568": {"6rxy", "7cli87t"}, "pr2097": {"7cek", "25c8"}},
			map[string]sToken{"pr1568": {"6rxy", "7cli87t"}, "pr2097": {"7cek", "25c8"}},
		},
		{
			"FAIL_UTOKEN", "", "", false,
			map[string]sToken{},
			map[string]sToken{"project1": {"5xe7k", "6xwei"}, "project2": {"5xw46", "4wx6"}},
		},
		{
			"FAIL_STOKENS", "utoken", "utoken", false,
			map[string]sToken{"pr225": {"8vgicö", "xfd6"}},
			map[string]sToken{"pr152": {}, "pr375": {}, "pr225": {"8vgicö", "xfd6"}},
		},
	}

	dummy := ""

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			mockT := &mockTokenator{uToken: tt.mockUToken, sTokens: tt.mockSTokens}
			c := connecter{tokenable: mockT, url: &dummy}

			newUToken, newSTokens := c.fetchTokens(tt.skip, mockT.keys())

			if newUToken != tt.uToken {
				t.Errorf("uToken incorrect. Expected=%s, received=%s", tt.uToken, newUToken)
			} else if !reflect.DeepEqual(newSTokens, tt.sTokens) {
				t.Errorf("sTokens incorrect.\nExpected=%s\nReceived=%s", tt.sTokens, newSTokens)
			}
		})
	}
}

func Test_SDConnect_GetEnvs(t *testing.T) {
	var tests = []struct {
		testname            string
		expectedMetadataURL string
		expectedDataURL     string
		expectedError       error
		mockGetEnv          func(string, bool) (string, error)
		mockTestURL         func(string) error
	}{
		{
			testname:            "OK",
			expectedMetadataURL: "https://metadata.csc.fi",
			expectedDataURL:     "https://data.csc.fi",
			expectedError:       nil,
			mockGetEnv: func(s string, b bool) (string, error) {
				if s == envMetaUrl {
					return "https://metadata.csc.fi", nil
				} else {
					return "https://data.csc.fi", nil
				}
			},
			mockTestURL: func(s string) error {
				return nil
			},
		},
		{
			testname:            "FAIL_METADATA_ENV",
			expectedMetadataURL: "",
			expectedDataURL:     "",
			expectedError:       errors.New("some error"),
			mockGetEnv: func(s string, b bool) (string, error) {
				return "", errors.New("some error")
			},
			mockTestURL: nil,
		},
		{
			testname:            "FAIL_METADATA_VALIDATE",
			expectedMetadataURL: metaUrl,
			expectedDataURL:     "",
			expectedError:       errors.New("Cannot connect to SD Connect metadata API: bad url"),
			mockGetEnv: func(s string, b bool) (string, error) {
				return metaUrl, nil
			},
			mockTestURL: func(s string) error {
				return errors.New("bad url")
			},
		},
		{
			testname:            "FAIL_DATA_ENV",
			expectedMetadataURL: metaUrl,
			expectedDataURL:     "",
			expectedError:       errors.New("some error"),
			mockGetEnv: func(s string, b bool) (string, error) {
				if s == envMetaUrl {
					return metaUrl, nil
				} else {
					return "", errors.New("some error")
				}
			},
			mockTestURL: func(s string) error {
				return nil
			},
		},
		{
			testname:            "FAIL_DATA_VALIDATE",
			expectedMetadataURL: metaUrl,
			expectedDataURL:     "http://data.csc.fi",
			expectedError:       errors.New("Cannot connect to SD Connect data API: bad url"),
			mockGetEnv: func(s string, b bool) (string, error) {
				if s == envMetaUrl {
					return metaUrl, nil
				} else {
					return "http://data.csc.fi", nil
				}
			},
			mockTestURL: func(s string) error {
				if s == metaUrl {
					return nil
				} else {
					return errors.New("bad url")
				}
			},
		},
	}

	origGetEnvs := getEnv
	origTestURL := testURL
	defer func() {
		getEnv = origGetEnvs
		testURL = origTestURL
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			// Place mocks
			sd := &sdConnectInfo{}
			getEnv = tt.mockGetEnv
			testURL = tt.mockTestURL

			// Invoke function
			err := sd.getEnvs()

			// Test results
			if err != nil {
				if err.Error() != tt.expectedError.Error() {
					t.Errorf("Function returned incorrect error\nExpected=%v\nReceived=%v", tt.expectedError, err)
				}
			}
			if sd.metadataURL != tt.expectedMetadataURL {
				t.Errorf("metadataURL incorrect. Expected=%v, received=%v", tt.expectedMetadataURL, sd.metadataURL)
			}
			if sd.dataURL != tt.expectedDataURL {
				t.Errorf("dataURL incorrect. Expected=%v, received=%v", tt.expectedDataURL, sd.dataURL)
			}
		})
	}
}

func Test_SDConnect_ValidateLogin_OK(t *testing.T) {
	projects := []Metadata{{56, "pr1"}, {45, "pr56"}, {8, "pr88"}}
	mockT := &mockTokenator{uToken: "uToken"}
	mockC := &mockConnecter{tokenable: mockT, uToken: "uToken", sTokenKey: "s1", sTokenValue: sToken{"sToken", "proj1"},
		projects: projects}
	sd := &sdConnectInfo{connectable: mockC}

	err := sd.validateLogin("user", "pass")
	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
	}
	if sd.token != "dXNlcjpwYXNz" {
		t.Errorf("Token incorrect. Expected=dXNlcjpwYXNz, received=%s", sd.token)
	}
	if sd.uToken != "uToken" {
		t.Errorf("uToken incorrect. Expected=uToken, received=%s", sd.uToken)
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

func Test_SDConnect_ValidateLogin_Fail_GetUToken(t *testing.T) {
	mockT := &mockTokenator{uToken: ""}
	mockC := &mockConnecter{tokenable: mockT}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "Error occurred for SD Connect"
	err := sd.validateLogin("user", "pass")
	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("Function failed\nExpected=%v\nReceived=%v", expectedError, err)
		}
	}
}

func Test_SDConnect_ValidateLogin_Fail_GetProjects(t *testing.T) {
	mockT := &mockTokenator{uToken: "token"}
	mockC := &mockConnecter{tokenable: mockT, uToken: ""}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "Error occurred for SD Connect"
	err := sd.validateLogin("user", "pass")
	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("Function failed\nExpected=%v\nReceived=%v", expectedError, err)
		}
	}
}

func Test_SDConnect_ValidateLogin_No_Projects(t *testing.T) {
	mockT := &mockTokenator{uToken: "token"}
	mockC := &mockConnecter{tokenable: mockT, uToken: "token", projects: nil}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "No projects found for SD Connect"
	err := sd.validateLogin("user", "pass")
	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("Function failed\nExpected=%v\nReceived=%v", expectedError, err)
		}
	}
}

func Test_SDConnect_ValidateLogin_401_Error(t *testing.T) {
	mockT := &mockTokenator{uToken: "token456"}
	mockC := &mockConnecter{tokenable: mockT, projectsErr: &RequestError{StatusCode: 401}}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "getProjects error: API responded with status 401 Unauthorized"
	err := sd.validateLogin("user", "pass")
	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("Function failed\nExpected=%v\nReceived=%v", expectedError, err)
		}
	}
}

func Test_SDConnect_ValidateLogin_500_Error(t *testing.T) {
	mockT := &mockTokenator{uToken: "token456"}
	mockC := &mockConnecter{tokenable: mockT, projectsErr: &RequestError{StatusCode: 500}}
	sd := &sdConnectInfo{connectable: mockC}

	expectedError := "SD Connect is not available, please contact CSC servicedesk"
	err := sd.validateLogin("user", "pass")
	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("Function failed\nExpected=%v\nReceived=%v", expectedError, err)
		}
	}
}

func Test_SDConnect_GetNthLevel_Projects(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

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

func Test_SDConnect_GetLoginMethod(t *testing.T) {
	sd := &sdConnectInfo{}
	loginMethod := sd.getLoginMethod()
	if loginMethod != 0 {
		t.Errorf("Function failed expected=%d, received=%d", 0, loginMethod)
	}
}

func Test_SDConnect_LevelCount(t *testing.T) {
	sd := sdConnectInfo{}
	if lc := sd.levelCount(); lc != 3 {
		t.Errorf("Function failed, expected=3, received=%d", lc)
	}
}

func Test_SDConnect_GetToken(t *testing.T) {
	sd := sdConnectInfo{token: "token"}
	if sdt := sd.getToken(); sdt != "token" {
		t.Errorf("Function failed, expected=token, received=%s", sdt)
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
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
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
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		_ = json.NewDecoder(bytes.NewReader([]byte(`[{"bytes":100,"name":"thingy1"}]`))).Decode(ret)
		return nil
	}
	sd := &sdConnectInfo{}

	// Test
	meta, err := sd.getNthLevel("fspath", "1")
	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
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
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		_ = json.NewDecoder(bytes.NewReader([]byte(`[{"bytes":100,"name":"thingy2"}]`))).Decode(ret)
		return nil
	}
	sd := &sdConnectInfo{}

	// Test
	meta, err := sd.getNthLevel("fspath", "1", "2")
	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
	}
	if meta[0].Bytes != 100 {
		t.Errorf("Function failed, expected=%d, received=%d", 100, meta[0].Bytes)
	}
	if meta[0].Name != "thingy2" {
		t.Errorf("Function failed, expected=%s, received=%s", "thingy2", meta[0].Name)
	}
}

func Test_SDConnect_GetNthLevel_Pass_TokenExpired(t *testing.T) {
	// Mock
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		if token == "expiredToken" {
			return &RequestError{http.StatusUnauthorized}
		} else {
			_ = json.NewDecoder(bytes.NewReader([]byte(`[{"bytes":100,"name":"thingy3"}]`))).Decode(ret)
			return nil
		}
	}
	mockC := &mockConnecter{sTokenKey: "project", sTokenValue: sToken{"freshToken", "project"}}
	sd := &sdConnectInfo{
		connectable: mockC,
		sTokens:     map[string]sToken{"project": {"expiredToken", "project"}},
		projects:    []Metadata{},
	}

	// Test
	meta, err := sd.getNthLevel("sdconnect", "project", "container")
	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
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
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		return &RequestError{http.StatusUnauthorized}
	}
	mockC := &mockConnecter{sTokenKey: "project", sTokenValue: sToken{"freshToken", "project"}}
	sd := &sdConnectInfo{
		connectable: mockC,
		sTokens:     map[string]sToken{"project": {"expiredToken", "project"}},
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

	origMakeRequest := makeRequest
	origCalculateDecryptedSize := calculateDecryptedSize

	defer func() {
		makeRequest = origMakeRequest
		calculateDecryptedSize = origCalculateDecryptedSize
	}()

	calculateDecryptedSize = func(fileSize, headerSize int64) int64 {
		return fileSize - 7
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
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

			var size int64 = tt.initSize
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
		value             interface{}
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

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
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
	expectedHeaders := map[string]string{"Range": "bytes=0-9", "X-Project-ID": "project"}
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		_, _ = io.ReadFull(bytes.NewReader(expectedBody), ret.([]byte))
		// Test that headers were computed properly
		if !reflect.DeepEqual(headers, expectedHeaders) {
			t.Errorf("Function failed, expected=%s, received=%s", expectedHeaders, headers)
		}
		return nil
	}
	sd := &sdConnectInfo{sTokens: map[string]sToken{"project": {"token", "project"}}}

	// Test
	buf := make([]byte, 10)
	err := sd.downloadData([]string{"project", "container", "object"}, buf, 0, 10)

	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
	}
	if !bytes.Equal(buf, expectedBody) {
		t.Errorf("Function failed, expected=%s, received=%s", string(expectedBody), string(buf))
	}
}

func Test_SDConnect_DownloadData_Pass_TokenExpired(t *testing.T) {
	// Mock
	expectedBody := []byte("hellothere")
	expectedHeaders := map[string]string{"Range": "bytes=0-9", "X-Project-ID": "project"}
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		if token == "expiredToken" {
			return &RequestError{http.StatusUnauthorized}
		} else {
			_, _ = io.ReadFull(bytes.NewReader(expectedBody), ret.([]byte))
			// Test that headers were computed properly
			if !reflect.DeepEqual(headers, expectedHeaders) {
				t.Errorf("Function failed, expected=%s, received=%s", expectedHeaders, headers)
			}
			return nil
		}
	}
	mockC := &mockConnecter{sTokenKey: "project", sTokenValue: sToken{"freshToken", "project"}}
	sd := &sdConnectInfo{
		connectable: mockC,
		sTokens:     map[string]sToken{"project": {"expiredToken", "project"}},
		projects:    []Metadata{},
	}

	// Test
	buf := make([]byte, 10)
	err := sd.downloadData([]string{"project", "container", "object"}, buf, 0, 10)

	if err != nil {
		t.Errorf("Function failed, expected no error, received=%v", err)
	}
	if !bytes.Equal(buf, expectedBody) {
		t.Errorf("Function failed, expected=%s, received=%s", string(expectedBody), string(buf))
	}
}
