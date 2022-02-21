package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	certPath              string
	mockDownloadDataBuf   []byte
	mockDownloadDataError error
}

//func (r *mockRepository) getEnvs() error                                    { return nil }
//func (r *mockRepository) getLoginMethod() LoginMethod                       { return Password }
//func (r *mockRepository) validateLogin(...string) error                     { return nil }
//func (r *mockRepository) levelCount() int                                   { return 0 }

func (r *mockRepository) getToken() string { return "" }

//func (r *mockRepository) getNthLevel(string, ...string) ([]Metadata, error) { return nil, nil }
//func (r *mockRepository) updateAttributes([]string, string, interface{})    {}

func (r *mockRepository) downloadData(nodes []string, buf interface{}, start, end int64) error {
	_, _ = io.ReadFull(bytes.NewReader(r.mockDownloadDataBuf), buf.([]byte))
	return r.mockDownloadDataError
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(i int, s []string) {})
	os.Exit(m.Run())
}

func TestRequestError(t *testing.T) {
	codes := []int{200, 206, 404, 500}
	for i := range codes {
		re := RequestError{codes[i]}
		message := fmt.Sprintf("API responded with status %d %s", codes[i], http.StatusText(codes[i]))
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
	origRepositories := hi.repositories
	defer func() { hi.repositories = origRepositories }()

	file1, err := ioutil.TempFile("", "cert")
	if err != nil {
		t.Fatalf("Failed to create file %q", file1.Name())
	}

	file2, err := ioutil.TempFile("", "cert")
	if err != nil {
		t.Fatalf("Failed to create file %q", file2.Name())
	}

	hi.repositories = map[string]fuseInfo{"rep1": &mockRepository{certPath: file1.Name()},
		"rep2": &mockRepository{certPath: file2.Name()}}

	if err := InitializeClient(); err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
}

func TestMakeRequest(t *testing.T) {
	handleCount := 0
	var tests = []struct {
		testname        string
		mockHandlerFunc func(http.ResponseWriter, *http.Request)
		query           map[string]string
		headers         map[string]string
		expectedBody    interface{}
	}{
		{
			testname: "OK_HEADERS",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				rw.Header().Set("X-Decrypted", "True")
				rw.Header().Set("X-Header-Size", "67")
				rw.Header().Set("X-Segmented-Object-Size", "345")
				if _, err := rw.Write([]byte("stuff")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			expectedBody: SpecialHeaders{Decrypted: true, HeaderSize: 67, SegmentedObjectSize: 345},
		},
		{
			testname: "OK_DATA",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if _, err := rw.Write([]byte("This is a message from the past")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			expectedBody: []byte("This is a message from the past"),
		},
		{
			testname: "OK_JSON",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				body, err := json.Marshal([]Metadata{{34, "project1"}, {67, "project/2"}, {8, "project3"}})
				if err != nil {
					rw.WriteHeader(http.StatusNotFound)
				} else {
					if _, err := rw.Write(body); err != nil {
						rw.WriteHeader(http.StatusNotFound)
					}
				}
			},
			expectedBody: []Metadata{{34, "project1"}, {67, "project/2"}, {8, "project3"}},
		},
		{
			testname: "FAIL_JSON",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				_, _ = rw.Write([]byte(""))
			},
			expectedBody: nil,
		},
		{
			testname: "OK_JSON_ADD_QUERY_AND_HEADERS",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				body, err := json.Marshal([]Metadata{{34, "project1"}, {67, "project/2"}, {8, "project3"}})
				if err != nil {
					rw.WriteHeader(http.StatusNotFound)
				} else {
					if _, err := rw.Write(body); err != nil {
						rw.WriteHeader(http.StatusNotFound)
					}
				}
			},
			query:        map[string]string{"some": "thing"},
			headers:      map[string]string{"some": "thing"},
			expectedBody: []Metadata{{34, "project1"}, {67, "project/2"}, {8, "project3"}},
		},
		{
			testname: "HEADERS_MISSING",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if _, err := rw.Write([]byte("stuff")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			expectedBody: SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: -1},
		},
		{
			testname: "FAIL_HEADER_SIZE_PARSE_1",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				rw.Header().Set("X-Decrypted", "True")
				rw.Header().Set("X-Header-Size", "NaN")
			},
			expectedBody: SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: -1},
		},
		{
			testname: "FAIL_HEADER_SIZE_PARSE_2",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				rw.Header().Set("X-Decrypted", "True")
				rw.Header().Set("X-Header-Size", "67")
				rw.Header().Set("X-Segmented-Object-Size", "NaN") // this one fails, but it's not fatal
			},
			expectedBody: SpecialHeaders{Decrypted: true, HeaderSize: 67, SegmentedObjectSize: -1},
		},
		{
			testname: "FAIL_SIZE_HEADER_MISSING",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				rw.Header().Set("X-Decrypted", "True")
			},
			expectedBody: SpecialHeaders{Decrypted: false, HeaderSize: 0, SegmentedObjectSize: -1},
		},
		{
			testname: "FAIL_ONCE",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if handleCount > 0 {
					if _, err := rw.Write([]byte("Hello, I am a robot")); err != nil {
						rw.WriteHeader(http.StatusNotFound)
					}
				} else {
					handleCount++
					http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
				}
			},
			expectedBody: []byte("Hello, I am a robot"),
		},
		{
			testname: "FAIL_ALL",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
			},
			expectedBody: nil,
		},
		{
			testname: "FAIL_400",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				http.Error(rw, "Bad request", 400)
			},
			expectedBody: nil,
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
				err = makeRequest(server.URL, "token", "", tt.query, tt.headers, &headers)
				ret = headers
			case []byte:
				buf := make([]byte, len(v))
				err = makeRequest(server.URL, "token", "mock", tt.query, tt.headers, buf)
				ret = buf
			default:
				var objects []Metadata
				err = makeRequest(server.URL, "", "mock", tt.query, tt.headers, &objects)
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

func TestDownloadData_FoundCache(t *testing.T) {

	// Substitute mock functions
	// Save original functions before test
	origDownloadCache := downloadCache
	// Restore original functions after test
	defer func() {
		downloadCache = origDownloadCache
	}()
	// Overwrite original functions with mock for duration of test
	downloadCache = &cache.Ristretto{Cacheable: &mockCache{}}

	// Save some data to cache
	expectedData := []byte("hellothere")
	downloadCache.Set("sdconnect_project_container_object_0", expectedData, time.Minute*1)

	// Invoke function
	data, err := DownloadData(
		[]string{"sdconnect", "project", "container", "object"},
		"/path/to/file.txt",
		0, 15, 10,
	)

	// Test results
	if err != nil {
		t.Errorf("TestDownloadData_FoundCache expected no error, received=%s", err.Error())
	}
	if !bytes.Equal(data, expectedData) {
		t.Errorf("TestDownloadData_FoundCache expected=%s, received=%s", string(expectedData), string(data))
	}

}

func TestDownloadData_FoundNoCache(t *testing.T) {

	// Substitute mock functions
	// Save original functions before test
	origDownloadCache := downloadCache
	origRepositories := hi.repositories
	// Restore original functions after test
	defer func() {
		downloadCache = origDownloadCache
		hi.repositories = origRepositories
	}()
	// Overwrite original functions with mock for duration of test
	downloadCache = &cache.Ristretto{Cacheable: &mockCache{}}
	expectedData := []byte("hellothere")
	mockRepo := &mockRepository{
		mockDownloadDataBuf:   expectedData,
		mockDownloadDataError: nil,
	}
	hi.repositories = map[string]fuseInfo{"sdconnect": mockRepo}

	// Invoke function
	data, err := DownloadData(
		[]string{"sdconnect", "project", "container", "object"},
		"/path/to/file.txt",
		0, 15, 10,
	)

	// Test results
	if err != nil {
		t.Errorf("TestDownloadData_FoundNoCache expected no error, received=%s", err.Error())
	}
	if !bytes.Equal(data, expectedData) {
		t.Errorf("TestDownloadData_FoundNoCache expected=%s, received=%s", string(expectedData), string(data))
	}

}

func TestDownloadData_FoundNoCache_Error(t *testing.T) {

	// Substitute mock functions
	// Save original functions before test
	origDownloadCache := downloadCache
	origRepositories := hi.repositories
	// Restore original functions after test
	defer func() {
		downloadCache = origDownloadCache
		hi.repositories = origRepositories
	}()
	// Overwrite original functions with mock for duration of test
	downloadCache = &cache.Ristretto{Cacheable: &mockCache{}}
	expectedError := "Retrieving data failed for \"/path/to/file.txt\": some error"
	mockRepo := &mockRepository{
		mockDownloadDataBuf:   nil,
		mockDownloadDataError: errors.New("some error"),
	}
	hi.repositories = map[string]fuseInfo{"sdconnect": mockRepo}

	// Invoke function
	data, err := DownloadData(
		[]string{"sdconnect", "project", "container", "object"},
		"/path/to/file.txt",
		0, 15, 10,
	)

	// Test results
	if err == nil {
		t.Errorf("TestDownloadData_FoundNoCache_Error expected an error, received=nil")
	}
	if err.Error() != expectedError {
		t.Errorf("TestDownloadData_FoundNoCache_Error expected=%s, received=%s", expectedError, err.Error())
	}
	if data != nil {
		t.Errorf("TestDownloadData_FoundNoCache_Error no data, received=%s", string(data))
	}

}

func TestValidURL(t *testing.T) {

	// Test failure
	expectedError := "Environment variable \"something\" is an invalid URL: parse \"something\": invalid URI for request"
	err := validURL("something")
	if err.Error() != expectedError {
		t.Errorf("TestValidURL expected %s received %s", expectedError, err.Error())
	}

	// Test failure
	expectedError = "Environment variable \"http://csc.fi\" does not have scheme 'https'"
	err = validURL("http://csc.fi")
	if err.Error() != expectedError {
		t.Errorf("TestValidURL expected %s received %s", expectedError, err.Error())
	}

	// Test passing
	err = validURL("https://csc.fi")
	if err != nil {
		t.Errorf("TestValidURL expected received error=%s", err.Error())
	}

}
