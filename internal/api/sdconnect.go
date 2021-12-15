package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"sda-filesystem/internal/logs"
	"strconv"
	"strings"
	"sync"
	"time"
)

// This file contains structs and functions that are strictly for SD-Connect

const SDConnect string = "SD-Connect"

// This exists for unit test mocking
type tokenable interface {
	getUToken(string) (string, error)
	getSToken(string, string) (sToken, error)
}

type tokenator struct {
}

// This exists for unit test mocking
type connectable interface {
	tokenable
	getProjects(string) ([]Metadata, error)
	fetchTokens(bool, []Metadata) (string, sync.Map)
}

type connecter struct {
	tokenable
	url *string
}

type sdConnectInfo struct {
	connectable
	dataURL     string
	metadataURL string
	certPath    string
	token       string
	uToken      string
	sTokens     sync.Map
	projects    []Metadata
}

// uToken is the unscoped token
type uToken struct {
	Token string `json:"token"`
}

// sToken is a scoped token
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
	cr := &connecter{tokenable: tokenator{}}
	sd := &sdConnectInfo{connectable: cr}
	cr.url = &sd.metadataURL
	possibleRepositories[SDConnect] = sd
}

//
// Functions for tokenator
//

// getUToken gets the unscoped token
func (t tokenator) getUToken(url string) (string, error) {
	// Request token
	token := uToken{}
	err := makeRequest(url+"/token", "", SDConnect, nil, nil, &token)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve %s unscoped token: %w", SDConnect, err)
	}

	logs.Debug("Retrieved unscoped token for ", SDConnect)
	return token.Token, nil
}

// getSToken gets the scoped tokens for a project
func (t tokenator) getSToken(url, project string) (sToken, error) {
	// Query params
	query := map[string]string{"project": project}

	// Request token
	token := sToken{}
	err := makeRequest(url+"/token", "", SDConnect, query, nil, &token)
	if err != nil {
		return sToken{}, fmt.Errorf("Failed to retrieve %s scoped token for %q: %w", SDConnect, project, err)
	}

	logs.Debugf("Retrieved %s scoped token for %q", SDConnect, project)
	return token, nil
}

//
// Functions for connecter
//

func (c *connecter) getProjects(token string) ([]Metadata, error) {
	var projects []Metadata
	err := makeRequest(*c.url+"/projects", token, SDConnect, nil, nil, &projects)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s projects: %w", SDConnect, err)
	}

	logs.Infof("Retrieved %d %s project(s)", len(projects), SDConnect)
	return projects, nil
}

// fetchTokens fetches the unscoped token and the scoped tokens
func (c *connecter) fetchTokens(skipUnscoped bool, projects []Metadata) (newUToken string, newSTokens sync.Map) {
	var err error
	if !skipUnscoped {
		newUToken, err = c.getUToken(*c.url)
		if err != nil {
			logs.Warningf("HTTP requests may be slower for %s: %w", SDConnect, err)
			return
		}
	}

	for i := range projects {
		go func(project Metadata) {
			token, err := c.getSToken(*c.url, project.Name)
			if err != nil {
				logs.Warningf("HTTP requests may be slower for %s project %q: %w", SDConnect, project.Name, err)
			} else {
				newSTokens.Store(project.Name, token)
			}
		}(projects[i])
	}

	time.Sleep(10 * time.Millisecond)
	logs.Infof("Fetched %s tokens", SDConnect)
	return
}

//
// Functions for sdConnectInfo
//

func (c *sdConnectInfo) getEnvs() error {
	api, err := getEnv("FS_SD_CONNECT_METADATA_API", true)
	if err != nil {
		return err
	}
	c.metadataURL = strings.TrimRight(api, "/")

	api, err = getEnv("FS_SD_CONNECT_DATA_API", true)
	if err != nil {
		return err
	}
	c.dataURL = strings.TrimRight(api, "/")

	c.certPath, err = getEnv("FS_SD_CONNECT_CERTS", false)
	return err
}

func (c *sdConnectInfo) getCertificatePath() string {
	return c.certPath
}

func (c *sdConnectInfo) testURLs() error {
	if err := testURL(c.metadataURL); err != nil {
		return fmt.Errorf("Cannot connect to %s metadata API: %w", SDConnect, err)
	}
	if err := testURL(c.dataURL); err != nil {
		return fmt.Errorf("Cannot connect to %s data API: %w", SDConnect, err)
	}
	return nil
}

func (c *sdConnectInfo) getLoginMethod() LoginMethod {
	return Password
}

func (c *sdConnectInfo) validateLogin(auth ...string) (err error) {
	if len(auth) == 2 {
		c.token = base64.StdEncoding.EncodeToString([]byte(auth[0] + ":" + auth[1]))
	}

	c.projects = nil
	if c.uToken, err = c.getUToken(c.metadataURL); err != nil {
		return
	}
	if c.projects, err = c.getProjects(c.uToken); err != nil {
		return
	}
	_, c.sTokens = c.fetchTokens(true, c.projects)
	return nil
}

func (c *sdConnectInfo) levelCount() int {
	return 3
}

func (c *sdConnectInfo) getToken() string {
	return c.token
}

func (c *sdConnectInfo) getNthLevel(nodes ...string) ([]Metadata, error) {
	if len(nodes) == 0 {
		return c.projects, nil
	}

	token := c.getSTokenFromMap(nodes[0])
	headers := map[string]string{"X-Project-ID": token.ProjectID}
	path := c.metadataURL + "/project/" + url.PathEscape(nodes[0])
	if len(nodes) == 1 {
		path += "/containers"
	} else if len(nodes) == 2 {
		path += "/container/" + url.PathEscape(nodes[1]) + "/objects"
	} else {
		return nil, nil
	}

	var meta []Metadata
	err := makeRequest(path, token.Token, SDConnect, nil, headers, &meta)
	if c.tokenExpired(err) {
		token = c.getSTokenFromMap(nodes[0])
		err = makeRequest(path, token.Token, SDConnect, nil, headers, &meta)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s metadata for path %q: %w", SDConnect, nodes, err)
	}

	logs.Infof("Retrieved %s metadata for %q", SDConnect, nodes)
	return meta, nil
}

func (c *sdConnectInfo) getSTokenFromMap(project string) (token sToken) {
	value, ok := c.sTokens.Load(project)
	if ok {
		token = value.(sToken)
	}
	return
}

func (c *sdConnectInfo) tokenExpired(err error) bool {
	var re *RequestError
	if errors.As(err, &re) && re.StatusCode == 401 {
		logs.Infof("%s tokens no longer valid. Fetching them again", SDConnect)
		c.uToken, c.sTokens = c.fetchTokens(false, c.projects)
		return true
	}
	return false
}

func (c *sdConnectInfo) updateAttributes(nodes []string, path string, attr interface{}) {
	if len(nodes) < 3 {
		logs.Errorf("Cannot update attributes for path %q", path)
		return
	}

	size, ok := attr.(*int64)
	if !ok {
		logs.Errorf("%s updateAttributes() was called with incorrect attribute. Expected type *int64, got %v",
			SDConnect, reflect.TypeOf(attr))
		*size = -1
		return
	}

	var headers SpecialHeaders
	if err := c.downloadData(nodes, &headers, 0, 1); err != nil {
		logs.Error(fmt.Errorf("Encryption status and segmented object size of object %q could not be determined: %w", path, err))
		*size = -1
		return
	}
	if headers.SegmentedObjectSize != -1 {
		logs.Infof("Object %q is a segmented object with size %d", path, headers.SegmentedObjectSize)
		*size = headers.SegmentedObjectSize
	}
	if headers.Decrypted {
		dSize := calculateDecryptedSize(*size, headers.HeaderSize)
		if dSize != -1 {
			logs.Infof("Object %q is automatically decrypted", path)
			*size = dSize
		} else {
			logs.Warningf("API returned header 'X-Decrypted' even though size of object %q is too small", path)
		}
	}
}

func (c *sdConnectInfo) downloadData(nodes []string, buffer interface{}, start, end int64) error {
	// Query params
	query := map[string]string{
		"project":   nodes[0],
		"container": nodes[1],
		"object":    strings.Join(nodes[2:], "/"),
	}

	token := c.getSTokenFromMap(nodes[0])

	// Additional headers
	headers := map[string]string{"Range": "bytes=" + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end-1, 10),
		"X-Project-ID": token.ProjectID}

	path := c.dataURL + "/data"

	// Request data
	err := makeRequest(path, token.Token, SDConnect, query, headers, buffer)
	if c.tokenExpired(err) {
		token = c.getSTokenFromMap(nodes[0])
		return makeRequest(path, token.Token, SDConnect, query, headers, buffer)
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
