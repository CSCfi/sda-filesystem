package api

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

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
}

func (c *mockConnecter) getProjects(string) ([]Metadata, error) {
	return nil, nil
}

func (c *mockConnecter) fetchTokens(bool, []Metadata) (string, sync.Map) {
	m := sync.Map{}
	m.Store(c.sTokenKey, c.sTokenValue)
	return c.uToken, m
}

func TestSDConnectGetUToken(t *testing.T) {
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
					return fmt.Errorf("makeRequest() was called with incorrect URL. Expected %q, got %q", tt.url+"/token", url)
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
				t.Errorf("Unscoped token is incorrect. Expected %q, got %q", tt.expectedToken, token)
			}
		})
	}
}

func TestSDConnectGetUToken_Error(t *testing.T) {
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
		t.Errorf("Unscoped token should have been empty, got %q", token)
	}
}

func TestSDConnectGetSToken(t *testing.T) {
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
					return fmt.Errorf("makeRequest() was called with incorrect url. Expected %q, got %q", tt.url+"/token", url)
				}
				if query["project"] != tt.project {
					return fmt.Errorf("makeRequest() was called with incorrect query. Expected key 'project' to have value %q, got %q",
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
				t.Errorf("Scoped token is incorrect. Expected %q, got %q", tt.expectedToken, token.Token)
			} else if tt.expectedID != token.ProjectID {
				t.Errorf("Project ID is incorrect. Expected %q, got %q", tt.expectedID, token.ProjectID)
			}
		})
	}
}

func TestSDConnectGetSToken_Error(t *testing.T) {
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
		t.Errorf("Scoped token should have been empty, got %q", token)
	}
}

func TestSDConnectGetProjects(t *testing.T) {
	var tests = []struct {
		testname, url, token string
		expectedMetaData     []Metadata
	}{
		{
			"OK_1", "google.com", "7ce5ic",
			[]Metadata{{234, "Jack", ""}, {2, "yur586bl", ""}, {7489, "rtu6u__78bgi", "rtu6u//78bgi"}},
		},
		{
			"OK_2", "example.com", "2cjv05fgi",
			[]Metadata{{740, "rtu6u__78boi", "rtu6u//78boi"}, {83, "85cek6o", "hei"}},
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
					return fmt.Errorf("makeRequest() was called with incorrect url. Expected %q, got %q", tt.url+"/projects", url)
				}
				if token != tt.token {
					return fmt.Errorf("makeRequest() was called with incorrect token. Expected %q, got %q", tt.token, token)
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
				t.Errorf("Projects incorrect. Expected %v, got %v", tt.expectedMetaData, projects)
			}
		})
	}
}

func TestSDConnectGetProjects_Error(t *testing.T) {
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
		t.Errorf("Slice should have been empty, got %q", projects)
	}
}

func TestSDConnectFetchTokens(t *testing.T) {
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

			newUToken, newSTokensSync := c.fetchTokens(tt.skip, mockT.keys())

			newSTokens := map[string]sToken{}
			newSTokensSync.Range(func(key, value interface{}) bool {
				newSTokens[fmt.Sprint(key)] = value.(sToken)
				return true
			})

			if newUToken != tt.uToken {
				t.Errorf("uToken incorrect. Expected %q, got %q", tt.uToken, newUToken)
			} else if !reflect.DeepEqual(newSTokens, tt.sTokens) {
				t.Errorf("sTokens incorrect.\nExpected %q\nGot %q", tt.sTokens, newSTokens)
			}
		})
	}
}

func TestSDConnectGetEnvs(t *testing.T) {
	var tests = []struct {
		testname string
		values   [3]string
		failIdx  int
	}{
		{"OK", [3]string{"cert.pem", "https://example.com", "https://google.com"}, -1},
		{"FAIL_1", [3]string{"", "https://example.com", "https://google.com"}, 0},
		{"FAIL_2", [3]string{"cert.pem", "http://example.com", "https://google.com"}, 1},
		{"FAIL_3", [3]string{"cert.pem", "https://example.com", ""}, 2},
	}

	origGetEnvs := getEnv
	defer func() { getEnv = origGetEnvs }()

	envNames := map[string]int{"FS_SD_CONNECT_CERTS": 0, "FS_SD_CONNECT_METADATA_API": 1, "FS_SD_CONNECT_DATA_API": 2}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			sd := &sdConnectInfo{}
			getEnv = func(name string, verifyURL bool) (string, error) {
				if tt.failIdx == envNames[name] {
					return "", errors.New("Error")
				}
				return tt.values[envNames[name]], nil
			}

			if err := sd.getEnvs(); err != nil {
				if tt.testname == "OK" {
					t.Errorf("Unexpected err: %s", err.Error())
				}
			} else if tt.testname != "OK" {
				t.Error("Function should have returned error")
			} else if sd.certPath != tt.values[0] {
				t.Errorf("Incorrect certificate path. Expected %q, got %q", tt.values[0], sd.certPath)
			} else if sd.metadataURL != tt.values[1] {
				t.Errorf("Incorrect metadata URL. Expected %q, got %q", tt.values[1], sd.metadataURL)
			} else if sd.dataURL != tt.values[2] {
				t.Errorf("Incorrect data URL. Expected %q, got %q", tt.values[2], sd.dataURL)
			}
		})
	}
}

func TestSDConnectTestURLs(t *testing.T) {
	var tests = []struct {
		testname, metadataURL, dataURL, failURL string
	}{
		{"OK", "google.com", "finnkino.fi", ""},
		{"FAIL_METADATA", "github.com", "gitlab.com", "github.com"},
		{"FAIL_DATA", "hs.fi", "is.fi", "is.fi"},
	}

	origTestURL := testURL
	defer func() { testURL = origTestURL }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			sd := &sdConnectInfo{metadataURL: tt.metadataURL, dataURL: tt.dataURL}
			testURL = func(url string) error {
				if tt.failURL != "" && url == tt.failURL {
					return errors.New("Error")
				}
				return nil
			}

			if err := sd.testURLs(); err != nil {
				if tt.testname == "OK" {
					t.Errorf("Unexpected error: %s", err.Error())
				}
			} else if tt.testname != "OK" {
				t.Error("Function did not return error")
			}
		})
	}
}

func TestSDConnectValidateLogin(t *testing.T) {
}

func TestSDConnectGetNthLevel_Projects(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	mockC := &mockConnecter{}
	projects := []Metadata{{34, "Pr3", ""}, {90, "Pr56", ""}, {123, "Pr7", ""}, {4, "Pr12", ""}}
	sd := &sdConnectInfo{connectable: mockC, projects: projects}

	meta, err := sd.getNthLevel()
	if err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	} else if !reflect.DeepEqual(meta, projects) {
		t.Errorf("Returned metadata incorrect. Expected %q, got %q", projects, meta)
	}
}

func TestSDConnectGetNthLevel_Containers(t *testing.T) {
	var tests = []struct {
		testname, project, token string
		expectedMetaData         []Metadata
	}{
		{
			"OK", "project345", "new_token",
			[]Metadata{{2341, "tukcdfku6", ""}, {45, "hf678cof7uib68or6", ""}, {6767, "rtu6u78bgi", ""}, {1, "9ob89bio", ""}},
		},
		{
			"OK_EMPTY", "projectID", "7cftlx67", []Metadata{},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			sd := &sdConnectInfo{connectable: &mockConnecter{}}
			sd.sTokens.Store(tt.project, sToken{tt.token, ""})

			makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
				path := "/project/" + tt.project + "/containers"
				if url != path {
					return fmt.Errorf("makeRequest() was called with incorrect URL path. Expected %q, got %q", path, url)
				}
				if token != tt.token {
					return &RequestError{401}
				}

				switch v := ret.(type) {
				case *[]Metadata:
					*v = tt.expectedMetaData
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			}

			if meta, err := sd.getNthLevel(tt.project); err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(meta, tt.expectedMetaData) {
				t.Errorf("Incorrect containers. Expected %q, got %q", tt.expectedMetaData, meta)
			}
		})
	}
}

func TestSDConnectGetNthLevel_Containers_Error(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	makeRequest = func(url string, token string, repository string, query map[string]string, headers map[string]string, ret interface{}) error {
		return errExpected
	}

	sd := &sdConnectInfo{connectable: &mockConnecter{}}
	meta, err := sd.getNthLevel("project")

	if err == nil {
		t.Error("Function did not return error")
	} else if !errors.Is(err, errExpected) {
		t.Errorf("Function returned incorrect error: %s", err.Error())
	}

	if meta != nil {
		t.Errorf("Metadata is non-nil: %q", meta)
	}
}

func TestSDConnectGetNthLevel_Containers_Expired(t *testing.T) {
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	expectedProject := "project737"
	expectedToken := "t7vwlv78"
	expectedContainers := []Metadata{{2341, "tukcdfku6", ""}, {47, "8cxgje6uk", ""}}

	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		path := "/project/" + expectedProject + "/containers"
		if url != path {
			return fmt.Errorf("makeRequest() was called with incorrect URL path. Expected %q, got %q", path, url)
		}
		if token != expectedToken {
			return &RequestError{401}
		}

		switch v := ret.(type) {
		case *[]Metadata:
			*v = expectedContainers
			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
		}
	}

	mockC := &mockConnecter{sTokenKey: expectedProject, sTokenValue: sToken{Token: expectedToken}}
	sd := &sdConnectInfo{connectable: mockC}
	meta, err := sd.getNthLevel(expectedProject)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}
	if !reflect.DeepEqual(meta, expectedContainers) {
		t.Fatalf("Containers incorrect. Expected %v, got %v", expectedContainers, meta)
	}
}

func TestSDConnectGetNthLevel_Objects(t *testing.T) {
	var tests = []struct {
		testname, project, container, token string
		expectedMetaData                    []Metadata
	}{
		{
			"OK", "project345", "containerID", "token",
			[]Metadata{{56, "tukcdfku6", ""}, {5, "hf678cof7ui6", ""}, {47685, "rtu6u__78bgi", ""}, {10, "9ob89bio", ""}},
		},
		{
			"OK_EMPTY", "projectID", "container349", "5voI8d", []Metadata{},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			sd := &sdConnectInfo{connectable: &mockConnecter{}}
			sd.sTokens.Store(tt.project, sToken{tt.token, ""})

			makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
				path := "/project/" + tt.project + "/container/" + tt.container + "/objects"
				if url != path {
					return fmt.Errorf("makeRequest() was called with incorrect URL path\nExpected %q\nGot %q", path, url)
				}
				if token != tt.token {
					return &RequestError{401}
				}

				switch v := ret.(type) {
				case *[]Metadata:
					*v = tt.expectedMetaData
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			}

			if meta, err := sd.getNthLevel(tt.project, tt.container); err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(meta, tt.expectedMetaData) {
				t.Errorf("Incorrect objects. Expected %q, got %q", tt.expectedMetaData, meta)
			}
		})
	}
}

/*func TestSDConnectUpdateAttributes(t *testing.T) {

}

{"user", "pass", "dXNlcjpwYXNz"},
		{"kalmari", "23t&io00_e", "a2FsbWFyaToyM3QmaW8wMF9l"},
		{"qwerty123", "mnbvc456", "cXdlcnR5MTIzOm1uYnZjNDU2"},

func TestGetSpecialHeaders(t *testing.T) {
	var tests = []struct {
		testname, path  string
		mockMakeRequest func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedHeaders SpecialHeaders
	}{
		{
			"FAIL", "project/container/object",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting headers")
			},
			SpecialHeaders{},
		},
		{
			"OK_1", "ich/du/sie",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SpecialHeaders:
					*v = SpecialHeaders{Decrypted: true, HeaderSize: 345, SegmentedObjectSize: 1098}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			SpecialHeaders{Decrypted: true, HeaderSize: 345, SegmentedObjectSize: 1098},
		},
		{
			"OK_2", "left/right/left",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SpecialHeaders:
					*v = SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: 90}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: 90},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			headers, err := GetSpecialHeaders(tt.path)

			if tt.expectedHeaders == (SpecialHeaders{}) {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if tt.expectedHeaders.Decrypted != headers.Decrypted {
				t.Errorf("Field 'Decrypted' incorrect. Expected %t, got %t", tt.expectedHeaders.Decrypted, headers.Decrypted)
			} else if tt.expectedHeaders.HeaderSize != headers.HeaderSize {
				t.Errorf("Field 'HeaderSize' incorrect. Expected %d, got %d", tt.expectedHeaders.HeaderSize, headers.HeaderSize)
			} else if tt.expectedHeaders.SegmentedObjectSize != headers.SegmentedObjectSize {
				t.Errorf("Field 'SegmentedObjectSize' incorrect. Expected %d, got %d",
					tt.expectedHeaders.SegmentedObjectSize, headers.SegmentedObjectSize)
			}
		})
	}
}*/
