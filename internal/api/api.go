package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var hi = HTTPInfo{requestTimeout: 10, httpRetry: 5}

// HTTPInfo ...
type HTTPInfo struct {
	requestTimeout int
	httpRetry      int
	certPath       string
	metadataURL    string
	dataURL        string
	token          string
	client         *http.Client
}

// Container stores container data from metadata API
type Container struct {
	Bytes int64  `json:"bytes"`
	Name  string `json:"name"`
}

// DataObject stores object data from metadata API
type DataObject struct {
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

// GetEnvs looks up the necessary environment variables
func GetEnvs() error {
	var err error
	hi.certPath, err = getEnv("SD_CONNECT_CERTS", false)
	if err != nil {
		return err
	}
	hi.metadataURL, err = getEnv("SD_CONNECT_METADATA_API", true)
	if err != nil {
		return err
	}
	hi.dataURL, err = getEnv("SD_CONNECT_DATA_API", true)
	if err != nil {
		return err
	}
	return nil
}

func getEnv(name string, verifyURL bool) (string, error) {
	env, ok := os.LookupEnv(name)

	if !ok {
		return "", fmt.Errorf("Environment variable %s not set", name)
	}

	if verifyURL {
		// Verify that repository URL is valid
		u, err := url.ParseRequestURI(env)
		if err != nil {
			log.Error(err)
			return "", fmt.Errorf("Environment variable %s is an invalid URL", name)
		}
		if u.Scheme != "https" {
			return "", fmt.Errorf("Environment variable %s does not have scheme 'https'", name)
		}
	}

	return env, nil
}

// CreateToken creates the authorization token based on username + password
func CreateToken(username, password string) {
	hi.token = base64.StdEncoding.EncodeToString([]byte(username + ":" + string(password)))
}

// InitializeClient initializes a global http client
// The returned string is the message shown to the user in the gui when the accompanying error occurs
func InitializeClient() (string, error) {
	// Handle certificate if one is set
	caCertPool := x509.NewCertPool()
	if len(hi.certPath) > 0 {
		caCert, err := ioutil.ReadFile(hi.certPath)
		if err != nil {
			return "Reading certificate file failed", err
		}
		caCertPool.AppendCertsFromPEM(caCert)
	} else {
		caCertPool = nil // So that default root certs are used
	}

	// Set up HTTP client
	timeout := time.Duration(hi.requestTimeout) * time.Second
	tr := http.DefaultTransport.(*http.Transport).Clone()

	tr.MaxConnsPerHost = 100
	tr.MaxIdleConnsPerHost = 100
	//tr.MaxIdleConns

	tr.TLSClientConfig = &tls.Config{
		RootCAs: caCertPool,
	}
	hi.client = &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}

	// Temporarily disabling the keep-alive feature so that
	// connections do not hog goroutines
	tr.DisableKeepAlives = true
	_, err := hi.client.Head(hi.metadataURL)
	if err != nil {
		return "Cannot connect to metadata API. Certificate possibly missing or invalid, or url incorrect", err
	}

	_, err = hi.client.Head(hi.dataURL)
	if err != nil {
		return "Cannot connect to data API. Certificate possibly missing or invalid, or url incorrect", err
	}
	tr.DisableKeepAlives = false

	log.Debug("Initializing http client successful")
	return "", nil
}

// makeRequest builds an authenticated HTTP client
// which sends HTTP requests and parses the responses
func makeRequest(url string, query map[string]string, headers map[string]string) ([]byte, error) {
	var response *http.Response

	// Build HTTP request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Place query params if they are set
	q := request.URL.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	request.URL.RawQuery = q.Encode()

	// Place mandatory headers
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Basic "+hi.token)

	// Place additional headers if any are available
	for k, v := range headers {
		request.Header.Set(k, v)
	}

	// Execute HTTP request
	// retry the request as specified by httpRetry variable
	for count := 0; count == 0 || (err != nil && count < hi.httpRetry); {
		response, err = hi.client.Do(request)
		log.Debugf("Trying Request %s, attempt %d/%d", request.URL, count+1, hi.httpRetry)
		count++
	}
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 && response.StatusCode != 206 {
		return nil, &RequestError{response.StatusCode}
	}

	// Parse request
	r, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	log.Debugf("Request %s returned a response", request.URL)
	return r, nil
}

// GetProjects gets all projects user has access to
func GetProjects() ([]Container, error) {
	// Request projects
	response, err := makeRequest(
		strings.TrimSuffix(hi.metadataURL, "/")+
			"/projects", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("Retrieving projects failed: %w", err)
	}

	// Parse the JSON response into a slice
	var projects []Container
	if err := json.Unmarshal(response, &projects); err != nil {
		return nil, fmt.Errorf("Unable to unmarshal response for retrieving projects: %w", err)
	}

	log.Info("Retrieved projects as per request")
	return projects, nil
}

// GetContainers gets conatiners inside the object
func GetContainers(project string) ([]Container, error) {
	// Request conteiners
	response, err := makeRequest(
		strings.TrimSuffix(hi.metadataURL, "/")+
			"/project/"+
			url.PathEscape(project)+"/containers", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("Retrieving container from project %s failed: %w", project, err)
	}

	// Parse the JSON response into a slice
	var containers []Container
	if err := json.Unmarshal(response, &containers); err != nil {
		return nil, fmt.Errorf("Unable to unmarshal response for retrieving containers:\n%w", err)
	}

	log.Infof("Retrieved containers for project %s", project)
	return containers, nil
}

// GetObjects gets objects inside container
func GetObjects(project, container string) ([]DataObject, error) {
	// Request objects
	response, err := makeRequest(
		strings.TrimSuffix(hi.metadataURL, "/")+
			"/project/"+
			url.PathEscape(project)+"/container/"+
			url.PathEscape(container)+"/objects", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("Retrieving objects from container %s failed: %w", container, err)
	}

	// Parse the JSON response into a struct
	var objects []DataObject
	if err := json.Unmarshal(response, &objects); err != nil {
		return nil, fmt.Errorf("Unable to unmarshal response for retrieving objects:\n%w", err)
	}
	log.Infof("Retrieved objects for %s", project+"/"+container)
	return objects, nil
}

// DownloadData gets content of object from data API
func DownloadData(path string, start int64, end int64) ([]byte, error) {
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	// Query params
	query := map[string]string{
		"project":   parts[0],
		"container": parts[1],
		"object":    parts[2],
	}

	// Additional headers
	headers := map[string]string{"Range": "bytes=" + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end-1, 10)}

	// Request data
	response, err := makeRequest(strings.TrimSuffix(hi.dataURL, "/")+"/data", query, headers)
	if err != nil {
		return nil, err
	}
	log.Infof("Downloaded object %s from coordinates %d-%d", path, start, end-1)
	return response, nil
}
