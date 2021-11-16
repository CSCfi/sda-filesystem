package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sda-filesystem/internal/cache"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	logrus.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestGetEnvs(t *testing.T) {
	var tests = []struct {
		testName  string
		envValues [3]string
		ok        bool
	}{
		{"OK", [3]string{"cert.pem", "https://example.com", "https://google.com"}, true},
		{"NO_CERT", [3]string{"", "https://example.com", "https://google.com"}, false},
		{"BAD_METADATA_API", [3]string{"cert.pem", "http://example.com", "https://google.com"}, false},
		{"BAD_DATA_API", [3]string{"cert.pem", "https://example.com", ""}, false},
	}

	envNames := []string{"FS_SD_CONNECT_CERTS", "FS_SD_CONNECT_METADATA_API", "FS_SD_CONNECT_DATA_API"}

	for _, tt := range tests {
		testname := tt.testName
		t.Run(testname, func(t *testing.T) {
			origValues := map[string]string{}

			// Define environment variables according to test
			for i := range envNames {
				origValue, ok := os.LookupEnv(envNames[i])

				if ok {
					origValues[envNames[i]] = origValue
				}

				if tt.envValues[i] != "" {
					os.Setenv(envNames[i], tt.envValues[i])
				} else if ok {
					os.Unsetenv(envNames[i])
				}
			}

			err := GetEnvs()

			// Redefine environment variables according to their original values
			for i := range envNames {
				if origValue, ok := origValues[envNames[i]]; ok {
					os.Setenv(envNames[i], origValue)
				} else {
					os.Unsetenv(envNames[i])
				}
			}

			if err != nil {
				if tt.ok {
					t.Errorf("Unexpected err: %s", err.Error())
				}
			} else {
				if !tt.ok {
					t.Error("Test should have returned error")
				}
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	var tests = []struct {
		testName, envName, envValue string
		verifyURL, ok               bool
	}{
		{"OK_1", "ENV_1", "banana", false, true},
		{"OK_2", "MUUTTUJA", "http://example.com", false, true},
		{"OK_3", "ENVAR_2", "https://github.com", true, true},
		{"NOT_SET", "TEST_ENV", "", false, false},
		{"WRONG_SCHEME", "MUUTTUJA_2", "http://example.com", true, false},
		{"NO_SCHEME", "OMENA", "google.com", true, false},
	}

	for _, tt := range tests {
		testname := tt.testName
		t.Run(testname, func(t *testing.T) {
			origValue, ok := os.LookupEnv(tt.envName)

			if tt.envValue != "" {
				os.Setenv(tt.envName, tt.envValue)
			} else {
				os.Unsetenv(tt.envName)
			}

			value, err := getEnv(tt.envName, tt.verifyURL)

			if ok {
				os.Setenv(tt.envName, origValue)
			} else {
				os.Unsetenv(tt.envName)
			}

			if tt.ok {
				if err != nil {
					t.Errorf("Returned unexpected err: %s", err.Error())
				} else if value != tt.envValue {
					t.Errorf("Environment variable has incorrect value. Got %q, expected %q", value, tt.envValue)
				}
			} else if err == nil {
				t.Error("Test should have returned error")
			}
		})
	}
}

func TestCreateToken(t *testing.T) {
	var tests = []struct {
		username, password, token string
	}{
		{"user", "pass", "dXNlcjpwYXNz"},
		{"kalmari", "23t&io00_e", "a2FsbWFyaToyM3QmaW8wMF9l"},
		{"qwerty123", "mnbvc456", "cXdlcnR5MTIzOm1uYnZjNDU2"},
	}

	origToken := hi.token
	defer func() { hi.token = origToken }()

	for i, tt := range tests {
		testname := fmt.Sprintf("TOKEN_%d", i)
		t.Run(testname, func(t *testing.T) {
			hi.token = ""
			CreateToken(tt.username, tt.password)
			if hi.token != tt.token {
				t.Errorf("Username %q and password %q should have returned token %q, got %q", tt.username, tt.password, tt.token, hi.token)
			}
		})
	}
}

func TestMakeRequest(t *testing.T) {
	handleCount := 0 // Bit of a hack, didn't figure out how to do this otherwise

	var tests = []struct {
		testname        string
		mockFetchTokens func()
		mockHandlerFunc func(rw http.ResponseWriter, req *http.Request)
		expectedBody    interface{}
	}{
		{
			"OK_HEADERS", func() {},
			func(rw http.ResponseWriter, req *http.Request) {
				rw.Header().Set("X-Decrypted", "True")
				rw.Header().Set("X-Header-Size", "67")
				rw.Header().Set("X-Segmented-Object-Size", "345")
				if _, err := rw.Write([]byte("stuff")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			SpecialHeaders{Decrypted: true, HeaderSize: 67, SegmentedObjectSize: 345},
		},
		{
			"OK_DATA", func() {},
			func(rw http.ResponseWriter, req *http.Request) {
				if _, err := rw.Write([]byte("This is a message from the past")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			[]byte("This is a message from the past"),
		},
		{
			"OK_JSON", func() {},
			func(rw http.ResponseWriter, req *http.Request) {
				body, err := json.Marshal([]Metadata{{34, "project1"}, {67, "project2"}, {8, "project3"}})
				if err != nil {
					rw.WriteHeader(http.StatusNotFound)
				} else {
					if _, err := rw.Write(body); err != nil {
						rw.WriteHeader(http.StatusNotFound)
					}
				}
			},
			[]Metadata{{34, "project1"}, {67, "project2"}, {8, "project3"}},
		},
		{
			"HEADERS_MISSING", func() {},
			func(rw http.ResponseWriter, req *http.Request) {
				if _, err := rw.Write([]byte("stuff")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: -1},
		},
		{
			"TOKEN_EXPIRED",
			func() { hi.uToken = "new_token" },
			func(rw http.ResponseWriter, req *http.Request) {
				if req.Header.Get("Authorization") == "Bearer new_token" {
					if _, err := rw.Write([]byte("Today is sunny")); err != nil {
						rw.WriteHeader(http.StatusNotFound)
					}
				} else {
					http.Error(rw, "Wrong token", 401)
				}
			},
			[]byte("Today is sunny"),
		},
		{
			"FAIL_ONCE", func() {},
			func(rw http.ResponseWriter, req *http.Request) {
				if handleCount > 0 {
					if _, err := rw.Write([]byte("Hello, I am a robot")); err != nil {
						rw.WriteHeader(http.StatusNotFound)
					}
				} else {
					handleCount++
					http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
				}
			},
			[]byte("Hello, I am a robot"),
		},
		{
			"FAIL_ALL", func() {},
			func(rw http.ResponseWriter, req *http.Request) {
				http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
			},
			nil,
		},
		{
			"FAIL_400", func() {},
			func(rw http.ResponseWriter, req *http.Request) {
				http.Error(rw, "Bad request", 400)
			},
			nil,
		},
	}

	origFetchTokens := FetchTokens
	origClient := hi.client
	origUToken := hi.uToken
	origLoggedIn := hi.loggedIn

	hi.loggedIn = true

	defer func() {
		FetchTokens = origFetchTokens
		hi.client = origClient
		hi.uToken = origUToken
		hi.loggedIn = origLoggedIn
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			FetchTokens = tt.mockFetchTokens
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandlerFunc))
			hi.client = server.Client()
			hi.uToken = ""

			// Causes client.Do() to fail when redirecting
			hi.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("Redirecting failed (as expected)")
			}

			var err error
			var ret interface{}
			switch v := tt.expectedBody.(type) {
			case SpecialHeaders:
				var headers SpecialHeaders
				err = makeRequest(server.URL, func() string { return hi.uToken }, nil, nil, &headers)
				ret = headers
			case []byte:
				buf := make([]byte, len(v))
				err = makeRequest(server.URL, func() string { return hi.uToken }, nil, nil, buf)
				ret = buf
			default:
				var objects []Metadata
				err = makeRequest(server.URL, func() string { return hi.uToken }, nil, nil, &objects)
				ret = objects
			}

			if tt.expectedBody == nil {
				if err == nil {
					t.Errorf("Function did not return error")
				}
			} else if err != nil {
				t.Errorf("Function returned error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedBody, ret) {
				t.Errorf("Incorrect response body.\nExpected %v\nGot %v", tt.expectedBody, ret)
			}

			server.Close()
		})
	}
}

func TestFetchTokens(t *testing.T) {
	var tests = []struct {
		mockGetUToken    func() error
		mockGetProjects  func() ([]Metadata, error)
		mockGetSToken    func(project string) error
		sTokens          map[string]SToken
		uToken, testname string
	}{
		{
			func() error {
				hi.uToken = "uToken"
				return nil
			},
			func() ([]Metadata, error) {
				return []Metadata{{Name: "project1", Bytes: 234}, {Name: "project2", Bytes: 52}, {Name: "project3", Bytes: 90}}, nil
			},
			func(project string) error {
				hi.sTokens[project] = SToken{project + "_token", "435" + project}
				return nil
			},
			map[string]SToken{"project1": {"project1_token", "435project1"},
				"project2": {"project2_token", "435project2"},
				"project3": {"project3_token", "435project3"}},
			"uToken", "OK",
		},
		{
			func() error {
				hi.uToken = "uToken"
				return errors.New("Error occurred")
			},
			func() ([]Metadata, error) {
				return []Metadata{{Name: "project1", Bytes: 234}, {Name: "project2", Bytes: 52}, {Name: "project3", Bytes: 90}}, nil
			},
			func(project string) error {
				hi.sTokens[project] = SToken{project + "_token", "435" + project}
				return nil
			},
			map[string]SToken{},
			"", "UTOKEN_ERROR",
		},
		{
			func() error {
				hi.uToken = "new_token"
				return nil
			},
			func() ([]Metadata, error) {
				return nil, errors.New("Error")
			},
			func(project string) error {
				hi.sTokens[project] = SToken{project + "_secret", "890" + project}
				return nil
			},
			map[string]SToken{},
			"new_token", "PROJECTS_ERROR",
		},
		{
			func() error {
				hi.uToken = "another_token"
				return nil
			},
			func() ([]Metadata, error) {
				return []Metadata{{Name: "pr1", Bytes: 43}, {Name: "pr2", Bytes: 51}, {Name: "pr3", Bytes: 900}}, nil
			},
			func(project string) error {
				if project == "pr2" {
					hi.sTokens[project] = SToken{project + "_secret", "890" + project}
					return errors.New("New error")
				}
				hi.sTokens[project] = SToken{"secret_token", "cactus"}
				return nil
			},
			map[string]SToken{"pr1": {"secret_token", "cactus"}, "pr3": {"secret_token", "cactus"}},
			"another_token", "STOKEN_ERROR",
		},
	}

	origGetUToken := GetUToken
	origGetProjects := GetProjects
	origGetSToken := GetSToken
	origUToken := hi.uToken
	origSTokens := hi.sTokens

	defer func() {
		GetUToken = origGetUToken
		GetProjects = origGetProjects
		GetSToken = origGetSToken
		hi.uToken = origUToken
		hi.sTokens = origSTokens
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			GetUToken = tt.mockGetUToken
			GetProjects = tt.mockGetProjects
			GetSToken = tt.mockGetSToken

			hi.uToken = ""
			hi.sTokens = map[string]SToken{}

			FetchTokens()

			if hi.uToken != tt.uToken {
				t.Errorf("uToken incorrect. Expected %q, got %q", tt.uToken, hi.uToken)
			} else if !reflect.DeepEqual(hi.sTokens, tt.sTokens) {
				t.Errorf("sTokens incorrect.\nExpected %q\nGot %q", tt.sTokens, hi.sTokens)
			}
		})
	}
}

func TestGetUToken(t *testing.T) {
	var tests = []struct {
		testname        string
		mockMakeRequest func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedToken   string
	}{
		{
			"FAIL",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting token")
			},
			"",
		},
		{
			"OK_1",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *UToken:
					*v = UToken{"myveryowntoken"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *UToken", reflect.TypeOf(v))
				}
			},
			"myveryowntoken",
		},
		{
			"OK_2",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *UToken:
					*v = UToken{"9765rty5678"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *UToken", reflect.TypeOf(v))
				}
			},
			"9765rty5678",
		},
	}

	origMakeRequest := makeRequest
	origUToken := hi.uToken

	defer func() {
		makeRequest = origMakeRequest
		hi.uToken = origUToken
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest
			hi.uToken = ""

			err := GetUToken()

			if tt.expectedToken == "" {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if tt.expectedToken != hi.uToken {
				t.Errorf("Unscoped token is incorrect. Expected %q, got %q", tt.expectedToken, hi.uToken)
			}
		})
	}
}

func TestGetSToken(t *testing.T) {
	var tests = []struct {
		testname, project string
		mockMakeRequest   func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedToken     string
		expectedID        string
	}{
		{
			"FAIL", "",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting token")
			},
			"", "",
		},
		{
			"OK_1", "project007",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SToken:
					*v = SToken{"myveryowntoken", "jbowegxf72nfbof"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *SToken", reflect.TypeOf(v))
				}
			},
			"myveryowntoken", "jbowegxf72nfbof",
		},
		{
			"OK_2", "projectID",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *SToken:
					*v = SToken{"9765rty5678", "ug8392nzdipqz9210z"}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *SToken", reflect.TypeOf(v))
				}
			},
			"9765rty5678", "ug8392nzdipqz9210z",
		},
	}

	origMakeRequest := makeRequest
	origSTokens := hi.sTokens

	defer func() {
		makeRequest = origMakeRequest
		hi.sTokens = origSTokens
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest
			hi.sTokens = make(map[string]SToken)

			err := GetSToken(tt.project)

			if tt.expectedToken == "" {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if _, ok := hi.sTokens[tt.project]; !ok {
				t.Errorf("Scoped token for %q is not defined", tt.project)
			} else if tt.expectedToken != hi.sTokens[tt.project].Token {
				t.Errorf("Scoped token is incorrect. Expected %q, got %q", tt.expectedToken, hi.sTokens[tt.project].Token)
			} else if tt.expectedID != hi.sTokens[tt.project].ProjectID {
				t.Errorf("Project ID is incorrect. Expected %q, got %q", tt.expectedID, hi.sTokens[tt.project].ProjectID)
			}
		})
	}
}

func TestGetProjects(t *testing.T) {
	var tests = []struct {
		testname         string
		mockMakeRequest  func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedMetaData []Metadata
	}{
		{
			"FAIL",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting projects")
			},
			nil,
		},
		{
			"EMPTY",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{},
		},
		{
			"OK",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{{234, "Jack"}, {2, "yur586bl"}, {7489, "rtu6u__78bgi"}}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{{234, "Jack"}, {2, "yur586bl"}, {7489, "rtu6u__78bgi"}},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			projects, err := GetProjects()

			if tt.expectedMetaData == nil {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, projects) {
				t.Errorf("Projects incorrect. Expected %v, got %v", tt.expectedMetaData, projects)
			}
		})
	}
}

func TestGetContainers(t *testing.T) {
	var tests = []struct {
		testname, project string
		mockMakeRequest   func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedMetaData  []Metadata
	}{
		{
			"FAIL", "",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting containers")
			},
			nil,
		},
		{
			"EMPTY", "projectID",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{},
		},
		{
			"OK", "project345",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{{2341, "tukcdfku6"}, {45, "hf678cof7uib68or6"}, {6767, "rtu6u__78bgi"}, {1, "9ob89bio"}}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{{2341, "tukcdfku6"}, {45, "hf678cof7uib68or6"}, {6767, "rtu6u__78bgi"}, {1, "9ob89bio"}},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			containers, err := GetContainers(tt.project)

			if tt.expectedMetaData == nil {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, containers) {
				t.Errorf("Containers incorrect. Expected %v, got %v", tt.expectedMetaData, containers)
			}
		})
	}
}

func TestGetObjects(t *testing.T) {
	var tests = []struct {
		testname, project, container string
		mockMakeRequest              func(string, func() string, map[string]string, map[string]string, interface{}) error
		expectedMetaData             []Metadata
	}{
		{
			"FAIL", "", "",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("error getting containers")
			},
			nil,
		},
		{
			"EMPTY", "projectID", "container349",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{},
		},
		{
			"OK", "project345", "containerID",
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				switch v := ret.(type) {
				case *[]Metadata:
					*v = []Metadata{{56, "tukcdfku6"}, {5, "hf678cof7uib68or6"}, {47685, "rtu6u__78bgi"}, {10, "9ob89bio"}}
					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *[]Metadata", reflect.TypeOf(v))
				}
			},
			[]Metadata{{56, "tukcdfku6"}, {5, "hf678cof7uib68or6"}, {47685, "rtu6u__78bgi"}, {10, "9ob89bio"}},
		},
	}

	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest

			objects, err := GetObjects(tt.project, tt.container)

			if tt.expectedMetaData == nil {
				if err == nil {
					t.Errorf("Expected non-nil error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedMetaData, objects) {
				t.Errorf("Objects incorrect. Expected %v, got %v", tt.expectedMetaData, objects)
			}
		})
	}
}

type mockCache struct {
	data []byte
	key  string
}

func (c *mockCache) Get(key string) (interface{}, bool) {
	if c.key == key && c.data != nil {
		return c.data, true
	}
	return nil, false
}

func (c *mockCache) Set(key string, value interface{}, ttl time.Duration) bool {
	c.key = key
	c.data = value.([]byte)
	return true
}

func (c *mockCache) Del(key string) {
}

func TestDownloadData(t *testing.T) {
	var tests = []struct {
		mockMakeRequest     func(string, func() string, map[string]string, map[string]string, interface{}) error
		data, response      []byte
		start, end, maxEnd  int64
		key, path, testname string
	}{
		// Data in cache
		{
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("Should not use makerequest")
			},
			[]byte{'i', 'a', 'm', 'i', 'n', 'f', 'o', 'r', 'm', 'a', 't', 'i', 'o', 'n', '2', '0', '!', '9'},
			[]byte{'i', 'n', 'f', 'o', 'r', 'm', 'a', 't'},
			3, 11, 18,
			"project_container_object_0", "project/container/object", "OK_1",
		},
		// Data not in cache
		{
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				copy(ret.([]byte), []byte{'i', 'e', 'm', 'z', 'q', 'f', 'd', 'r', 'm', 'k', 't', 'i', 'o', 'n', '3', '_', '+'})
				return nil
			},
			nil, []byte{'e', 'm', 'z', 'q', 'f', 'd', 'r'},
			chunkSize + 1, chunkSize + 8, chunkSize + 17,
			"", "monday/tuesday/wednesay", "OK_2",
		},
		// Makerequest returns an error
		{
			func(url string, tokenFunc func() string, query map[string]string, headers map[string]string, ret interface{}) error {
				return errors.New("Someting went wrong")
			},
			nil, nil,
			3, 10, 17,
			"project_container_object_0", "project/container/object", "MAKEREQUEST_ERROR",
		},
	}

	origMakeRequest := makeRequest
	origDownloadCache := downloadCache

	defer func() {
		makeRequest = origMakeRequest
		downloadCache = origDownloadCache
	}()

	downloadCache = &cache.Ristretto{Cacheable: &mockCache{}}

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			makeRequest = tt.mockMakeRequest
			downloadCache.Set(tt.key, tt.data, -1)

			ret, err := DownloadData(tt.path, tt.start, tt.end, tt.maxEnd)

			if tt.response == nil {
				if err == nil {
					t.Errorf("Function should have returned an error")
				}
			} else if err != nil {
				t.Errorf("Function returned an error: %s", err.Error())
			} else if !bytes.Equal(ret, tt.response) {
				t.Errorf("Response incorrect. Expected %v, got %v", tt.response, ret)
			}
		})
	}
}
