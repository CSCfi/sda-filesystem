package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sda-filesystem/internal/cache"
	"sda-filesystem/internal/logs"
	"strconv"
	"strings"
	"time"
)

const chunkSize = 1 << 25

var hi = httpInfo{requestTimeout: 20, httpRetry: 3, fuseInfos: make(map[string]fuseInfo)}
var possibleRepositories = make(map[string]fuseInfo)
var downloadCache *cache.Ristretto

// httpInfo contains all necessary variables used during HTTP requests
type httpInfo struct {
	requestTimeout int
	httpRetry      int
	client         *http.Client
	fuseInfos      map[string]fuseInfo
}

// If you wish to add a new repository, it must implement the following functions
type fuseInfo interface {
	getEnvs() error
	validateLogin(...string) error
	getCertificatePath() string
	testURLs() error
	getToken() string
	isHidden() bool
	getFirstLevel() ([]Metadata, error)
	getSecondLevel(string) ([]Metadata, error)
	getThirdLevel(string, string) ([]Metadata, error)
	updateAttributes([]string, string, interface{})
	downloadData([]string, []byte, int64, int64) error
}

// Metadata contains node metadata fetched from an api
type Metadata struct {
	Bytes int64  `json:"bytes"`
	Name  string `json:"name"`
}

// RequestError is used to obtain the status code from the HTTP request
type RequestError struct {
	StatusCode int
}

func (re *RequestError) Error() string {
	return fmt.Sprintf("API responded with status %d", re.StatusCode)
}

// GetAllPossibleRepositories returns the names of every possible repository.
// Every repository needs to add itself to the possibleRepositories map in an init function
var GetAllPossibleRepositories = func() []string {
	var names []string
	for key := range possibleRepositories {
		names = append(names, key)
	}
	return names
}

// GetEnabledRepositories returns the name of every repository the user has enabled
func GetEnabledRepositories() []string {
	var names []string
	for key := range hi.fuseInfos {
		names = append(names, key)
	}
	return names
}

// AddRepository adds a repository to hi.fuseInfos
var AddRepository = func(r string) {
	hi.fuseInfos[r] = possibleRepositories[r]
}

// RemoveRepository removes a repository from hi.fuseInfos
var RemoveRepository = func(r string) {
	delete(hi.fuseInfos, r)
}

// SetRequestTimeout redefines hi.requestTimeout
var SetRequestTimeout = func(timeout int) {
	hi.requestTimeout = timeout
}

// GetEnvs looks up the necessary environment variables
func GetEnvs() error {
	for i := range hi.fuseInfos {
		if err := hi.fuseInfos[i].getEnvs(); err != nil {
			return err
		}
	}
	return nil
}

// getEnv looks up environment variable given in 'name'
func getEnv(name string, verifyURL bool) (string, error) {
	env, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("Environment variable %q not set", name)
	}
	if verifyURL {
		return env, validURL(env)
	}
	return env, nil
}

func validURL(env string) error {
	u, err := url.ParseRequestURI(env)
	if err != nil {
		return fmt.Errorf("Environment variable %q is an invalid URL: %w", env, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("Environment variable %q does not have scheme 'https'", env)
	}
	return nil
}

// ValidateLogin checks if user is able to log in with given input
var ValidateLogin = func(rep string, auth ...string) error {
	return hi.fuseInfos[rep].validateLogin(auth...)
}

// InitializeCache creates a cache for downloaded data
func InitializeCache() error {
	var err error
	downloadCache, err = cache.NewRistrettoCache()
	if err != nil {
		return fmt.Errorf("Could not create cache: %w", err)
	}
	return nil
}

// InitializeClient initializes a global http client
func InitializeClient() error {
	// Handle certificates if ones are set
	caCertPool := x509.NewCertPool()
	for i := range hi.fuseInfos {
		certPath := hi.fuseInfos[i].getCertificatePath()
		if len(certPath) > 0 {
			caCert, err := ioutil.ReadFile(certPath)
			if err != nil {
				return fmt.Errorf("Reading certificate file %q failed: %w", certPath, err)
			}
			caCertPool.AppendCertsFromPEM(caCert)
		}
	}

	// Set up HTTP client
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxConnsPerHost = 100
	tr.MaxIdleConnsPerHost = 100
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // REMOVE THIS
		RootCAs:            caCertPool,
		MinVersion:         tls.VersionTLS12,
	}
	tr.ForceAttemptHTTP2 = true
	tr.DisableKeepAlives = false

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return fmt.Errorf("Failed to create a cookie jar: %w", err)
	}

	timeout := time.Duration(hi.requestTimeout) * time.Second

	hi.client = &http.Client{
		Timeout:   timeout,
		Transport: tr,
		Jar:       jar,
	}

	logs.Debug("Initializing HTTP client successful")
	return testURLs()
}

// testUrls tests whether we can connect to API urls and that certificates are valid
var testURLs = func() error {
	for fs := range hi.fuseInfos {
		if err := hi.fuseInfos[fs].testURLs(); err != nil {
			return err
		}
	}
	return nil
}

func testURL(url string) error {
	response, err := hi.client.Head(url)
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

// makeRequest sends HTTP requests and parses the responses
func makeRequest(url, token, repository string, query, headers map[string]string, ret interface{}) error {
	var response *http.Response

	// Build HTTP request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Place query params if they are set
	q := request.URL.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	request.URL.RawQuery = q.Encode()

	// Place mandatory headers
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	} else {
		request.Header.Set("Authorization", "Basic "+hi.fuseInfos[repository].getToken())
	}

	// Place additional headers if any are available
	for k, v := range headers {
		request.Header.Set(k, v)
	}

	// Execute HTTP request
	// retry the request as specified by hi.httpRetry variable
	count := 0
	for {
		response, err = hi.client.Do(request)
		logs.Debugf("Trying request %q, attempt %d/%d", request.URL, count+1, hi.httpRetry)
		count++

		if err != nil && count >= hi.httpRetry {
			return err
		}
		if err == nil {
			break
		}
	}
	defer response.Body.Close()

	if response.StatusCode != 200 && response.StatusCode != 206 {
		return &RequestError{response.StatusCode}
	}

	// Parse request
	switch v := ret.(type) {
	case *SpecialHeaders:
		(*v).Decrypted = (response.Header.Get("X-Decrypted") == "True")

		if (*v).Decrypted {
			if headerSize := response.Header.Get("X-Header-Size"); headerSize != "" {
				if (*v).HeaderSize, err = strconv.ParseInt(headerSize, 10, 0); err != nil {
					logs.Warningf("Could not convert header X-Header-Size to integer: %s", err.Error())
					(*v).Decrypted = false
				}
			} else {
				logs.Warningf("Could not find header X-Header-Size in response")
				(*v).Decrypted = false
			}
		}

		if segSize := response.Header.Get("X-Segmented-Object-Size"); segSize != "" {
			if (*v).SegmentedObjectSize, err = strconv.ParseInt(segSize, 10, 0); err != nil {
				logs.Warningf("Could not convert header X-Segmented-Object-Size to integer: %s", err.Error())
				(*v).SegmentedObjectSize = -1
			}
		} else {
			(*v).SegmentedObjectSize = -1
		}

		if _, err = io.Copy(io.Discard, response.Body); err != nil {
			logs.Warningf("Discarding response body failed when reading headers: %s", err.Error())
		}
	case []byte:
		if _, err = io.ReadFull(response.Body, v); err != nil {
			return fmt.Errorf("Copying response failed: %w", err)
		}
	default:
		if err = json.NewDecoder(response.Body).Decode(v); err != nil {
			return fmt.Errorf("Unable to decode response: %w", err)
		}
	}

	logs.Debug("Request ", request.URL, " returned a response")
	return nil
}

// IsHidden returns true if the directories returned by getFirstLevel() should be hidden.
// The children of these directories are visible.
// This is implemented like this so that
// 1. If a repository does not have three levels this is a way to artifitially add a level
// 2. SD-Submit has multiple APIs which is why these APIs need to be added to the hierarchy without being seen by the user
func IsHidden(rep string) bool {
	return hi.fuseInfos[rep].isHidden()
}

// GetFirstLevel returns the topmost directories in repository 'rep'
func GetFirstLevel(rep string) ([]Metadata, error) {
	return hi.fuseInfos[rep].getFirstLevel()
}

// GetSecondLevel returns the directories in repository 'rep' under directory 'dir'
func GetSecondLevel(rep, dir string) ([]Metadata, error) {
	return hi.fuseInfos[rep].getSecondLevel(dir)
}

// GetThirdLevel returns the directories in repository 'rep' under directory 'dir1/dir2'
func GetThirdLevel(rep, dir1, dir2 string) ([]Metadata, error) {
	return hi.fuseInfos[rep].getThirdLevel(dir1, dir2)
}

// UpdateAttributes modifies attributes of node in 'path' in repository 'rep'.
// 'nodes' contains the original names of nodes in 'path'
func UpdateAttributes(nodes []string, path string, attr interface{}) {
	if len(nodes) < 4 {
		logs.Errorf("Invalid path %q. Not deep enough", path)
		return
	}
	hi.fuseInfos[nodes[0]].updateAttributes(nodes[1:], path, attr)
}

// DownloadData requests data between range [start, end) from an API.
func DownloadData(nodes []string, path string, start int64, end int64, maxEnd int64) ([]byte, error) {
	if len(nodes) < 4 {
		return nil, fmt.Errorf("Invalid path %q. Not deep enough", path)
	}

	// chunk index of cache
	chunk := start / chunkSize
	// start coordinate of chunk
	chStart := chunk * chunkSize
	// end coordinate of chunk
	chEnd := (chunk + 1) * chunkSize

	if chEnd > maxEnd {
		chEnd = maxEnd
	}

	ofst := start - chStart
	endofst := end - chStart

	// We create the cache key based on path and requested bytes
	cacheKey := strings.Join(nodes, "_") + "_" + strconv.FormatInt(chStart, 10)
	response, found := downloadCache.Get(cacheKey)

	if !found {
		buf := make([]byte, chEnd-chStart)
		err := hi.fuseInfos[nodes[0]].downloadData(nodes[1:], buf, chStart, chEnd)
		if err != nil {
			return nil, fmt.Errorf("Retrieving data failed for %q: %w", path, err)
		}

		downloadCache.Set(cacheKey, buf, time.Minute*60)
		logs.Debugf("File %q stored in cache, with coordinates %d-%d", path, chStart, chEnd-1)

		if endofst > int64(len(buf)) {
			endofst = int64(len(buf))
		}
		return buf[ofst:endofst], nil
	}

	ret := response.([]byte)
	if endofst > int64(len(ret)) {
		endofst = int64(len(ret))
	}
	logs.Debugf("Retrieved file %q from cache, with coordinates %d-%d", path, start, end-1)
	return ret[ofst:endofst], nil
}
