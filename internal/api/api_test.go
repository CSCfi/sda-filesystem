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
	"sort"
	"testing"
	"time"

	"sda-filesystem/internal/cache"
	"sda-filesystem/internal/logs"
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
	envError              error
	mockDownloadDataBuf   []byte
	mockDownloadDataError error
}

func (r *mockRepository) getEnvs() error { return r.envError }

func (r *mockRepository) getToken() string { return "" }

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
			t.Fatalf("RequestError has incorrect error message. Expected=%s, received=%s", message, reMessage)
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
		t.Fatalf("Function returned incorrect value\nExpected=%v\nReceived=%v", ans, reps)
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
		t.Fatalf("Function returned incorrect value\nExpected=%v\nReceived=%v", ans, reps)
	}
}

func TestAddRepository(t *testing.T) {
	origPossibleRepositories := possibleRepositories
	origRepositories := hi.repositories

	defer func() {
		possibleRepositories = origPossibleRepositories
		hi.repositories = origRepositories
	}()

	twoRep := &mockRepository{}
	threeRep := &mockRepository{envError: errExpected}
	possibleRepositories = map[string]fuseInfo{"One": nil, "Two": twoRep, "Three": threeRep}
	hi.repositories = map[string]fuseInfo{}

	err := AddRepository("Two")
	if err != nil {
		t.Fatalf("Function returned error for repository Two: %s", err.Error())
	}
	expectedReps := map[string]fuseInfo{"Two": twoRep}
	if !reflect.DeepEqual(hi.repositories, expectedReps) {
		t.Fatalf("Function did not add repository correctly. Expected=%v, received=%v", expectedReps, hi.repositories)
	}

	err = AddRepository("Three")
	if err == nil {
		t.Fatalf("Function did not return error")
	}
	if err.Error() != errExpected.Error() {
		t.Fatalf("Function returned incorrect error\nExpected=%s\nReceived=%s", errExpected.Error(), err.Error())
	}
	if !reflect.DeepEqual(hi.repositories, expectedReps) {
		t.Fatalf("Function did not add repository correctly. Expected=%v, received=%v", expectedReps, hi.repositories)
	}
}

func TestRequestTimeout(t *testing.T) {
	timeouts := []int{34, 6, 1200, 84}

	for i := range timeouts {
		SetRequestTimeout(timeouts[i])
		if hi.requestTimeout != timeouts[i] {
			t.Fatalf("Incorrect request timeout. Expected=%d, received=%d", timeouts[i], hi.requestTimeout)
		}
	}
}

func TestGetEnv(t *testing.T) {
	var tests = []struct {
		testname, envName, envValue, errText string
		verifyURL                            bool
		err                                  error
	}{
		{"OK_1", "MUUTTUJA234", "banana", "", false, nil},
		{"OK_2", "MUUTTUJA9476", "https://github.com", "", true, nil},
		{"FAIL_INVALID_URL", "MUUTTUJA0346", "http://google.com", errExpected.Error(), true, errExpected},
		{"FAIL_NOT_SET", "MUUTTUJA195", "", "Environment variable MUUTTUJA195 not set", false, nil},
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

			if tt.errText == "" {
				if err != nil {
					t.Errorf("Returned unexpected err: %s", err.Error())
				} else if value != tt.envValue {
					t.Errorf("Environment variable has incorrect value. Expected=%s, received=%s", tt.envValue, value)
				}
			} else if err == nil {
				t.Error("Function should have returned error")
			} else if err.Error() != tt.errText {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errText, err.Error())
			}
		})
	}
}

func TestValidURL(t *testing.T) {
	// Test failure
	expectedError := "Environment variable something is an invalid URL: parse \"something\": invalid URI for request"
	err := validURL("something")
	if err.Error() != expectedError {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}

	// Test failure
	expectedError = "Environment variable http://csc.fi does not have scheme 'https'"
	err = validURL("http://csc.fi")
	if err.Error() != expectedError {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", expectedError, err.Error())
	}

	// Test passing
	err = validURL("https://csc.fi")
	if err != nil {
		t.Errorf("Function received unexpected error: %s", err.Error())
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
	errText := "Could not create cache: " + errExpected.Error()

	err := InitializeCache()
	if err == nil {
		t.Fatalf("Function should have returned error")
	} else if err.Error() != errText {
		t.Fatalf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
	}
}

func TestInitializeClient(t *testing.T) {
	origRepositories := hi.repositories
	origCertPath := hi.certPath
	defer func() {
		hi.repositories = origRepositories
		hi.certPath = origCertPath
	}()

	file, err := ioutil.TempFile("", "cert")
	if err != nil {
		t.Fatalf("Failed to create file %s", file.Name())
	}
	defer os.RemoveAll(file.Name())

	hi.certPath = file.Name()
	hi.repositories = map[string]fuseInfo{"rep1": &mockRepository{}, "rep2": &mockRepository{}}

	if err := InitializeClient(); err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	}
}

func TestInitializeClient_Error(t *testing.T) {
	origRepositories := hi.repositories
	origCertPath := hi.certPath
	defer func() {
		hi.repositories = origRepositories
		hi.certPath = origCertPath
	}()

	file, err := ioutil.TempFile("", "cert")
	if err != nil {
		t.Fatalf("Failed to create file %s", file.Name())
	}
	os.RemoveAll(file.Name())

	hi.certPath = file.Name()
	hi.repositories = map[string]fuseInfo{"rep1": &mockRepository{}, "rep2": &mockRepository{}}
	errText := fmt.Sprintf("Reading certificate file failed: open %s: no such file or directory", hi.certPath)

	if err := InitializeClient(); err == nil {
		t.Error("Function should have returned error")
	} else if err.Error() != errText {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
	}
}

type testTransport struct {
	err error
}

func (t testTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{}, t.err
}

func TestTestURL(t *testing.T) {
	var tests = []struct {
		testname string
		err      error
	}{
		{"OK", nil},
		{"FAIL", errExpected},
	}

	origClient := hi.client
	defer func() { hi.client = origClient }()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			hi.client = &http.Client{
				Transport: testTransport{err: tt.err},
			}

			err := testURL("test_url")
			if tt.err == nil && err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if tt.err != nil {
				if err == nil {
					t.Errorf("Function should have returned error")
				} else if !errors.Is(err, errExpected) {
					t.Errorf("Function returned incorrect error: %s", err.Error())
				}
			}
		})
	}
}

func TestMakeRequest(t *testing.T) {
	handleCount := 0
	var tests = []struct {
		testname, errText string
		mockHandlerFunc   func(http.ResponseWriter, *http.Request)
		query             map[string]string
		headers           map[string]string
		expectedBody      interface{}
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
			testname: "FAIL_DATA",
			errText:  "Copying response failed: unexpected EOF",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if _, err := rw.Write([]byte("This is a message")); err != nil {
					rw.WriteHeader(http.StatusNotFound)
				}
			},
			expectedBody: []byte("This is a message from the past"),
		},
		{
			testname: "FAIL_JSON",
			errText:  "Unable to decode response: EOF",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				_, _ = rw.Write([]byte(""))
			},
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
			errText:  "Get \"https://google.com\": Redirecting failed (as expected)",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
			},
		},
		{
			testname: "FAIL_400",
			errText:  "API responded with status 400 Bad Request",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				http.Error(rw, "Bad request", 400)
			},
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

			if tt.errText != "" {
				if err == nil {
					t.Errorf("Function did not return error")
				} else if err.Error() != tt.errText {
					t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errText, err.Error())
				}
			} else if err != nil {
				t.Errorf("Function returned unexpected error: %s", err.Error())
			} else if !reflect.DeepEqual(tt.expectedBody, ret) {
				t.Errorf("Incorrect response body\nExpected=%v\nReceived=%v", tt.expectedBody, ret)
			}

			server.Close()
		})
	}
}

func TestMakeRequest_NewRequest_Error(t *testing.T) {
	buf := make([]byte, 5)
	buf[0] = 0x7f
	errText := fmt.Sprintf("parse %q: net/url: invalid control character in URL", string(buf))

	if err := makeRequest(string(buf), "token", "mock", nil, nil, buf); err == nil {
		t.Error("Function did not return error with invalid URL")
	} else if err.Error() != errText {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
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
	expectedError := "Retrieving data failed for /path/to/file.txt: some error"
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
