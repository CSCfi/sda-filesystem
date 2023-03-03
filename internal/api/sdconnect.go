package api

import (
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
	getProjects() ([]Metadata, error)
	getToken(string) (sToken, error)
	getSTokens([]Metadata) map[string]sToken
}

type connecter struct {
	url       *string
	token     *string
	overriden *bool
}

type sdConnectInfo struct {
	connectable
	url       string
	token     string
	sTokens   map[string]sToken
	projects  []Metadata
	overriden bool
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
	cr.url = &sd.url
	cr.token = &sd.token
	cr.overriden = &sd.overriden
	allRepositories[SDConnect] = sd
}

//
// Functions for connecter
//

func (c *connecter) getProjects() ([]Metadata, error) {
	var projects []Metadata
	headers := map[string]string{"X-Authorization": "Basic " + *c.token}
	err := MakeRequest(*c.url+"/projects", nil, headers, nil, &projects)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s projects: %w", SDConnect, err)
	}

	return projects, nil
}

func (c *connecter) getToken(name string) (sToken, error) {
	query := map[string]string{"project": name}
	headers := map[string]string{"X-Authorization": "Basic " + *c.token}

	if *c.overriden {
		headers["X-Project-Name"] = name
	}

	// Request token
	ret := sToken{}
	err := MakeRequest(*c.url+"/token", query, headers, nil, &ret)

	return ret, err
}

// getSTokens fetches the scoped tokens
// Using only one goroutine since SD Desktop VM should only allow for one project
func (c *connecter) getSTokens(projects []Metadata) map[string]sToken {
	newSTokens := make(map[string]sToken)

	for i := range projects {
		projectToken, err := c.getToken(projects[i].Name)
		if err != nil {
			logs.Warningf("Failed to retrieve %s scoped token for %s: %w", SDConnect, projects[i].Name, err)

			continue
		}

		logs.Debugf("Retrieved %s scoped token for %s", SDConnect, projects[i].Name)
		newSTokens[projects[i].Name] = sToken{Token: projectToken.Token, ProjectID: projectToken.ProjectID}
	}

	logs.Infof("Fetching %s tokens finished", SDConnect)

	return newSTokens
}

//
// Functions for sdConnectInfo
//

func (c *sdConnectInfo) getEnvs() error {
	api, err := GetEnv("FS_SD_CONNECT_API", true)
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
	if len(auth) < 2 {
		return fmt.Errorf("validateLogin() called with too few parameters")
	}

	var err error
	c.projects = nil
	c.token = auth[0]
	projectReplacement := auth[1]

	if projectReplacement != "" {
		c.overriden = true
		c.projects = []Metadata{{-1, projectReplacement}}

		var token sToken
		token, err = c.getToken(projectReplacement)
		if err == nil {
			c.sTokens = map[string]sToken{projectReplacement: token}

			return nil
		}
	} else if c.projects, err = c.getProjects(); err == nil {
		if len(c.projects) == 0 {
			return fmt.Errorf("No projects found for %s", SDConnect)
		}
		logs.Infof("Retrieved %d %s project(s)", len(c.projects), SDConnect)
		c.sTokens = c.getSTokens(c.projects)

		return nil
	}

	var re *RequestError
	if errors.As(err, &re) && re.StatusCode == 401 {
		return fmt.Errorf("%s login failed: %w", SDConnect, err)
	}
	if errors.As(err, &re) && re.StatusCode == 500 {
		return fmt.Errorf("%s is not available, please contact CSC servicedesk: %w", SDConnect, err)
	}

	return err
}

func (c *sdConnectInfo) levelCount() int {
	return 3
}

func (c *sdConnectInfo) getNthLevel(fsPath string, nodes ...string) ([]Metadata, error) {
	if len(nodes) == 0 {
		return c.projects, nil
	}

	headers := map[string]string{}
	path := c.url + "/project/" + url.PathEscape(nodes[0])
	switch len(nodes) {
	case 1:
		path += "/containers"
	case 2:
		path += "/container/" + url.PathEscape(nodes[1]) + "/objects"
	default:
		return nil, nil
	}

	var meta []Metadata
	err := c.makeRequest(path, nodes[0], nil, headers, &meta)
	if c.tokenExpired(err) {
		err = c.makeRequest(path, nodes[0], nil, headers, &meta)
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
		c.sTokens = c.getSTokens(c.projects)

		return true
	}

	return false
}

func (c *sdConnectInfo) updateAttributes(nodes []string, path string, attr any) error {
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

func (c *sdConnectInfo) makeRequest(path, project string, query, headers map[string]string, ret any) error {
	token := c.sTokens[project]
	headers["X-Project-ID"] = token.ProjectID
	headers["X-Authorization"] = "Bearer " + token.Token

	if c.overriden {
		headers["X-Project-Name"] = project
	}

	return MakeRequest(path, query, headers, nil, ret)
}

func (c *sdConnectInfo) downloadData(nodes []string, buffer any, start, end int64) error {
	// Query params
	query := map[string]string{
		"project":   nodes[0],
		"container": nodes[1],
		"object":    strings.Join(nodes[2:], "/"),
	}

	// Additional headers
	headers := map[string]string{"Range": "bytes=" + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end-1, 10)}

	path := c.url + "/data"

	// Request data
	err := c.makeRequest(path, nodes[0], query, headers, buffer)
	if c.tokenExpired(err) {
		return c.makeRequest(path, nodes[0], query, headers, buffer)
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
		remainder += macSize
	}

	// Add the previous info back together
	decryptedSize := blocks*blockSize + remainder

	return decryptedSize
}
