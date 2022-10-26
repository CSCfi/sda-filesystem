package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sda-filesystem/internal/cache"
	"sda-filesystem/internal/logs"
)

const chunkSize = 1 << 25

var hi = httpInfo{requestTimeout: 20, httpRetry: 3, repositories: make(map[string]fuseInfo)}
var allRepositories = make(map[string]fuseInfo)
var downloadCache *cache.Ristretto

// httpInfo contains all necessary variables used during HTTP requests
type httpInfo struct {
	requestTimeout int
	httpRetry      int
	certPath       string
	basicToken     string
	sdsToken       string
	client         *http.Client
	repositories   map[string]fuseInfo
}

// If you wish to add a new repository, it must implement the following functions
type fuseInfo interface {
	getEnvs() error
	validateLogin(...string) error
	levelCount() int
	getNthLevel(string, ...string) ([]Metadata, error)
	updateAttributes([]string, string, any) error
	downloadData([]string, any, int64, int64) error
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
	return fmt.Sprintf("API responded with status %d %s", re.StatusCode, http.StatusText(re.StatusCode))
}

// GetAllRepositories returns the names of every possible repository.
// Every repository needs to add itself to the allRepositories map in an init function
var GetAllRepositories = func() []string {
	var names []string
	for key := range allRepositories {
		names = append(names, key)
	}
	return names
}

// GetEnabledRepositories returns the name of every repository the user has enabled
var GetEnabledRepositories = func() []string {
	var names []string
	for key := range hi.repositories {
		names = append(names, key)
	}
	return names
}

// SetRequestTimeout redefines hi.requestTimeout
var SetRequestTimeout = func(timeout int) {
	hi.requestTimeout = timeout
}

// GetEnvs gets the environment variables for repository 'r'
var GetEnvs = func(r string) error {
	return allRepositories[r].getEnvs()
}

// GetEnv looks up environment variable given in 'name'
var GetEnv = func(name string, verifyURL bool) (string, error) {
	env, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("Environment variable %s not set", name)
	}
	if verifyURL {
		if err := validURL(env); err != nil {
			return "", fmt.Errorf("Environment variable %s not a valid URL: %w", name, err)
		}
	}
	return env, nil
}

var validURL = func(value string) error {
	u, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("%s is an invalid URL: %w", value, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%s does not have scheme 'https'", value)
	}
	return nil
}

// GetCommonEnvs gets the environmental varibles that are needed for all repositories
// This may need to be changed if new repositories are added
func GetCommonEnvs() (err error) {
	if hi.certPath, err = GetEnv("FS_CERTS", false); err != nil {
		return err
	}
	if hi.sdsToken, err = GetEnv("SDS_ACCESS_TOKEN", false); err != nil {
		return err
	}
	return nil
}

var GetSDSToken = func() string {
	return hi.sdsToken
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
	if len(hi.certPath) > 0 {
		caCert, err := os.ReadFile(hi.certPath)
		if err != nil {
			return fmt.Errorf("Reading certificate file failed: %w", err)
		}
		caCertPool.AppendCertsFromPEM(caCert)
	} else {
		caCertPool = nil
	}

	// Set up HTTP client
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxConnsPerHost = 100
	tr.MaxIdleConnsPerHost = 100
	tr.TLSClientConfig = &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS12,
	}
	tr.ForceAttemptHTTP2 = true
	tr.DisableKeepAlives = false

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return fmt.Errorf("Failed to create a cookie jar: %w", err)
	}

	hi.client = &http.Client{
		Transport: tr,
		Jar:       jar,
	}

	logs.Debug("Initializing HTTP client successful")
	return nil
}

var testURL = func(url string) error {
	response, err := hi.client.Head(url)
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

var SetBasicToken = func(username, password string) {
	hi.basicToken = base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// ValidateLogin checks if user is able to log in with given input
var ValidateLogin = func(username, password string) (bool, error) {
	SetBasicToken(username, password)
	err := allRepositories[SDConnect].validateLogin(hi.basicToken)
	if err != nil {
		return false, err
	}
	hi.repositories[SDConnect] = allRepositories[SDConnect]

	// SDSubmit not necessary if it is unavailable
	err = allRepositories[SDSubmit].validateLogin()
	if err == nil {
		hi.repositories[SDSubmit] = allRepositories[SDSubmit]
	}
	return true, err
}

// LevelCount returns the amount of levels repository 'rep' has
var LevelCount = func(rep string) int {
	return hi.repositories[rep].levelCount()
}

// makeRequest sends HTTP requests and parses the responses
var MakeRequest = func(url string, query, headers map[string]string, body io.Reader, ret any) error {
	var response *http.Response

	// Build HTTP request
	var err error
	var timeout time.Duration
	var request *http.Request

	if body != nil {
		request, err = http.NewRequest("PUT", url, body)
		timeout = time.Duration(hi.requestTimeout) * time.Second
	} else {
		request, err = http.NewRequest("GET", url, nil)
		timeout = time.Duration(108000) * time.Second
	}

	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	request = request.WithContext(ctx)

	// Place query params if they are set
	q := request.URL.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	request.URL.RawQuery = q.Encode()

	if body == nil {
		request.Header.Set("Authorization", "Bearer "+hi.sdsToken)
	} else {
		request.Header.Set("Authorization", "Basic "+hi.basicToken)
	}

	// Place additional headers if any are available
	for k, v := range headers {
		request.Header.Set(k, v)
	}

	escapedURL := strings.Replace(request.URL.EscapedPath(), "\n", "", -1)
	escapedURL = strings.Replace(escapedURL, "\r", "", -1)

	// Execute HTTP request
	// retry the request as specified by hi.httpRetry variable
	count := 0
	for {
		response, err = hi.client.Do(request)
		logs.Debugf("Trying Request %s, attempt %d/%d", escapedURL, count+1, hi.httpRetry)
		count++

		if err != nil && count >= hi.httpRetry {
			return err
		}
		if err == nil {
			break
		}
	}
	defer response.Body.Close()

	if body == nil && response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return &RequestError{response.StatusCode}
	}
	if body != nil && response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusAccepted {
		return &RequestError{response.StatusCode}
	}

	// Parse request
	switch v := ret.(type) {
	case *SpecialHeaders:
		(*v).Decrypted = (response.Header.Get("X-Decrypted") == "True")

		if (*v).Decrypted {
			if headerSize := response.Header.Get("X-Header-Size"); headerSize != "" {
				if (*v).HeaderSize, err = strconv.ParseInt(headerSize, 10, 0); err != nil {
					logs.Warningf("Could not convert header X-Header-Size to integer: %w", err)
					(*v).Decrypted = false
				}
			} else {
				logs.Warningf("Could not find header X-Header-Size in response")
				(*v).Decrypted = false
			}
		}

		if segSize := response.Header.Get("X-Segmented-Object-Size"); segSize != "" {
			if (*v).SegmentedObjectSize, err = strconv.ParseInt(segSize, 10, 0); err != nil {
				logs.Warningf("Could not convert header X-Segmented-Object-Size to integer: %w", err)
				(*v).SegmentedObjectSize = -1
			}
		} else {
			(*v).SegmentedObjectSize = -1
		}

		_, _ = io.Copy(io.Discard, response.Body)
	case []byte:
		if _, err = io.ReadFull(response.Body, v); err != nil {
			return fmt.Errorf("Copying response failed: %w", err)
		}
	case *[]byte:
		if *v, err = io.ReadAll(response.Body); err != nil {
			return fmt.Errorf("Copying response failed: %w", err)
		}
	default:
		if err = json.NewDecoder(response.Body).Decode(v); err != nil {
			return fmt.Errorf("Unable to decode response: %w", err)
		}
	}

	logs.Debugf("Request %s returned a response", escapedURL)
	return nil
}

var GetNthLevel = func(rep string, fsPath string, nodes ...string) ([]Metadata, error) {
	return hi.repositories[rep].getNthLevel(filepath.FromSlash(fsPath), nodes...)
}

// UpdateAttributes modifies attributes of node in 'fsPath'.
// 'nodes' contains the original names of each node in 'fsPath'
var UpdateAttributes = func(nodes []string, fsPath string, attr any) error {
	return hi.repositories[nodes[0]].updateAttributes(nodes[1:], filepath.FromSlash(fsPath), attr)
}

// DownloadData requests data between range [start, end) from an API.
var DownloadData = func(nodes []string, path string, start int64, end int64, maxEnd int64) ([]byte, error) {
	// chunk index of cache
	chunk := start / chunkSize
	// start coordinate of chunk
	chStart := chunk * chunkSize
	// end coordinate of chunk
	chEnd := (chunk + 1) * chunkSize

	// Final chunk may be shorter than others if file size restricts it
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
		err := hi.repositories[nodes[0]].downloadData(nodes[1:], buf, chStart, chEnd)
		if err != nil {
			return nil, fmt.Errorf("Retrieving data failed for %s: %w", path, err)
		}

		downloadCache.Set(cacheKey, buf, int64(len(buf)), time.Minute*60)
		logs.Debugf("File %s stored in cache, with coordinates [%d, %d)", path, chStart, chEnd)

		if endofst > int64(len(buf)) {
			endofst = int64(len(buf))
		}
		return buf[ofst:endofst], nil
	}

	ret := response.([]byte)
	if endofst > int64(len(ret)) {
		endofst = int64(len(ret))
	}
	logs.Debugf("Retrieved file %s from cache, with coordinates [%d, %d)", path, start, end)
	return ret[ofst:endofst], nil
}

var ClearCache = func() {
	downloadCache.Clear()
}
