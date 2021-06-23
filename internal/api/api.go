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
	// Certificate ...
	Certificate string
	// RequestTimeout ...
	RequestTimeout int
	// MetadataURL ...
	MetadataURL string
	// DataURL ...
	DataURL string
	// Project ...
	Project string
	// hhtpRetry ...
	httpRetry int = 5
	// token ...
	token string
)

// Container ...
type Container struct {
	Bytes int64  `json:"bytes"`
	Count int64  `json:"count"`
	Name  string `json:"name"`
}

// DataObject ...
type DataObject struct {
	Bytes       int64  `json:"bytes"`
	ContentType string `json:"content_type"`
	Hash        string `json:"hash"`
	Name        string `json:"name"`
	Subdir      string `json:"subdir"`
}

// CreateToken ...
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
		caCertPool = nil // So that default root ca are used
	}

	// Set up HTTP client
	timeout := time.Duration(RequestTimeout) * time.Second
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			ForceAttemptHTTP2: true,
		},
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
		log.Debug(err)
		count++
	}
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 && response.StatusCode != 206 {
		return nil, fmt.Errorf("Metadata API responded with status %d", response.StatusCode)
	}

	// Parse request
	r, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	log.Debugf("Request %s returned a response", request.URL)
	return r, nil
}

// GetProjects ...
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

// GetContainers ...
func GetContainers(project string) ([]Container, error) {
	// Request datasets
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

// GetObjects ...
func GetObjects(project, container string) ([]DataObject, error) {
	// Request object details
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

/*
// DownloadData ...
func DownloadData(fileID string, start int64, end int64) ([]byte, error) {
}
*/
