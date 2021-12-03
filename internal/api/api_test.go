package api

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

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

func TestMain(m *testing.M) {
	logrus.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestRequestError(t *testing.T) {
	codes := []int{200, 206, 404, 500}
	for i := range codes {
		re := RequestError{codes[i]}
		message := fmt.Sprintf("API responded with status %d", codes[i])
		reMessage := re.Error()
		if reMessage != message {
			t.Fatalf("RequestError has incorrect error message. Expectedd %q, got %q", message, reMessage)
		}
	}
}

func TestGetAllPossibleRepositories(t *testing.T) {
	origPossibleRepositories := possibleRepositories
	defer func() { possibleRepositories = origPossibleRepositories }()

	possibleRepositories = make(map[string]fuseInfo)
	possibleRepositories["Pouta"] = nil
	possibleRepositories["Pilvi"] = nil
	possibleRepositories["Aurinko"] = nil

	origReps := []string{"Aurinko", "Pilvi", "Pouta"}
	reps := GetAllPossibleRepositories()
	sort.Strings(reps)
	if !reflect.DeepEqual(origReps, reps) {
		t.Fatalf("Function returned incorrect value\nExpected %v\nGot %v", origReps, reps)
	}
}

func TestRequestTimeout(t *testing.T) {
	timeouts := []int{34, 6, 1200, 84}

	for i := range timeouts {
		SetRequestTimeout(timeouts[i])
		if hi.requestTimeout != timeouts[i] {
			t.Fatalf("Incorrect request timeout. Expected %d, got %d", timeouts[i], hi.requestTimeout)
		}
	}
}

func TestGetEnvs(t *testing.T) {

}

/*func TestGetEnvs(t *testing.T) {
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

func TestInitializeCache(t *testing.T) {
	origNewRistretto := cache.NewRistrettoCache
	defer func() { cache.NewRistrettoCache = origNewRistretto }()

	newCache := &cache.Ristretto{Cacheable: &mockCache{}}
	cache.NewRistrettoCache = func() (*cache.Ristretto, error) {
		return newCache, nil
	}

	err := InitializeCache()
	if err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
	if downloadCache != newCache {
		t.Fatalf("downloadCache does not point to new cache")
	}
}

func TestInitializeCache_Error(t *testing.T) {
	origNewRistretto := cache.NewRistrettoCache
	defer func() { cache.NewRistrettoCache = origNewRistretto }()

	cache.NewRistrettoCache = func() (*cache.Ristretto, error) {
		return nil, errors.New("Creating cache failed")
	}

	err := InitializeCache()
	if err == nil {
		t.Fatalf("Function should have returned non-nil error")
	}
}

func TestInitializeClient(t *testing.T) {
	origTestUrls := testUrls
	origCertPath := hi.certPath

	defer func() {
		testUrls = origTestUrls
		hi.certPath = origCertPath
	}()

	testUrls = func() error { return nil }

	file, err := ioutil.TempFile("", "cert")
	if err != nil {
		t.Fatalf("Failed to create file %q", file.Name())
	}

	err = InitializeClient()

	if err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
	//if hi.client.
}

func TestInitializeClient_Empty_Certs(t *testing.T) {
	origTestUrls := testUrls
	origCertPath := hi.certPath

	defer func() {
		testUrls = origTestUrls
		hi.certPath = origCertPath
	}()

	testUrls = func() error { return nil }
	hi.certPath = ""

	err := InitializeClient()

	if err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
}

func TestInitializeClient_Certs_Not_Found(t *testing.T) {
	origTestUrls := testUrls
	origCertPath := hi.certPath

	defer func() {
		testUrls = origTestUrls
		hi.certPath = origCertPath
	}()

	testUrls = func() error { return nil }
	hi.certPath = ""

	err := InitializeClient()

	if err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
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
			func() { hi.ci.uToken = "new_token" },
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
	origUToken := hi.ci.uToken
	origLoggedIn := hi.loggedIn

	hi.loggedIn = true

	defer func() {
		FetchTokens = origFetchTokens
		hi.client = origClient
		hi.ci.uToken = origUToken
		hi.loggedIn = origLoggedIn
	}()

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			FetchTokens = tt.mockFetchTokens
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandlerFunc))
			hi.client = server.Client()
			hi.ci.uToken = ""

			// Causes client.Do() to fail when redirecting
			hi.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("Redirecting failed (as expected)")
			}

			var err error
			var ret interface{}
			switch v := tt.expectedBody.(type) {
			case SpecialHeaders:
				var headers SpecialHeaders
				err = makeRequest(server.URL, func() string { return hi.ci.uToken }, nil, nil, &headers)
				ret = headers
			case []byte:
				buf := make([]byte, len(v))
				err = makeRequest(server.URL, func() string { return hi.ci.uToken }, nil, nil, buf)
				ret = buf
			default:
				var objects []Metadata
				err = makeRequest(server.URL, func() string { return hi.ci.uToken }, nil, nil, &objects)
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
*/
