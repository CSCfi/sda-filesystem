package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sd-connect-fuse/internal/cache"
	"sd-connect-fuse/internal/logs"
	"strconv"
	"strings"
	"time"
)

const chunkSize = 1 << 25

var hi = HTTPInfo{requestTimeout: 20, httpRetry: 3, sTokens: make(map[string]SToken), loggedIn: false}
var downloadCache *cache.Ristretto = nil

var makeRequest func(url string, token string, query map[string]string, headers map[string]string, ret interface{}) error

// HTTPInfo contains all necessary variables used during HTTP requests
type HTTPInfo struct {
	requestTimeout int
	httpRetry      int
	certPath       string
	metadataURL    string
	dataURL        string
	token          string
	uToken         string
	sTokens        map[string]SToken
	client         *http.Client
	loggedIn       bool
}

// Metadata stores data from either a call to a project, container or object
type Metadata struct {
	Bytes int64  `json:"bytes"`
	Name  string `json:"name"`
}

// UToken is the unscoped token
type UToken struct {
	Token string `json:"token"`
}

// SToken is a scoped token
type SToken struct {
	Token     string `json:"token"`
	ProjectID string `json:"projectID"`
}

type specialHeaders struct {
	decrypted           bool
	segmentedObjectSize int64
}

// RequestError is used to obtain the status code from the HTTP request
type RequestError struct {
	StatusCode int
}

func (re *RequestError) Error() string {
	return fmt.Sprintf("API responded with status %d", re.StatusCode)
}

func init() {
	makeRequest = makeRequestPlaceholder
}

// GetEnvs looks up the necessary environment variables
func GetEnvs() error {
	var err error
	hi.certPath, err = getEnv("FS_SD_CONNECT_CERTS", false)
	if err != nil {
		return err
	}
	hi.metadataURL, err = getEnv("FS_SD_CONNECT_METADATA_API", true)
	if err != nil {
		return err
	}
	hi.dataURL, err = getEnv("FS_SD_CONNECT_DATA_API", true)
	if err != nil {
		return err
	}
	return nil
}

// getEnv looks up environment variable 'name'
func getEnv(name string, verifyURL bool) (string, error) {
	env, ok := os.LookupEnv(name)

	if !ok {
		return "", fmt.Errorf("Environment variable %s not set", name)
	}

	if verifyURL {
		// Verify that repository URL is valid
		u, err := url.ParseRequestURI(env)
		if err != nil {
			return "", fmt.Errorf("Environment variable %s is an invalid URL: %w", name, err)
		}
		if u.Scheme != "https" {
			return "", fmt.Errorf("Environment variable %s does not have scheme 'https'", name)
		}
	}

	return env, nil
}

// SetRequestTimeout redefines hi.requestTimeout
func SetRequestTimeout(timeout int) {
	hi.requestTimeout = timeout
}

// SetLoggedIn sets hi.loggedIn as true
func SetLoggedIn() {
	hi.loggedIn = true
}

// CreateToken creates the authorization token based on username + password
func CreateToken(username, password string) {
	hi.token = base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
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
	// Handle certificate if one is set
	caCertPool := x509.NewCertPool()
	if len(hi.certPath) > 0 {
		caCert, err := ioutil.ReadFile(hi.certPath)
		if err != nil {
			return fmt.Errorf("Reading certificate file failed: %w", err)
		}
		caCertPool.AppendCertsFromPEM(caCert)
	} else {
		caCertPool = nil // So that default root certs are used
	}

	// Set up HTTP client
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxConnsPerHost = 100
	tr.MaxIdleConnsPerHost = 100
	tr.TLSClientConfig = &tls.Config{
		RootCAs: caCertPool,
	}
	tr.ForceAttemptHTTP2 = true
	tr.DisableKeepAlives = false

	timeout := time.Duration(hi.requestTimeout) * time.Second

	hi.client = &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}

	_, err := hi.client.Head(hi.metadataURL)
	if err != nil {
		return fmt.Errorf("Cannot connect to metadata API: %w", err)
	}

	_, err = hi.client.Head(hi.dataURL)
	if err != nil {
		return fmt.Errorf("Cannot connect to data API: %w", err)
	}

	logs.Debug("Initializing HTTP client successful")
	return nil
}

// makeRequest sends HTTP requests and parses the responses
func makeRequestPlaceholder(url string, token string, query map[string]string, headers map[string]string, ret interface{}) error {
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
		request.Header.Set("Authorization", "Basic "+hi.token)
	}

	// Place additional headers if any are available
	for k, v := range headers {
		request.Header.Set(k, v)
	}

	// Execute HTTP request
	// retry the request as specified by hi.httpRetry variable
	for count := 0; count == 0 || (err != nil && count < hi.httpRetry); {
		response, err = hi.client.Do(request)
		logs.Debugf("Trying Request %s, attempt %d/%d", request.URL, count+1, hi.httpRetry)
		count++
	}
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if hi.loggedIn && response.StatusCode == 401 {
		logs.Info("Tokens no longer valid. Fetching them again")
		FetchTokens()
		hi.loggedIn = false // To prevent unlikely infinite loop
		err = makeRequest(url, token, query, headers, ret)
		hi.loggedIn = true
		return err
	}

	if response.StatusCode != 200 && response.StatusCode != 206 {
		return &RequestError{response.StatusCode}
	}

	// Parse request
	switch v := ret.(type) {
	case *specialHeaders:
		(*v).decrypted = (response.Header.Get("X-Decrypted") == "True")
		if segSize := response.Header.Get("X-Segmented-Object-Size"); segSize != "" {
			if (*v).segmentedObjectSize, err = strconv.ParseInt(segSize, 10, 0); err != nil {
				logs.Warningf("Could not convert header X-Segmented-Object-Size to integer: %s", err.Error())
				(*v).segmentedObjectSize = -1
			}
		} else {
			(*v).segmentedObjectSize = -1
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

// FetchTokens fetches the unscoped token and the scoped tokens
func FetchTokens() {
	err := GetUToken()
	if err != nil {
		logs.Warningf("HTTP requests may be slower: %s", err.Error())
		hi.uToken = ""
		return
	}

	projects, err := GetProjects()
	if err != nil {
		logs.Warningf("HTTP requests may be slower: %s", err.Error())
		hi.sTokens = map[string]SToken{}
	}

	for i := range projects {
		err = GetSToken(projects[i].Name)
		if err != nil {
			logs.Warning(err)
			delete(hi.sTokens, projects[i].Name)
		}
	}

	logs.Info("Fetched tokens")
}

// GetUToken gets the unscoped token
func GetUToken() error {
	// Request token
	uToken := UToken{}
	err := makeRequest(strings.TrimSuffix(hi.metadataURL, "/")+"/token", "", nil, nil, &uToken)
	if err != nil {
		return fmt.Errorf("Retrieving unscoped token failed: %w", err)
	}

	hi.uToken = uToken.Token
	logs.Debug("Retrieved unscoped token")
	return nil
}

// GetSToken gets the scoped tokens for a project
func GetSToken(project string) error {
	// Query params
	query := map[string]string{"project": project}

	// Request token
	sToken := SToken{}
	err := makeRequest(strings.TrimSuffix(hi.metadataURL, "/")+"/token", "", query, nil, &sToken)
	if err != nil {
		return fmt.Errorf("Retrieving scoped token for %s failed: %w", project, err)
	}

	hi.sTokens[project] = sToken
	logs.Debug("Retrieved scoped token for ", project)
	return nil
}

// GetProjects gets all projects user has access to
func GetProjects() ([]Metadata, error) {
	// Request projects
	var projects []Metadata
	err := makeRequest(strings.TrimSuffix(hi.metadataURL, "/")+"/projects", hi.uToken, nil, nil, &projects)
	if err != nil {
		return nil, fmt.Errorf("HTTP request for projects failed: %w", err)
	}

	logs.Debugf("Retrieved %d projects", len(projects))
	return projects, nil
}

// GetContainers gets conatainers inside project
func GetContainers(project string) ([]Metadata, error) {
	// Additional headers
	headers := map[string]string{"X-Project-ID": hi.sTokens[project].ProjectID}

	// Request containers
	var containers []Metadata
	err := makeRequest(
		strings.TrimSuffix(hi.metadataURL, "/")+
			"/project/"+
			url.PathEscape(project)+"/containers", hi.sTokens[project].Token, nil, headers, &containers)
	if err != nil {
		return nil, fmt.Errorf("Retrieving containers for %s failed: %w", project, err)
	}

	logs.Infof("Retrieved containers for %s", project)
	return containers, nil
}

// GetObjects gets objects inside container
func GetObjects(project, container string) ([]Metadata, error) {
	// Additional headers
	headers := map[string]string{"X-Project-ID": hi.sTokens[project].ProjectID}

	// Request objects
	var objects []Metadata
	err := makeRequest(
		strings.TrimSuffix(hi.metadataURL, "/")+
			"/project/"+
			url.PathEscape(project)+"/container/"+
			url.PathEscape(container)+"/objects", hi.sTokens[project].Token, nil, headers, &objects)
	if err != nil {
		return nil, fmt.Errorf("Retrieving objects for %s failed: %w", container, err)
	}

	logs.Infof("Retrieved objects for %s/%s", project, container)
	return objects, nil
}

// GetSpecialHeaders returns information on headers that can only be retirived from data api
func GetSpecialHeaders(path string) (bool, int64, error) {
	parts := strings.SplitN(path, "/", 3)
	project := parts[0]

	// Query params
	query := map[string]string{
		"project":   parts[0],
		"container": parts[1],
		"object":    parts[2],
	}

	// Additional headers
	headers := map[string]string{"Range": "bytes=0-1", "X-Project-ID": hi.sTokens[project].ProjectID}

	var ret specialHeaders
	err := makeRequest(strings.TrimSuffix(hi.dataURL, "/")+"/data", hi.sTokens[project].Token, query, headers, &ret)
	if err != nil {
		return false, -1, fmt.Errorf("Retrieving data failed for %s: %w", path, err)
	}

	return ret.decrypted, ret.segmentedObjectSize, nil
}

// DownloadData gets content of object from data API
func DownloadData(path string, start int64, end int64, maxEnd int64) ([]byte, error) {
	parts := strings.SplitN(path, "/", 3)
	project := parts[0]

	// Query params
	query := map[string]string{
		"project":   parts[0],
		"container": parts[1],
		"object":    parts[2],
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

	// we make the cache key based on object path and requested bytes
	cacheKey := parts[0] + "_" + parts[1] + "_" + parts[2] + "_" + strconv.FormatInt(chStart, 10)
	response, found := downloadCache.Get(cacheKey)

	if !found {
		// Additional headers
		headers := map[string]string{"Range": "bytes=" + strconv.FormatInt(chStart, 10) + "-" + strconv.FormatInt(chEnd-1, 10),
			"X-Project-ID": hi.sTokens[project].ProjectID}

		// Request data
		buf := make([]byte, chEnd-chStart)
		err := makeRequest(strings.TrimSuffix(hi.dataURL, "/")+"/data", hi.sTokens[project].Token, query, headers, buf)
		if err != nil {
			return nil, fmt.Errorf("Retrieving data failed for %s: %w", path, err)
		}

		downloadCache.Set(cacheKey, buf, time.Minute*60)
		logs.Debugf("Object %s stored in cache, with coordinates %d-%d", path, chStart, chEnd-1)

		if endofst > int64(len(buf)) {
			endofst = int64(len(buf))
		}
		return buf[ofst:endofst], nil
	}

	ret := response.([]byte)
	if endofst > int64(len(ret)) {
		endofst = int64(len(ret))
	}
	logs.Debugf("Retrieved object %s from cache, with coordinates %d-%d", path, start, end-1)
	return ret[ofst:endofst], nil
}
