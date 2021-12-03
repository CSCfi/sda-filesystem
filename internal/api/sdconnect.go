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
)

// This file contains structs and functions that are strictly for SD-Connect

const SDConnect string = "SD-Connect"

type sdConnectInfo struct {
	loggedIn    bool
	metadataURL string
	dataURL     string
	certPath    string
	token       string
	uToken      string
	sTokens     map[string]sToken
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
	possibleRepositories[SDConnect] = &sdConnectInfo{sTokens: make(map[string]sToken)}
}

func (c *sdConnectInfo) getEnvs() error {
	var err error
	c.certPath, err = getEnv("FS_SD_CONNECT_CERTS", false)
	if err != nil {
		return err
	}
	c.metadataURL, err = getEnv("FS_SD_CONNECT_METADATA_API", true)
	if err != nil {
		return err
	}
	c.dataURL, err = getEnv("FS_SD_CONNECT_DATA_API", true)
	if err != nil {
		return err
	}
	return nil
}

func (c *sdConnectInfo) getLoginMethod() LoginMethod {
	return Password
}

func (c *sdConnectInfo) validateLogin(auth ...string) error {
	if len(auth) == 2 {
		c.token = base64.StdEncoding.EncodeToString([]byte(auth[0] + ":" + auth[1]))
	}

	c.projects = nil
	c.loggedIn = false

	err := c.getUToken()
	if err != nil {
		return err
	}

	c.projects, err = c.getProjects()
	if err != nil {
		return err
	}
	if len(c.projects) == 0 {
		return fmt.Errorf("No project permissions found for %s", SDConnect)
	}

	c.loggedIn = true
	c.fetchTokens()
	return nil
}

// fetchTokens fetches the unscoped token and the scoped tokens
func (c *sdConnectInfo) fetchTokens() {
	if !c.loggedIn {
		err := c.getUToken()
		if err != nil {
			logs.Warningf("HTTP requests may be slower for %s: %w", SDConnect, err)
			c.uToken = ""
			return
		}
	}

	c.sTokens = make(map[string]sToken)
	for i := range c.projects {
		err := c.getSToken(c.projects[i].Name)
		if err != nil {
			logs.Warningf("HTTP requests may be slower for %s files under %q: %w", SDConnect, c.projects[i].Name, err)
		}
	}

	logs.Infof("Fetched %s tokens", SDConnect)
}

// getUToken gets the unscoped token
func (c *sdConnectInfo) getUToken() error {
	// Request token
	uToken := uToken{}
	err := makeRequest(strings.TrimSuffix(c.metadataURL, "/")+"/token", "", SDConnect, nil, nil, &uToken)
	if err != nil {
		return fmt.Errorf("Failed to retrieve unscoped token: %w", err)
	}

	c.uToken = uToken.Token
	logs.Debug("Retrieved unscoped token for ", SDConnect)
	return nil
}

// GetSToken gets the scoped tokens for a project
func (c *sdConnectInfo) getSToken(project string) error {
	// Query params
	query := map[string]string{"project": project}

	// Request token
	sToken := sToken{}
	err := makeRequest(strings.TrimSuffix(c.metadataURL, "/")+"/token", "", SDConnect, query, nil, &sToken)
	if err != nil {
		return fmt.Errorf("Failed to retrieve %s scoped token for %q: %w", SDConnect, project, err)
	}

	c.sTokens[project] = sToken
	logs.Debug("Retrieved %s scoped token for ", SDConnect, project)
	return nil
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

func (c *sdConnectInfo) getToken() string {
	return c.token
}

func (c *sdConnectInfo) levelCount() int {
	return 3
}

func (c *sdConnectInfo) getNthLevel(nodes ...string) ([]Metadata, error) {
	switch len(nodes) {
	case 0:
		return c.projects, nil
	case 1:
		return c.getContainers(nodes[0])
	case 2:
		return c.getObjects(nodes[0], nodes[1])
	default:
		return nil, nil
	}
}

func (c *sdConnectInfo) isTokenExpired(err error) bool {
	var re *RequestError
	if c.loggedIn && errors.As(err, &re) && re.StatusCode == 401 {
		logs.Infof("%s tokens no longer valid. Fetching them again", SDConnect)
		c.loggedIn = false
		c.fetchTokens()
		return true
	}
	return false
}

func (c *sdConnectInfo) getProjects() ([]Metadata, error) {
	var projects []Metadata
	err := makeRequest(strings.TrimSuffix(c.metadataURL, "/")+"/projects", c.uToken, SDConnect, nil, nil, &projects)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s projects: %w", SDConnect, err)
	}

	logs.Infof("Retrieved %d %s project(s)", len(projects), SDConnect)
	return projects, nil
}

// GetContainers gets containers inside project
func (c *sdConnectInfo) getContainers(project string) ([]Metadata, error) {
	// Additional headers
	headers := map[string]string{"X-Project-ID": c.sTokens[project].ProjectID}

	// Request containers
	var containers []Metadata
	err := makeRequest(
		strings.TrimSuffix(c.metadataURL, "/")+
			"/project/"+
			url.PathEscape(project)+"/containers", c.sTokens[project].Token, SDConnect, nil, headers, &containers)

	if c.isTokenExpired(err) {
		return c.getContainers(project)
	}
	c.loggedIn = true

	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s containers for %q: %w", SDConnect, project, err)
	}

	logs.Infof("Retrieved %s containers for %q", SDConnect, project)
	return containers, nil
}

// GetObjects gets objects inside container
func (c *sdConnectInfo) getObjects(project, container string) ([]Metadata, error) {
	// Additional headers
	headers := map[string]string{"X-Project-ID": c.sTokens[project].ProjectID}

	// Request objects
	var objects []Metadata
	err := makeRequest(
		strings.TrimSuffix(c.metadataURL, "/")+
			"/project/"+
			url.PathEscape(project)+"/container/"+
			url.PathEscape(container)+"/objects", c.sTokens[project].Token, SDConnect, nil, headers, &objects)

	if c.isTokenExpired(err) {
		return c.getObjects(project, container)
	}
	c.loggedIn = true

	if err != nil {
		return nil, fmt.Errorf("Failed to retrieving %s objects for %q: %w", SDConnect, project+"/"+container, err)
	}

	logs.Infof("Retrieved %s objects for %q", SDConnect, project+"/"+container)
	return objects, nil
}

func (c *sdConnectInfo) updateAttributes(nodes []string, path string, attr interface{}) {
	size, ok := attr.(*int64)
	if !ok {
		logs.Errorf("%s updateAttributes() was called with incorrect attribute. Expected type *int64, got %v",
			SDConnect, reflect.TypeOf(attr))
		*size = -1
		return
	}

	headers, err := c.getSpecialHeaders(nodes, path)
	if err != nil {
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

// getSpecialHeaders returns information on headers that can only be retirived from data api
func (c *sdConnectInfo) getSpecialHeaders(nodes []string, path string) (SpecialHeaders, error) {
	project := nodes[0]

	// Query params
	query := map[string]string{
		"project":   nodes[0],
		"container": nodes[1],
		"object":    strings.Join(nodes[2:], "/"),
	}

	// Additional headers
	headers := map[string]string{"Range": "bytes=0-1", "X-Project-ID": c.sTokens[project].ProjectID}

	var ret SpecialHeaders
	err := makeRequest(strings.TrimSuffix(c.dataURL, "/")+"/data", c.sTokens[project].Token, SDConnect, query, headers, &ret)

	if c.isTokenExpired(err) {
		return c.getSpecialHeaders(nodes, path)
	}
	c.loggedIn = true

	if err != nil {
		return ret, fmt.Errorf("Failed to retrieve headers for %q: %w", path, err)
	}
	return ret, nil
}

// calculateDecryptedSize calculates the decrypted size of an encrypted file size
func calculateDecryptedSize(fileSize, headerSize int64) int64 {
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

func (c *sdConnectInfo) downloadData(nodes []string, buffer []byte, start, end int64) error {
	project := nodes[0]

	// Query params
	query := map[string]string{
		"project":   nodes[0],
		"container": nodes[1],
		"object":    strings.Join(nodes[2:], "/"),
	}

	// Additional headers
	headers := map[string]string{"Range": "bytes=" + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end-1, 10),
		"X-Project-ID": c.sTokens[project].ProjectID}

	// Request data
	err := makeRequest(strings.TrimSuffix(c.dataURL, "/")+"/data", c.sTokens[project].Token, SDConnect, query, headers, buffer)

	if c.isTokenExpired(err) {
		return c.downloadData(nodes, buffer, start, end)
	}
	c.loggedIn = true
	return err
}
