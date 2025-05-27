package api

import (
	"context"
	"crypto/tls"
	"embed"
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

const SDSubmit string = "SD-Apply"
const SDConnect string = "SD-Connect"

var ai = apiInfo{
	hi: httpInfo{
		requestTimeout: 60,
		httpRetry:      3,
		client:         &http.Client{},
	},
	vi: vaultInfo{
		keyName: uuid.NewString(),
	},
	repositories: []string{}, // SD Apply will be added here once it works with S3
}
var downloadCache *cache.Ristretto

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
	httpRetry      int
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
// The parameter `files` is empty if the program does not need mTLS enabled.
// Otherwise `files` contains all the files from the `certs` directory.
func Setup(files ...embed.FS) error {
	var err error
	ai.proxy, err = GetEnv("PROXY_URL", true)
	if err != nil {
		return fmt.Errorf("required environment variables missing: %w", err)
	}

	ai.token, err = GetEnv("SDS_ACCESS_TOKEN", false)
	if err != nil {
		return fmt.Errorf("required environment variables missing: %w", err)
	}

	downloadCache, err = cache.NewRistrettoCache()
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}

	certs, err := loadCertificates(files)
	if err != nil {
		return fmt.Errorf("failed to load certficates: %w", err)
	}

	if err = initialiseS3Client(certs); err != nil {
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

func GetProfile() (bool, error) {
	err := MakeRequest("GET", "/profile", nil, nil, nil, &ai.userProfile)
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
	err := MakeRequest("GET", "/credentials/check", nil, nil, nil, nil)
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

// loadCertificates loads the certificate files from the `./certs` directory and expects
// them to be in the format <hostname>.crt and <hostname>.key. If no such files are found,
// a warning is logged but no error is returned. If the files are successfully parsed,
// the certificates are added to the http client.
func loadCertificates(certFiles []embed.FS) ([]tls.Certificate, error) {
	if len(certFiles) == 0 {
		return nil, nil
	}

	u, err := url.Parse(ai.proxy)
	if err != nil {
		return nil, fmt.Errorf("could not parse proxy url: %w", err)
	}
	proxyHost := u.Hostname()

	certBytes, err1 := certFiles[0].ReadFile(proxyHost + ".crt")
	keyBytes, err2 := certFiles[0].ReadFile(proxyHost + ".key")
	if err1 != nil || err2 != nil {
		err = errors.Join(errors.New("disabled mTLS for S3 upload"), err1, err2)
		logs.Warning(err)

		return nil, nil
	}

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load client x509 key pair for host %s", proxyHost)
	}

	certificates := []tls.Certificate{cert}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig.Certificates = certificates
	ai.hi.client = &http.Client{Transport: tr}

	return certificates, nil
}

// ToPrint returns repository name in printable format
func ToPrint(rep string) string {
	return strings.ReplaceAll(rep, "-", " ")
}

// GetAllRepositories returns the list of all possible repositories
func GetAllRepositories() []string {
	return []string{SDConnect, SDSubmit}
}

// GetRepositories returns the list of repositories the filesystem can access
func GetRepositories() []string {
	return ai.repositories
}

func GetUsername() string {
	return ai.userProfile.Username
}

func GetProjectName() string {
	return ai.userProfile.ProjectName
}

func GetProjectType() string {
	return ai.userProfile.ProjectType
}

func SDConnectEnabled() bool {
	return ai.userProfile.SDConnect
}

func IsProjectManager() bool {
	return ai.userProfile.PI
}

// MakeRequest sends HTTP request to KrakenD and parses the response
var MakeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
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
