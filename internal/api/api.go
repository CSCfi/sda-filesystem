package api

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	// Certificate is the TLS certificate to repositories
	Certificate string
	// RequestTimeout dictates the number of seconds to wait before timing out an http request
	RequestTimeout int
	// MetadataURL is url to metadata API
	MetadataURL string
	// DataURL is url to data API
	DataURL   string
	httpRetry int = 5
	token     string
)

// Container stores container data from metadata API
type Container struct {
	Bytes int64  `json:"bytes"`
	Count int64  `json:"count"`
	Name  string `json:"name"`
}

// DataObject stores object data from metadata API
type DataObject struct {
	Bytes       int64  `json:"bytes"`
	ContentType string `json:"content_type"`
	Hash        string `json:"hash"`
	Name        string `json:"name"`
	Subdir      string `json:"subdir"`
}

// CreateToken creates the authorization token based on username + password
func CreateToken() {
	fmt.Print("Enter username: ")
	scanner := bufio.NewScanner(os.Stdin)
	var username string
	if scanner.Scan() {
		username = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Print("Enter password: ")
	password, err := terminal.ReadPassword(syscall.Stdin)
	fmt.Println()
	if err != nil {
		log.Fatal(err)
	}

	token = base64.StdEncoding.EncodeToString([]byte(username + ":" + string(password)))
}

// makeRequest builds an authenticated HTTP client
// which sends HTTP requests and parses the responses
func makeRequest(url string, query map[string]string) ([]byte, error) {
	var response *http.Response

	// Handle certificate if one is set
	caCertPool := x509.NewCertPool()
	if len(Certificate) > 0 {
		caCert, err := ioutil.ReadFile(Certificate)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		caCertPool.AppendCertsFromPEM(caCert)
	} else {
		caCertPool = nil // So that default root certs are used
	}

	// Set up HTTP client
	timeout := time.Duration(RequestTimeout) * time.Second
	t := http.DefaultTransport.(*http.Transport).Clone()

	// Don't know if these are needed
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	t.TLSClientConfig = &tls.Config{
		RootCAs: caCertPool,
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: t,
	}

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
	request.Header.Set("Authorization", "Basic "+token)

	// Execute HTTP request
	// retry the request as specified by httpRetry variable
	for count := 0; count == 0 || (err != nil && count < httpRetry); {
		response, err = client.Do(request)
		log.Debugf("Trying Request %s, attempt %d/%d", request.URL, count+1, httpRetry)
		count++
	}
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 && response.StatusCode != 206 {
		return nil, fmt.Errorf("API responded with status %d", response.StatusCode)
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
func GetProjects() ([]string, error) {
	// Request projects
	response, err := makeRequest(strings.TrimRight(MetadataURL, "/")+"/projects", nil)
	if err != nil {
		return nil, fmt.Errorf("Retrieving projects failed: %w", err)
	}

	// Parse the JSON response into a slice
	var projects []string
	if err := json.Unmarshal(response, &projects); err != nil {
		return nil, fmt.Errorf("Unable to unmarshal response for retrieving projects: %w", err)
	}

	log.Info("Retrieved projects as per request")
	return projects, nil
}

// GetContainers gets conatiners inside the object
func GetContainers(project string) ([]Container, error) {
	// Request conteiners
	response, err := makeRequest(strings.TrimRight(MetadataURL, "/")+"/project/"+project+"/containers", nil)
	if err != nil {
		return nil, fmt.Errorf("Retrieving container from project %s failed: %w", project, err)
	}

	// Parse the JSON response into a slice
	var containers []Container
	if err := json.Unmarshal(response, &containers); err != nil {
		return nil, fmt.Errorf("Unable to unmarshal response for retrieving containers: %w", err)
	}

	log.Infof("Retrieved containers for project %s", project)
	return containers, nil
}

// GetObjects gets objects inside container
func GetObjects(project, container string) ([]DataObject, error) {
	// Request objects
	response, err := makeRequest(strings.TrimRight(MetadataURL, "/")+"/project/"+project+"/container/"+container+"/objects", nil)
	if err != nil {
		return nil, fmt.Errorf("Retrieving objects from container %s failed: %w", container, err)
	}

	// Parse the JSON response into a struct
	var objects []DataObject
	if err := json.Unmarshal(response, &objects); err != nil {
		return nil, fmt.Errorf("Unable to unmarshal response for retrieving objects: %w", err)
	}
	log.Infof("Retrieved objects for %s", project+"/"+container)
	return objects, nil
}

// DownloadData gets content of object from data API
func DownloadData(path string, start int64, end int64) ([]byte, error) {
	parts := strings.SplitN(strings.TrimLeft(path, "/"), "/", 3)
	// Query params
	query := map[string]string{
		"project":   parts[0],
		"container": parts[1],
		"object":    parts[2],
	}

	// Request data
	response, err := makeRequest(strings.TrimRight(DataURL, "/")+"/data", query)
	if err != nil {
		return nil, err
	}
	log.Infof("Downloaded object %s", path)
	//log.Infof("Downloaded object %s from coordinates %d-%d", path, start, end)
	return response, nil
}
