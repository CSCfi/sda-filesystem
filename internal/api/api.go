package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
var possibleRepositories = make(map[string]fuseInfo)
var downloadCache *cache.Ristretto

// LoginMethod is an enum that main will use to know which login method to use
type LoginMethod int

const (
	Password LoginMethod = iota
	Token
)

// httpInfo contains all necessary variables used during HTTP requests
type httpInfo struct {
	requestTimeout int
	httpRetry      int
	certPath       string
	sdsToken       string
	userinfoURL    string
	client         *http.Client
	repositories   map[string]fuseInfo
}

// If you wish to add a new repository, it must implement the following functions
type fuseInfo interface {
	getEnvs() error
	getLoginMethod() LoginMethod
	validateLogin(...string) error
	levelCount() int
	getNthLevel(string, ...string) ([]Metadata, error)
	updateAttributes([]string, string, interface{}) error
	downloadData([]string, interface{}, int64, int64) error
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
var GetEnabledRepositories = func() []string {
	var names []string
	for key := range hi.repositories {
		names = append(names, key)
	}
	return names
}

// AddRepository adds a repository to hi.repositories
var AddRepository = func(r string) {
	hi.repositories[r] = possibleRepositories[r]
}

// RemoveRepository removes a repository from hi.repositories
var RemoveRepository = func(r string) {
	delete(hi.repositories, r)
}

// SetRequestTimeout redefines hi.requestTimeout
var SetRequestTimeout = func(timeout int) {
	hi.requestTimeout = timeout
}

// GetEnvs gets the environment variables for repository 'r'
var GetEnvs = func(r string) error {
	return possibleRepositories[r].getEnvs()
}

// getEnv looks up environment variable given in 'name'
var getEnv = func(name string, verifyURL bool) (string, error) {
	env, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("Environment variable %s not set", name)
	}
	if verifyURL {
		return env, validURL(env)
	}
	return env, nil
}

var validURL = func(env string) error {
	u, err := url.ParseRequestURI(env)
	if err != nil {
		return fmt.Errorf("Environment variable %s is an invalid URL: %w", env, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("Environment variable %s does not have scheme 'https'", env)
	}
	return nil
}

// GetCommonEnvs gets the environmental varibles that are needed for all repositories
// This may need to be changed if new repositories are added
func GetCommonEnvs() (err error) {
	if hi.certPath, err = getEnv("FS_CERTS", false); err != nil {
		return err
	}
	if hi.sdsToken, err = getEnv("SDS_ACCESS_TOKEN", false); err != nil {
		return err
	}
	if hi.userinfoURL, err = getEnv("USERINFO_ENDPOINT", true); err != nil {
		return err
	}
	return nil
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
		caCert, err := ioutil.ReadFile(hi.certPath)
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

	timeout := time.Duration(hi.requestTimeout) * time.Second

	hi.client = &http.Client{
		Timeout:   timeout,
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

var IsProjectManager = func() (bool, error) {
	errStr := "Could not determine to which project this Desktop belongs"

	file, err := os.ReadFile("/etc/pam_userinfo/config.json")
	if err != nil {
		return false, fmt.Errorf("%s: %w", errStr, err)
	}

	var data map[string]interface{}
	if err = json.Unmarshal(file, &data); err != nil {
		return false, fmt.Errorf("%s: %w", errStr, err)
	}

	pr, ok := data["login_aud"]
	if !ok {
		return false, fmt.Errorf("%s: %w", errStr, errors.New("Config file did not contain key 'login_aud'"))
	}

	if err := makeRequest(hi.userinfoURL, nil, nil, &data); err != nil {
		var re *RequestError
		if errors.As(err, &re) && re.StatusCode == 400 {
			return false, fmt.Errorf("Invalid token")
		} else {
			return false, err
		}
	}

	if projectPI, ok := data["projectPI"]; !ok {
		return false, fmt.Errorf("Response body did not contain key 'projectPI'")
	} else {
		projects := strings.Split(fmt.Sprintf("%v", projectPI), " ")
		for i := range projects {
			if projects[i] == fmt.Sprintf("%v", pr) {
				return true, nil
			}
		}
		return false, nil
	}
}

// GetLoginMethod returns the login method of repository 'rep'
var GetLoginMethod = func(rep string) LoginMethod {
	return possibleRepositories[rep].getLoginMethod()
}

// ValidateLogin checks if user is able to log in with given input to repository 'rep'
// The returned error contains ONLY the text that will be shown in an UI error popup, UNLESS the error contains status 401
var ValidateLogin = func(rep string, auth ...string) error {
	return hi.repositories[rep].validateLogin(auth...)
}

// LevelCount returns the amount of levels repository 'rep' has
var LevelCount = func(rep string) int {
	return hi.repositories[rep].levelCount()
}

// makeRequest sends HTTP requests and parses the responses
var makeRequest = func(url string, query, headers map[string]string, ret interface{}) error {
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
	request.Header.Set("Authorization", "Bearer "+hi.sdsToken)

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
var UpdateAttributes = func(nodes []string, fsPath string, attr interface{}) error {
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

var ExportFile = func(folder, file string) {

}
