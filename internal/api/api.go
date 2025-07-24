package api

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"sda-filesystem/internal/cache"
	"sda-filesystem/internal/logs"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/neicnordic/crypt4gh/keys"
	"golang.org/x/crypto/chacha20poly1305"
)

const SDApply string = "SD-Apply"
const SDConnect string = "SD-Connect"
const Findata string = "Findata"

var Port string // Defined at build time if binary is for older Ubuntu

var ai = apiInfo{
	hi: httpInfo{
		requestTimeout: 60,
		httpRetry:      3,
		client:         &http.Client{Transport: http.DefaultTransport},
	},
	vi: vaultInfo{
		keyName: uuid.NewString(),
	},
	repositories: []string{}, // SD Apply will be added here once it works with S3
}
var downloadCache *cache.Ristretto

// FileReader is used as a variable type instead of embed.FS so that mocking during tests is easier
type FileReader interface {
	ReadFile(name string) ([]byte, error)
}

// apiInfo contains variables required for Data Gateway to work with KrakendD
type apiInfo struct {
	proxy        string
	token        string
	password     string // base64 encoded
	repositories []string
	userProfile  profile
	hi           httpInfo
	vi           vaultInfo
}

// httpInfo contains variables used during HTTP requests
type httpInfo struct {
	requestTimeout int
	s3Timeout      int
	httpRetry      int
	endpoints      apiEndpoints
	client         *http.Client
	s3Client       *s3.Client
}

type profile struct {
	Username     string `json:"username"`
	ProjectName  string `json:"project"`
	ProjectType  string `json:"projectType"`
	DesktopToken string `json:"access_token"`
	PI           bool   `json:"PI"`
	SDConnect    bool   `json:"sdConnect"`
	S3Access     bool   `json:"s3Access"`
}

type configResponse struct {
	Timeouts struct {
		S3 int `json:"s3"`
	} `json:"timeouts"`
	Endpoints apiEndpoints `json:"endpoints"`
}

type apiEndpoints struct {
	Profile     string `json:"profile"`
	Password    string `json:"valid_password"`
	AllasHeader string `json:"allas_header"`
	S3          struct {
		Default string `json:"default"`
		Head    string `json:"head"`
	} `json:"s3"`
	Vault struct {
		Key       string `json:"project_key"`
		Headers   string `json:"headers"`
		Whitelist string `json:"whitelist"`
	} `json:"vault"`
}

// RequestError is used to obtain the status code from a HTTP request
type RequestError struct {
	StatusCode int
	errStr     string
}

func (re *RequestError) Error() (err string) {
	if re.errStr != "" {
		krakendErr := struct { // Sometimes KrakenD sends an error in this format
			Message string `json:"message"`
			Status  int    `json:"status"`
		}{}
		unmarshalErr := json.Unmarshal([]byte(re.errStr), &krakendErr)
		if unmarshalErr != nil {
			err = fmt.Sprintf("%d %s", re.StatusCode, re.errStr)
		} else {
			err = fmt.Sprintf("%d %s", krakendErr.Status, krakendErr.Message)
		}
	} else {
		err = fmt.Sprintf("%d %s", re.StatusCode, http.StatusText(re.StatusCode))
	}

	return
}

type CredentialsError struct {
}

func (e *CredentialsError) Error() string {
	return "Incorrect password"
}

// SetRequestTimeout redefines the timeout for an http request
var SetRequestTimeout = func(timeout int) {
	ai.hi.requestTimeout = timeout
}

// GetEnv looks up environment variable given in `name`
var GetEnv = func(name string, verifyURL bool) (string, error) {
	env, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("environment variable %s not set", name)
	}
	if verifyURL {
		if _, err := url.ParseRequestURI(env); err != nil {
			return "", fmt.Errorf("environment variable %s not a valid URL: %w", name, err)
		}
	}

	return env, nil
}

// Setup reads the necessary environment varibles needed for requests,
// generates key pair for vault, and initialises s3 client.
// `files` contains all the files from the `certs` directory.
func Setup(files ...FileReader) error {
	var err error
	ai.token, err = GetEnv("SDS_ACCESS_TOKEN", false)
	if err != nil {
		return fmt.Errorf("required environment variables missing: %w", err)
	}
	proxy, err := GetEnv("PROXY_URL", true)
	if err != nil {
		return fmt.Errorf("required environment variables missing: %w", err)
	}

	var config string
	if Port != "" {
		proxyURL, _ := url.ParseRequestURI(proxy)
		proxyHost := proxyURL.Hostname()
		proxyURL.Host = fmt.Sprintf("%s:%s", proxyHost, Port)
		proxy = proxyURL.String()

		config = proxy + "/static/configuration.json"
	} else if config, err = GetEnv("CONFIG_ENDPOINT", true); err != nil {
		return fmt.Errorf("required environment variables missing: %w", err)
	}

	ai.proxy = "" // So that GUI refresh works during development
	// This needs to be called first before any other http requests
	if err = getAPIEndpoints(config); err != nil {
		return fmt.Errorf("failed to get static configuration.json file: %w", err)
	}
	ai.proxy = proxy

	downloadCache, err = cache.NewRistrettoCache()
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}

	if err := loadCertificates(files); err != nil {
		return fmt.Errorf("failed to load certficates: %w", err)
	}

	if err = initialiseS3Client(); err != nil {
		return fmt.Errorf("failed to initialise S3 client: %w", err)
	}

	var publicKey [chacha20poly1305.KeySize]byte
	publicKey, ai.vi.privateKey, err = keys.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	ai.vi.publicKey = base64.StdEncoding.EncodeToString(publicKey[:])

	return nil
}

func getAPIEndpoints(url string) error {
	// Since ai.proxy is still empty, giving the url as path works here
	var resp configResponse
	if err := makeRequest("GET", url, nil, nil, nil, &resp); err != nil {
		return err
	}

	ai.hi.s3Timeout = resp.Timeouts.S3 + 5
	ai.hi.endpoints = resp.Endpoints

	return nil
}

func GetProfile() (bool, error) {
	err := makeRequest("GET", ai.hi.endpoints.Profile, nil, nil, nil, &ai.userProfile)
	if err != nil {
		return false, fmt.Errorf("failed to get user profile: %w", err)
	}
	ai.token = ai.userProfile.DesktopToken
	if ai.userProfile.SDConnect && !slices.Contains(ai.repositories, SDConnect) {
		ai.repositories = append(ai.repositories, SDConnect)
	}

	return ai.userProfile.S3Access, nil
}

// Authenticate checks that user passes the authentication checks in KrakenD with the given password.
var Authenticate = func(password string) error {
	ai.password = base64.StdEncoding.EncodeToString([]byte(password))
	err := makeRequest("GET", ai.hi.endpoints.Password, nil, nil, nil, nil)
	if err != nil {
		ai.password = ""

		var re *RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			return &CredentialsError{}
		}

		return fmt.Errorf("failed to authenicate user: %w", err)
	}

	return nil
}

// loadCertificates loads the certificate files from the `certs` directory and expects
// them to be in the format <hostname>.crt and <hostname>.key. If no such files are found,
// a warning is logged but no error is returned. If the files are successfully parsed,
// the certificates are added to the http client.
var loadCertificates = func(certFiles []FileReader) error {
	if len(certFiles) == 0 {
		return nil
	}

	u, err := url.ParseRequestURI(ai.proxy)
	if err != nil {
		return fmt.Errorf("could not parse proxy url: %w", err)
	}
	proxyHost := u.Hostname()

	certBytes, err1 := certFiles[0].ReadFile(proxyHost + ".crt")
	keyBytes, err2 := certFiles[0].ReadFile(proxyHost + ".key")
	if err1 != nil || err2 != nil {
		err = errors.Join(errors.New("disabled mTLS for S3 upload"), err1, err2)
		logs.Warning(err)

		return nil
	}

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return fmt.Errorf("failed to load client x509 key pair for host %s: %w", proxyHost, err)
	}

	tr := ai.hi.client.Transport.(*http.Transport).Clone()
	tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
	ai.hi.client = &http.Client{Transport: tr}

	logs.Debug("Certificate successfully added to client")

	return nil
}

// ToPrint returns repository name in printable format
func ToPrint(rep string) string {
	return strings.ReplaceAll(rep, "-", " ")
}

// GetAllRepositories returns the list of all possible repositories
func GetAllRepositories() []string {
	return []string{SDConnect, SDApply}
}

// GetRepositories returns the list of repositories the filesystem can access
var GetRepositories = func() []string {
	return ai.repositories
}

func GetUsername() string {
	return ai.userProfile.Username
}

var GetProjectName = func() string {
	return ai.userProfile.ProjectName
}

var GetProjectType = func() string {
	return ai.userProfile.ProjectType
}

var SDConnectEnabled = func() bool {
	return ai.userProfile.SDConnect
}

var IsProjectManager = func() bool {
	return ai.userProfile.PI
}

// makeRequest sends HTTP request to KrakenD and parses the response
var makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
	var response *http.Response

	// Build HTTP request
	request, err := http.NewRequest(method, ai.proxy+path, reqBody)
	if err != nil {
		return fmt.Errorf("creating request failed: %w", err)
	}

	// Adjust request timeout
	timeout := time.Duration(ai.hi.requestTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	request = request.WithContext(ctx)
	defer cancel()

	// Place query params if they are set
	q := request.URL.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	request.URL.RawQuery = q.Encode()

	request.Header.Set("Authorization", "Bearer "+ai.token)
	if ai.password != "" {
		request.Header.Set("CSC-Password", ai.password)
	}

	// Place additional headers if any are available
	for k, v := range headers {
		request.Header.Set(k, v)
	}

	escapedURL := strings.ReplaceAll(request.URL.EscapedPath(), "\n", "")
	escapedURL = strings.ReplaceAll(escapedURL, "\r", "")

	// Execute HTTP request
	// Retry the request as specified by ai.hi.httpRetry variable
	count := 0
	for {
		response, err = ai.hi.client.Do(request)
		logs.Debugf("Trying Request %s, attempt %d/%d", escapedURL, count+1, ai.hi.httpRetry)
		count++

		if err != nil && count >= ai.hi.httpRetry {
			return err
		}
		if err == nil {
			break
		}
	}
	defer response.Body.Close()

	// Read response body (size unknown)
	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if response.StatusCode >= 400 {
		return &RequestError{response.StatusCode, string(respBody)}
	}

	// Parse response
	if ret != nil {
		if err := json.Unmarshal(respBody, ret); err != nil {
			return fmt.Errorf("unable to decode response: %w", err)
		}
	}

	logs.Debugf("Request %s returned a response", escapedURL)

	return nil
}

func toCacheKey(nodes []string, chunkIdx int64) string {
	return strings.Join(nodes, "/") + "_" + strconv.FormatInt(chunkIdx, 10)
}

// ClearCache empties the entire ristretto cache
var ClearCache = func() {
	downloadCache.Clear()
}

// DeleteFileFromCache clears all entries from a given file/object from cache
var DeleteFileFromCache = func(nodes []string, size int64) {
	i := int64(0)
	for i < size {
		key := toCacheKey(nodes, i)
		downloadCache.Del(key)
		i += chunkSize
	}
}
