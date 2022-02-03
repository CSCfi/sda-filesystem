package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sda-filesystem/internal/cache"
	"sda-filesystem/internal/logs"
	"sort"
	"strings"
	"testing"
	"time"
)

var errExpected = errors.New("Expected error for test")

type mockCache struct {
	cache.Cacheable
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

type mockRepository struct {
	fuseInfo
	certPath string
}

//func (r *mockRepository) getEnvs() error                                    { return nil }
//func (r *mockRepository) getLoginMethod() LoginMethod                       { return Password }
//func (r *mockRepository) validateLogin(...string) error                     { return nil }
func (r *mockRepository) getCertificatePath() string { return r.certPath }

//func (r *mockRepository) testURLs() error                                   { return nil }
func (r *mockRepository) getToken() string { return "" }

//func (r *mockRepository) levelCount() int                                   { return 0 }
//func (r *mockRepository) getNthLevel(...string) ([]Metadata, error)         { return nil, nil }
//func (r *mockRepository) updateAttributes([]string, string, interface{})    {}
//func (r *mockRepository) downloadData([]string, []byte, int64, int64) error { return nil }

var testURLs = func() error {
	return nil
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(i int, s []string) {})
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

	possibleRepositories = map[string]fuseInfo{"Pouta": nil, "Pilvi": nil, "Aurinko": nil}
	ans := []string{"Aurinko", "Pilvi", "Pouta"}
	reps := GetAllPossibleRepositories()
	sort.Strings(reps)
	if !reflect.DeepEqual(ans, reps) {
		t.Fatalf("Function returned incorrect value\nExpected %v\nGot %v", ans, reps)
	}
}

func TestGetEnabledRepositories(t *testing.T) {
	origRepositories := hi.repositories
	defer func() { hi.repositories = origRepositories }()

	hi.repositories = map[string]fuseInfo{"Monday": nil, "Friday": nil, "Sunday": nil}
	ans := []string{"Friday", "Monday", "Sunday"}
	reps := GetEnabledRepositories()
	sort.Strings(reps)
	if !reflect.DeepEqual(ans, reps) {
		t.Fatalf("Function returned incorrect value\nExpected %v\nGot %v", ans, reps)
	}
}

/*func TestAddRepository(t *testing.T) {
	origPossibleRepositories := possibleRepositories
	origRepositories := hi.repositories

	defer func() {
		possibleRepositories = origPossibleRepositories
		hi.repositories = origRepositories
	}()

	possibleRepositories = map[string]fuseInfo{"one": nil, "two": nil, "three": nil}
	hi.repositories = map[string]fuseInfo{}

	AddRepository("two")
	if !reflect.DeepEqual(hi.repositories, map[string]fuseInfo{"two": nil}) {

	}
}*/

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

func TestGetEnv(t *testing.T) {
	var tests = []struct {
		testname, envName, envValue string
		verifyURL                   bool
		err                         error
	}{
		{"OK_1", "MUUTTUJA234", "banana", false, nil},
		{"OK_2", "MUUTTUJA9476", "https://github.com", true, nil},
		{"FAIL_INVALID_URL", "MUUTTUJA0346", "http://google.com", true, errExpected},
		{"FAIL_NOT_SET", "MUUTTUJA195", "", false, nil},
	}

	origValidURL := validURL
	defer func() { validURL = origValidURL }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			validURL = func(env string) error { return tt.err }
			if tt.envValue != "" {
				os.Setenv(tt.envName, tt.envValue)
			} else {
				os.Unsetenv(tt.envName)
			}

			value, err := getEnv(tt.envName, tt.verifyURL)
			os.Unsetenv(tt.envName)

			if strings.HasPrefix(tt.testname, "OK") {
				if err != nil {
					t.Errorf("Returned unexpected err: %s", err.Error())
				} else if value != tt.envValue {
					t.Errorf("Environment variable has incorrect value. Expected %q, got %q", tt.envValue, value)
				}
			} else if err == nil {
				t.Error("Function should have returned error")
			} else if tt.err != nil && !errors.Is(err, errExpected) {
				t.Errorf("Function returned incorrect error: %s", err.Error())
			}
		})
	}
}

func TestValidURL(t *testing.T) {
	/*{"WRONG_SCHEME", "MUUTTUJA_2", "http://example.com", true, false},
	{"NO_SCHEME", "OMENA", "google.com", true, false},*/
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
		return nil, errExpected
	}

	err := InitializeCache()
	if err == nil {
		t.Fatalf("Function should have returned non-nil error")
	} else if !errors.Is(err, errExpected) {
		t.Fatalf("Function returned incorrect error: %s", err.Error())
	}
}

func TestInitializeClient(t *testing.T) {
	origTestUrls := testURLs
	origRepositories := hi.repositories

	defer func() {
		testURLs = origTestUrls
		hi.repositories = origRepositories
	}()

	file1, err := ioutil.TempFile("", "cert")
	if err != nil {
		t.Fatalf("Failed to create file %q", file1.Name())
	}

	file2, err := ioutil.TempFile("", "cert")
	if err != nil {
		t.Fatalf("Failed to create file %q", file2.Name())
	}

	testURLs = func() error { return nil }
	hi.repositories = map[string]fuseInfo{"rep1": &mockRepository{certPath: file1.Name()},
		"rep2": &mockRepository{certPath: file2.Name()}}

	if err := InitializeClient(); err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
}

func TestInitializeClient_Certs_Not_Found(t *testing.T) {
	origHTTPInfo := hi

	defer func() {
		hi = origHTTPInfo
	}()

	hi.certPath = "/tmp/path/that/does/not/exist.txt"

	if err := InitializeClient(); err == nil {
		t.Fatalf("Function did not return error")
	}
}

func TestMakeRequest(t *testing.T) {
	handleCount := 0
	var tests = []struct {
		testname        string
		mockHandlerFunc func(http.ResponseWriter, *http.Request)
		expectedBody    interface{}
	}{
		{
			"OK_HEADERS",
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
			"OK_DATA",
			func(rw http.ResponseWriter, req *http.Request) {
				if _, err := rw.Write([]byte("This is a message from the past")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			[]byte("This is a message from the past"),
		},
		{
			"OK_JSON",
			func(rw http.ResponseWriter, req *http.Request) {
				body, err := json.Marshal([]Metadata{{34, "project1", ""}, {67, "project/2", "project_2"}, {8, "project3", ""}})
				if err != nil {
					rw.WriteHeader(http.StatusNotFound)
				} else {
					if _, err := rw.Write(body); err != nil {
						rw.WriteHeader(http.StatusNotFound)
					}
				}
			},
			[]Metadata{{34, "project1", ""}, {67, "project/2", "project_2"}, {8, "project3", ""}},
		},
		{
			"HEADERS_MISSING",
			func(rw http.ResponseWriter, req *http.Request) {
				if _, err := rw.Write([]byte("stuff")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: -1},
		},
		{
			"FAIL_ONCE",
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
			"FAIL_ALL",
			func(rw http.ResponseWriter, req *http.Request) {
				http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
			},
			nil,
		},
		{
			"FAIL_400",
			func(rw http.ResponseWriter, req *http.Request) {
				http.Error(rw, "Bad request", 400)
			},
			nil,
		},
	}

	origClient := hi.client
	origRepositories := hi.repositories

	defer func() {
		hi.client = origClient
		hi.repositories = origRepositories
	}()

	hi.repositories = map[string]fuseInfo{"mock": &mockRepository{}}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandlerFunc))
			hi.client = server.Client()

			// Causes client.Do() to fail when redirecting
			hi.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("Redirecting failed (as expected)")
			}

			var err error
			var ret interface{}
			switch v := tt.expectedBody.(type) {
			case SpecialHeaders:
				var headers SpecialHeaders
				err = makeRequest(server.URL, "token", "", nil, nil, &headers)
				ret = headers
			case []byte:
				buf := make([]byte, len(v))
				err = makeRequest(server.URL, "token", "mock", nil, nil, buf)
				ret = buf
			default:
				var objects []Metadata
				err = makeRequest(server.URL, "", "mock", nil, nil, &objects)
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

/*
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
