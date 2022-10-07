package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"sda-filesystem/internal/logs"
)

// This file contains structs and functions that are strictly for SD Connect

const SDConnect string = "SD Connect"

// This exists for unit test mocking
type connectable interface {
	getProjects(string, string) ([]Metadata, error)
	getSTokens([]Metadata, string, string) map[string]sToken
}

type connecter struct {
}

type sdConnectInfo struct {
	connectable
	url      string
	token    string
	sTokens  map[string]sToken
	projects []Metadata
}

// sToken is a scoped token for a certain project
type sToken struct {
	Token     string `json:"token"`
	ProjectID string `json:"projectID"`
}

// SpecialHeaders are important http response headers from sd-connect-api that need to be
// fetched before a file can be opened
type SpecialHeaders struct {
	Decrypted           bool
	SegmentedObjectSize int64
	HeaderSize          int64
}

func init() {
	cr := &connecter{}
	sd := &sdConnectInfo{connectable: cr, sTokens: make(map[string]sToken)}
	allRepositories[SDConnect] = sd
}

//
// Functions for connecter
//

func (c *connecter) getProjects(url, token string) ([]Metadata, error) {
	var projects []Metadata
	headers := map[string]string{"X-Authorization": "Basic " + token}
	err := makeRequest(url+"/projects", nil, headers, &projects)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s projects: %w", SDConnect, err)
	}
	return projects, nil
}

// getSTokens fetches the scoped tokens
// Using only one goroutine since SD Desktop VM should only allow for one project
func (c *connecter) getSTokens(projects []Metadata, url, token string) map[string]sToken {
	newSTokens := make(map[string]sToken)

	for i := range projects {
		query := map[string]string{"project": projects[i].Name}
		headers := map[string]string{"X-Authorization": "Basic " + token}

		// Request token
		token := sToken{}
		if err := makeRequest(url+"/token", query, headers, &token); err != nil {
			logs.Warningf("Failed to retrieve %s scoped token for %s: %w", SDConnect, projects[i].Name, err)
			continue
		}

		logs.Debugf("Retrieved %s scoped token for %s", SDConnect, projects[i].Name)
		newSTokens[projects[i].Name] = sToken{Token: token.Token, ProjectID: token.ProjectID}
	}

	logs.Infof("Fetching %s tokens finished", SDConnect)
	return newSTokens
}

//
// Functions for sdConnectInfo
//

func (c *sdConnectInfo) getEnvs() error {
	api, err := getEnv("FS_SD_CONNECT_API", true)
	if err != nil {
		return err
	}
	c.url = strings.TrimRight(api, "/")
	if err := testURL(c.url); err != nil {
		return fmt.Errorf("Cannot connect to %s API: %w", SDConnect, err)
	}
	return nil
}

func (c *sdConnectInfo) validateLogin(auth ...string) error {
	if len(auth) == 2 {
		c.token = base64.StdEncoding.EncodeToString([]byte(auth[0] + ":" + auth[1]))
	}

	var err error
	c.projects = nil
	if c.projects, err = c.getProjects(c.url, c.token); err == nil {
		if len(c.projects) == 0 {
			return fmt.Errorf("No projects found for %s", SDConnect)
		}
		logs.Infof("Retrieved %d %s project(s)", len(c.projects), SDConnect)
		c.sTokens = c.getSTokens(c.projects, c.url, c.token)
		return nil
	}

	var re *RequestError
	if errors.As(err, &re) && re.StatusCode == 401 {
		return fmt.Errorf("%s login failed: %w", SDConnect, err)
	}
	if errors.As(err, &re) && re.StatusCode == 500 {
		return fmt.Errorf("%s is not available, please contact CSC servicedesk: %w", SDConnect, err)
	}
	return fmt.Errorf("Error occurred for %s: %w", SDConnect, err)
}

func (c *sdConnectInfo) levelCount() int {
	return 3
}

func (c *sdConnectInfo) getNthLevel(fsPath string, nodes ...string) ([]Metadata, error) {
	if len(nodes) == 0 {
		return c.projects, nil
	}

	token := c.sTokens[nodes[0]]
	headers := map[string]string{"X-Project-ID": token.ProjectID, "X-Authorization": "Bearer " + token.Token}
	path := c.url + "/project/" + url.PathEscape(nodes[0])
	if len(nodes) == 1 {
		path += "/containers"
	} else if len(nodes) == 2 {
		path += "/container/" + url.PathEscape(nodes[1]) + "/objects"
	} else {
		return nil, nil
	}

	var meta []Metadata
	err := makeRequest(path, nil, headers, &meta)
	if c.tokenExpired(err) {
		token = c.sTokens[nodes[0]]
		headers["X-Project-ID"] = token.ProjectID
		headers["X-Authorization"] = "Bearer " + token.Token
		err = makeRequest(path, nil, headers, &meta)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve metadata for %s: %w", fsPath, err)
	}

	logs.Infof("Retrieved metadata for %s", fsPath)
	return meta, nil
}

func (c *sdConnectInfo) tokenExpired(err error) bool {
	var re *RequestError
	if errors.As(err, &re) && re.StatusCode == 401 {
		logs.Infof("%s tokens no longer valid. Fetching them again", SDConnect)
		c.sTokens = c.getSTokens(c.projects, c.url, c.token)
		return true
	}
	return false
}

func (c *sdConnectInfo) updateAttributes(nodes []string, path string, attr interface{}) error {
	if len(nodes) < 3 {
		return fmt.Errorf("Cannot update attributes for path %s", path)
	}

	size, ok := attr.(*int64)
	if !ok {
		return fmt.Errorf("%s updateAttributes() was called with incorrect attribute. Expected type *int64, received %v",
			SDConnect, reflect.TypeOf(attr))
	}

	var headers SpecialHeaders
	if err := c.downloadData(nodes, &headers, 0, 2); err != nil {
		return err
	}
	if headers.SegmentedObjectSize != -1 {
		logs.Infof("Object %s is a segmented object with size %d", path, headers.SegmentedObjectSize)
		*size = headers.SegmentedObjectSize
	}
	if headers.Decrypted {
		dSize := calculateDecryptedSize(*size, headers.HeaderSize)
		if dSize != -1 {
			logs.Debugf("Object %s is automatically decrypted", path)
			*size = dSize
		} else {
			logs.Warningf("API returned header 'X-Decrypted' even though size of object %s is too small", path)
		}
	}
	return nil
}

func (c *sdConnectInfo) downloadData(nodes []string, buffer interface{}, start, end int64) error {
	// Query params
	query := map[string]string{
		"project":   nodes[0],
		"container": nodes[1],
		"object":    strings.Join(nodes[2:], "/"),
	}

	token := c.sTokens[nodes[0]]

	// Additional headers
	headers := map[string]string{"Range": "bytes=" + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end-1, 10),
		"X-Project-ID": token.ProjectID, "X-Authorization": "Bearer " + token.Token}

	path := c.url + "/data"

	// Request data
	err := makeRequest(path, query, headers, buffer)
	if c.tokenExpired(err) {
		token = c.sTokens[nodes[0]]
		headers["X-Project-ID"] = token.ProjectID
		headers["X-Authorization"] = "Bearer " + token.Token
		return makeRequest(path, query, headers, buffer)
	}
	return err
}

// calculateDecryptedSize calculates the decrypted size of an encrypted file size
var calculateDecryptedSize = func(fileSize, headerSize int64) int64 {
	// Crypt4GH settings
	var blockSize int64 = 65536
	var macSize int64 = 28
	cipherBlockSize := blockSize + macSize

	// Crypt4GH files have a minimum possible size of 152 bytes
	if fileSize < headerSize+macSize {
		return -1
	}

	// Calculate body size without header
	bodySize := fileSize - headerSize

	// Calculate number of cipher blocks in body
	// number of complete 64kiB datablocks
	blocks := int64(math.Floor(float64(bodySize) / float64(cipherBlockSize)))
	// the last block can be smaller than 64kiB
	remainder := bodySize%cipherBlockSize - macSize
	if remainder < 0 {
		remainder = remainder + macSize
	}

	// Add the previous info back together
	decryptedSize := blocks*blockSize + remainder

	return decryptedSize
}
