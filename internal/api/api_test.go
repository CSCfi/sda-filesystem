package api

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"sda-filesystem/internal/cache"
	"sda-filesystem/internal/logs"

	"github.com/neicnordic/crypt4gh/keys"
)

var errExpected = errors.New("expected error for test")
var testConfig apiEndpoints

type MockReader struct {
	Files map[string][]byte
}

func (m MockReader) ReadFile(name string) ([]byte, error) {
	data, ok := m.Files[name]
	if !ok {
		return nil, fmt.Errorf("file does not exist")
	}

	return data, nil
}

func setupCerts(filename string) ([]byte, MockReader, error) {
	ca := &x509.Certificate{
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(1 * time.Hour),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, MockReader{}, fmt.Errorf("failed to generate CA private key: %w", err)
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, MockReader{}, fmt.Errorf("failed to create CA: %w", err)
	}

	// pem encode
	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	cert := &x509.Certificate{
		Subject:     pkix.Name{CommonName: "Data Gateway client"},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(1 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"something"},
	}
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, MockReader{}, fmt.Errorf("failed to generate certificate private key: %w", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, MockReader{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	certFile := filename + ".crt"
	keyFile := filename + ".key"

	reader := MockReader{Files: map[string][]byte{certFile: certPEM, keyFile: keyPEM}}

	return caPEM, reader, nil
}

type mockCache struct {
	cache.Cacheable

	keys map[string][]byte
}

func (ms *mockCache) Del(key string) {
	delete(ms.keys, key)
}

func (ms *mockCache) Get(key string) ([]byte, bool) {
	cached, ok := ms.keys[key]

	return cached, ok
}

func (ms *mockCache) Set(key string, data []byte, _ int64, _ time.Duration) bool {
	ms.keys[key] = data

	return true
}

func init() {
	testConfig = apiEndpoints{
		Profile:       "/profile-endpoint",
		Password:      "/password-endpoint",
		AllasHeader:   "/allas-header-endpoint/",
		SharedBuckets: "/shared-buckets-endpoint",
	}
	testConfig.S3.Default = "/s3-default-endpoint/"
	testConfig.S3.Head = "/s3-head-endpoint/"

	testConfig.Vault.Key = "/project-key-endpoint"
	testConfig.Vault.Headers = "/headers-endpoint"
	testConfig.Vault.Whitelist = "/whitelist-endpoint/"
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(string, []string) {})
	os.Exit(m.Run())
}

func TestRequestError(t *testing.T) {
	var tests = []struct {
		testname, inErr, outErr string
		code                    int
	}{
		{"OK_1", "", "Internal Server Error", 500},
		{"OK_2", "", "Not Found", 404},
		{"OK_3", errExpected.Error(), errExpected.Error(), 403},
		{"OK_4", "{\"message\": \"" + errExpected.Error() + "\", \"status\": 404}", errExpected.Error(), 404},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			re := RequestError{tt.code, tt.inErr}

			errCompare := fmt.Sprintf("%d %s", tt.code, tt.outErr)
			if re.Error() != errCompare {
				t.Errorf("RequestError has incorrect format\nExpected=%s\nReceived=%s", errCompare, re.Error())
			}
		})
	}
}

func TestRequestTimeout(t *testing.T) {
	timeouts := []int{34, 6, 1200, 84}

	for i := range timeouts {
		SetRequestTimeout(timeouts[i])
		if ai.hi.requestTimeout != timeouts[i] {
			t.Errorf("Incorrect request timeout. Expected=%d, received=%d", timeouts[i], ai.hi.requestTimeout)
		}
	}
}

func TestGetEnv(t *testing.T) {
	var tests = []struct {
		testname, envName, envValue, errText string
		verifyURL                            bool
	}{
		{"OK_1", "MUUTTUJA234", "banana", "", false},
		{"OK_2", "MUUTTUJA9476", "https://github.com", "", true},
		{
			"FAIL_INVALID_URL", "MUUTTUJA0346", "google.com",
			"environment variable MUUTTUJA0346 not a valid URL: parse \"google.com\": invalid URI for request",
			true,
		},
		{"FAIL_NOT_SET", "MUUTTUJA195", "", "environment variable MUUTTUJA195 not set", false},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envName, tt.envValue)
			} else {
				os.Unsetenv(tt.envName)
			}

			value, err := GetEnv(tt.envName, tt.verifyURL)
			os.Unsetenv(tt.envName)

			switch {
			case tt.errText == "":
				if err != nil {
					t.Errorf("Returned unexpected err: %s", err.Error())
				} else if value != tt.envValue {
					t.Errorf("Environment variable has incorrect value. Expected=%s, received=%s", tt.envValue, value)
				}
			case err == nil:
				t.Error("Function should have returned error")
			case err.Error() != tt.errText:
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errText, err.Error())
			}
		})
	}
}

func TestSetup(t *testing.T) {
	origGetEnv := GetEnv
	origMakeRequest := makeRequest
	origNewRistretto := cache.NewRistrettoCache
	origLoadCertificates := loadCertificates
	origInitialiseS3Client := initialiseS3Client
	origProxy := ai.proxy
	origToken := ai.token
	origS3Timeout := ai.hi.s3Timeout
	defer func() {
		GetEnv = origGetEnv
		makeRequest = origMakeRequest
		cache.NewRistrettoCache = origNewRistretto
		loadCertificates = origLoadCertificates
		initialiseS3Client = origInitialiseS3Client
		ai.proxy = origProxy
		ai.token = origToken
		ai.hi.s3Timeout = origS3Timeout
	}()

	ai.hi.endpoints = apiEndpoints{}

	GetEnv = func(name string, verifyURL bool) (string, error) {
		switch name {
		case "PROXY_URL":
			return "test_url", nil
		case "SDS_ACCESS_TOKEN":
			return "test_token", nil
		case "CONFIG_ENDPOINT":
			return "config_url", nil
		default:
			return "", fmt.Errorf("unknown env %s", name)
		}
	}
	makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if ai.proxy != "" {
			t.Errorf("ai.proxy should be empty, received=%s", ai.proxy)
		}
		if path != "config_url" {
			return fmt.Errorf("Incorrect path \nExpected=config_url\nReceived=%s", path)
		}

		switch v := ret.(type) {
		case *configResponse:
			v.Timeouts.S3 = 13
			v.Endpoints = testConfig

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *configResponse", reflect.TypeOf(v))
		}
	}
	newCache := &cache.Ristretto{Cacheable: &mockCache{keys: make(map[string][]byte)}}
	cache.NewRistrettoCache = func() (*cache.Ristretto, error) {
		return newCache, nil
	}
	mockFiles := MockReader{Files: map[string][]byte{"filename": {4, 78, 95, 90}}}
	loadCertificates = func(certFiles FileReader) error {
		if !reflect.DeepEqual(certFiles, mockFiles) {
			return fmt.Errorf("loadCertificates() received invalid argument")
		}

		return nil
	}
	initialiseS3Client = func() error {
		return nil
	}

	err := Setup(mockFiles)
	if err != nil {
		t.Fatalf("Function returned error: %s", err.Error())
	}
	if ai.proxy != "test_url" {
		t.Errorf("Proxy has incorrect value\nExpected=test_url\nReceived=%s", ai.proxy)
	}
	if ai.token != "test_token" {
		t.Errorf("Token has incorrect value\nExpected=test_token\nReceived=%s", ai.token)
	}
	if ai.hi.s3Timeout != 18 {
		t.Errorf("S3 timeout has incorrect value\nExpected=18\nReceived=%v", ai.hi.s3Timeout)
	}
	if !reflect.DeepEqual(ai.hi.endpoints, testConfig) {
		t.Errorf("Config json has incorrect value\nExpected=%v\nReceived=%v", testConfig, ai.hi.s3Timeout)
	}
	if downloadCache != newCache {
		t.Errorf("downloadCache does not point to new cache")
	}
	publicKey := keys.DerivePublicKey(ai.vi.privateKey)
	publicKey64 := base64.StdEncoding.EncodeToString(publicKey[:])
	if publicKey64 != ai.vi.publicKey {
		t.Errorf("Public key has incorrect value\nExpected=%v\nReceived=%v", publicKey, ai.vi.publicKey)
	}
}

func TestSetup_Port(t *testing.T) {
	var tests = []struct {
		testname, expectedProxy string
		setOverride             bool
	}{
		{"OK_1", "http://localhost:8081", false},
		{"OK_2", "override_url", true},
	}

	origGetEnv := GetEnv
	origMakeRequest := makeRequest
	origNewRistretto := cache.NewRistrettoCache
	origLoadCertificates := loadCertificates
	origInitialiseS3Client := initialiseS3Client
	origProxy := ai.proxy
	origToken := ai.token
	origS3Timeout := ai.hi.s3Timeout
	defer func() {
		GetEnv = origGetEnv
		makeRequest = origMakeRequest
		cache.NewRistrettoCache = origNewRistretto
		loadCertificates = origLoadCertificates
		initialiseS3Client = origInitialiseS3Client
		ai.proxy = origProxy
		ai.token = origToken
		ai.hi.s3Timeout = origS3Timeout
		Port = ""
	}()

	ai.hi.endpoints = apiEndpoints{}
	os.Unsetenv("OVERRIDE_PROXY_URL")

	GetEnv = func(name string, verifyURL bool) (string, error) {
		switch name {
		case "PROXY_URL":
			return "http://localhost:80", nil
		case "SDS_ACCESS_TOKEN":
			return "test_token", nil
		case "CONFIG_ENDPOINT":
			return "config_url", nil
		case "OVERRIDE_PROXY_URL":
			return "override_url", nil
		default:
			return "", fmt.Errorf("unknown env %s", name)
		}
	}
	newCache := &cache.Ristretto{Cacheable: &mockCache{keys: make(map[string][]byte)}}
	cache.NewRistrettoCache = func() (*cache.Ristretto, error) {
		return newCache, nil
	}
	mockFiles := MockReader{Files: map[string][]byte{"filename": {4, 78, 95, 90}}}
	loadCertificates = func(certFiles FileReader) error {
		if !reflect.DeepEqual(certFiles, mockFiles) {
			return fmt.Errorf("loadCertificates() received invalid argument")
		}

		return nil
	}
	initialiseS3Client = func() error {
		return nil
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
				if ai.proxy != "" {
					t.Errorf("ai.proxy should be empty, received=%s", ai.proxy)
				}
				expectedPath := tt.expectedProxy + "/static/configuration.json"
				if path != expectedPath {
					t.Errorf("makeRequest() received incorrect path\nExpected=%s\nReceived=%s", expectedPath, path)
				}

				switch v := ret.(type) {
				case *configResponse:
					v.Timeouts.S3 = 13
					v.Endpoints = testConfig

					return nil
				default:
					return fmt.Errorf("ret has incorrect type %v, expected *configResponse", reflect.TypeOf(v))
				}
			}

			if tt.setOverride {
				os.Setenv("OVERRIDE_PROXY_URL", "")
				t.Cleanup(func() { os.Unsetenv("OVERRIDE_PROXY_URL") })
			}

			Port = "8081"
			err := Setup(mockFiles)
			if err != nil {
				t.Fatalf("Function returned error: %s", err.Error())
			}
			if ai.proxy != tt.expectedProxy {
				t.Errorf("Proxy has incorrect value\nExpected=%s\nReceived=%s", tt.expectedProxy, ai.proxy)
			}
			if ai.token != "test_token" {
				t.Errorf("Token has incorrect value\nExpected=test_token\nReceived=%s", ai.token)
			}
			if ai.hi.s3Timeout != 18 {
				t.Errorf("S3 timeout has incorrect value\nExpected=18\nReceived=%v", ai.hi.s3Timeout)
			}
			if !reflect.DeepEqual(ai.hi.endpoints, testConfig) {
				t.Errorf("Config json has incorrect value\nExpected=%v\nReceived=%v", testConfig, ai.hi.s3Timeout)
			}
			if downloadCache != newCache {
				t.Errorf("downloadCache does not point to new cache")
			}
			publicKey := keys.DerivePublicKey(ai.vi.privateKey)
			publicKey64 := base64.StdEncoding.EncodeToString(publicKey[:])
			if publicKey64 != ai.vi.publicKey {
				t.Errorf("Public key has incorrect value\nExpected=%v\nReceived=%v", publicKey, ai.vi.publicKey)
			}
		})
	}
}

func TestSetup_Error(t *testing.T) {
	var tests = []struct {
		testname, errText                                                            string
		tokenErr, configErr, reqErr, proxyErr, cacheErr, certErr, s3Err, overrideErr error
	}{
		{"FAIL_1", "required environment variables missing", errExpected, nil, nil, nil, nil, nil, nil, nil},
		{"FAIL_2", "required environment variables missing", nil, errExpected, nil, nil, nil, nil, nil, nil},
		{"FAIL_3", "failed to get static configuration.json file", nil, nil, errExpected, nil, nil, nil, nil, nil},
		{"FAIL_4", "required environment variables missing", nil, nil, nil, errExpected, nil, nil, nil, nil},
		{"FAIL_5", "failed to create cache", nil, nil, nil, nil, errExpected, nil, nil, nil},
		{"FAIL_6", "failed to load certficates", nil, nil, nil, nil, nil, errExpected, nil, nil},
		{"FAIL_7", "failed to initialise S3 client", nil, nil, nil, nil, nil, nil, errExpected, nil},
		{"FAIL_8", "invalid environment variable", nil, nil, nil, nil, nil, nil, nil, errExpected},
	}

	origGetEnv := GetEnv
	origMakeRequest := makeRequest
	origNewRistretto := cache.NewRistrettoCache
	origLoadCertificates := loadCertificates
	origInitialiseS3Client := initialiseS3Client
	origProxy := ai.proxy
	origToken := ai.token
	defer func() {
		GetEnv = origGetEnv
		makeRequest = origMakeRequest
		cache.NewRistrettoCache = origNewRistretto
		loadCertificates = origLoadCertificates
		initialiseS3Client = origInitialiseS3Client
		ai.proxy = origProxy
		ai.token = origToken
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			GetEnv = func(name string, verifyURL bool) (string, error) {
				switch name {
				case "PROXY_URL":
					return "", tt.proxyErr
				case "SDS_ACCESS_TOKEN":
					return "", tt.tokenErr
				case "CONFIG_ENDPOINT":
					return "", tt.configErr
				case "OVERRIDE_PROXY_URL":
					return "", tt.overrideErr
				default:
					return "", fmt.Errorf("unknown env %s", name)
				}
			}
			makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
				return tt.reqErr
			}
			cache.NewRistrettoCache = func() (*cache.Ristretto, error) {
				return nil, tt.cacheErr
			}
			loadCertificates = func(certFiles FileReader) error {
				return tt.certErr
			}
			initialiseS3Client = func() error {
				return tt.s3Err
			}

			if tt.testname == "FAIL_8" {
				Port = "8080"
				os.Setenv("OVERRIDE_PROXY_URL", "")
				t.Cleanup(func() { os.Unsetenv("OVERRIDE_PROXY_URL") })
			}

			err := Setup(MockReader{})
			errText := tt.errText + ": " + errExpected.Error()
			if err == nil {
				t.Error("Function should have returned error")
			} else if err.Error() != errText {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
			}
		})
	}
}

func TestGetProfile(t *testing.T) {
	origMakeRequest := makeRequest
	origRepositories := ai.repositories
	origGetEnv := GetEnv
	defer func() {
		ai.repositories = origRepositories
		makeRequest = origMakeRequest
		GetEnv = origGetEnv
	}()

	ai.hi.endpoints = testConfig

	accessOnce := true
	makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if path != "/profile-endpoint" {
			return fmt.Errorf("Incorrect path\nExpected=/profile-endpoint\nReceived=%s", path)
		}

		switch v := ret.(type) {
		case *profile:
			v.DesktopToken = "test_token"
			v.S3Access = accessOnce
			v.SDConnect = true
			if accessOnce {
				v.ProjectType = "findata"
			} else {
				v.ProjectType = "default"
			}
			accessOnce = false

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *profile", reflect.TypeOf(v))
		}
	}
	GetEnv = func(name string, verifyURL bool) (string, error) {
		switch name {
		case "CLAMAV_SOCKET":
			return "socket_path", nil
		default:
			return "", fmt.Errorf("unknown env %s", name)
		}
	}
	ai.repositories = []Repo{"mock-repo"}
	finalRepositories := []Repo{"mock-repo", SDConnect}
	testCh := make(chan bool)

	access, err := GetProfile(testCh)
	switch {
	case err != nil:
		t.Errorf("First call returned error: %s", err.Error())
	case !access:
		t.Errorf("User should have access")
	case ai.token != "test_token":
		t.Errorf("Incorrect token\nExpected=test_token\nReceived=%s", ai.token)
	case !reflect.DeepEqual(ai.repositories, finalRepositories):
		t.Errorf("Incorrect repositories\nExpected=%v\nReceived=%v", finalRepositories, ai.repositories)
	case ai.scan != testCh:
		t.Errorf("Incorrect scanning channel\nExpected=%v\nReceived=%v", testCh, ai.scan)
	case ai.ui.address != "socket_path":
		t.Errorf("Incorrect ClamAV socket path\nExpected=socket_path\nReceived=%s", ai.ui.address)
	}

	GetEnv = func(name string, verifyURL bool) (string, error) {
		return "", fmt.Errorf("unknown env %s", name)
	}

	access, err = GetProfile(testCh)
	switch {
	case err != nil:
		t.Errorf("Second call returned error: %s", err.Error())
	case access:
		t.Errorf("User should not have access")
	case ai.token != "test_token":
		t.Errorf("Incorrect token\nExpected=test_token\nReceived=%s", ai.token)
	case !reflect.DeepEqual(ai.repositories, finalRepositories):
		t.Errorf("Incorrect repositories\nExpected=%v\nReceived=%v", finalRepositories, ai.repositories)
	}

	select {
	case <-testCh:
	default:
		t.Error("Scanning channel is not closed")
	}
}

func TestGetProfile_Error(t *testing.T) {
	origMakeRequest := makeRequest
	origRepositories := ai.repositories
	origGetEnv := GetEnv
	defer func() {
		ai.repositories = origRepositories
		makeRequest = origMakeRequest
		GetEnv = origGetEnv
	}()

	makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		return errExpected
	}
	GetEnv = func(name string, verifyURL bool) (string, error) {
		return "", nil
	}
	errText := "failed to get user profile: " + errExpected.Error()

	if _, err := GetProfile(); err == nil {
		t.Error("Function should have returned error")
	} else if err.Error() != errText {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
	}
}

func TestGetProfile_Clamav(t *testing.T) {
	origMakeRequest := makeRequest
	origRepositories := ai.repositories
	origGetEnv := GetEnv
	defer func() {
		ai.repositories = origRepositories
		makeRequest = origMakeRequest
		GetEnv = origGetEnv
	}()

	projectType := "findata"
	makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		switch v := ret.(type) {
		case *profile:
			v.ProjectType = projectType

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected *profile", reflect.TypeOf(v))
		}
	}
	GetEnv = func(name string, verifyURL bool) (string, error) {
		return "", errExpected
	}
	errText := "required environment variables missing: " + errExpected.Error()

	if _, err := GetProfile(); err == nil {
		t.Error("Function should have returned error")
	} else if err.Error() != errText {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
	}

	projectType = "default"

	if _, err := GetProfile(); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
}

func TestAuthenticate(t *testing.T) {
	origMakeRequest := makeRequest
	origPassword := ai.password
	defer func() {
		ai.password = origPassword
		makeRequest = origMakeRequest
	}()

	password := "passw0rd"
	ai.hi.endpoints = testConfig

	makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if path != "/password-endpoint" {
			return fmt.Errorf("Incorrect path\nExpected=/password-endpoint\nReceived=%s", path)
		}
		if ai.password != "cGFzc3cwcmQ=" {
			return fmt.Errorf("Incorrect password\nExpected=cGFzc3cwcmQ=\nReceived=%s", ai.password)
		}

		return nil
	}

	if err := Authenticate(password); err != nil {
		t.Errorf("Function returned error: %s", err.Error())
	}
}

func TestAuthenticate_Error(t *testing.T) {
	var tests = []struct {
		testname, errText string
		requestErr        error
	}{
		{"FAIL_1", "Incorrect password", &RequestError{StatusCode: 401}},
		{"FAIL_2", "failed to authenicate user: " + errExpected.Error(), errExpected},
	}

	origMakeRequest := makeRequest
	origPassword := ai.password
	defer func() {
		ai.password = origPassword
		makeRequest = origMakeRequest
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			makeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
				return tt.requestErr
			}

			if err := Authenticate("mock-password"); err == nil {
				t.Error("Function should have returned error")
			} else if err.Error() != tt.errText {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errText, err.Error())
			}
		})
	}
}

func TestLoadCertificates(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
	}()

	ai.proxy = "http://localhost:8080"
	ai.hi.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // #nosec G402
			},
		},
	}

	caPEM, mockReader, err := setupCerts("localhost")
	if err != nil {
		t.Fatalf("Could not setup certificates: %s", err.Error())
	}
	if err := loadCertificates(mockReader); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(caPEM)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  certpool,
		MinVersion: tls.VersionTLS13,
	}
	srv.StartTLS()
	t.Cleanup(func() { srv.Close() })

	// Send request to server
	resp, err := ai.hi.client.Get(srv.URL)
	if err != nil {
		t.Fatalf("Request to mock server failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Mock server responded with error: %s", string(body))
	}
}

func TestLoadCertificates_InvalidProxy(t *testing.T) {
	origProxy := ai.proxy
	defer func() { ai.proxy = origProxy }()

	ai.proxy = "not-a-proper-url"

	_, mockReader, err := setupCerts("localhost")
	if err != nil {
		t.Fatalf("Could not setup certificates: %s", err.Error())
	}

	errStr := "could not parse proxy url: parse \"not-a-proper-url\": invalid URI for request"

	err = loadCertificates(mockReader)
	if err == nil {
		t.Fatal("Function did not return error")
	}
	if err.Error() != errStr {
		t.Fatalf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestLoadCertificates_MissingFiles(t *testing.T) {
	origProxy := ai.proxy
	defer func() { ai.proxy = origProxy }()

	ai.proxy = "https://github.com"

	_, mockReader, err := setupCerts("github")
	if err != nil {
		t.Fatalf("Could not setup certificates: %s", err.Error())
	}
	delete(mockReader.Files, "github.crt")

	if err = loadCertificates(mockReader); err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
}

func TestLoadCertificates_InvalidCertificates(t *testing.T) {
	origProxy := ai.proxy
	defer func() { ai.proxy = origProxy }()

	ai.proxy = "http://localhost:8080"

	caPEM, mockReader, err := setupCerts("localhost")
	if err != nil {
		t.Fatalf("Could not setup certificates: %s", err.Error())
	}
	mockReader.Files["localhost.crt"] = caPEM

	errStr := "failed to load client x509 key pair for host localhost: tls: private key does not match public key"

	err = loadCertificates(mockReader)
	if err == nil {
		t.Fatal("Function did not return error")
	}
	if err.Error() != errStr {
		t.Fatalf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestMakeRequest(t *testing.T) {
	handleCount := 0
	var tests = []struct {
		testname, errText, method string
		mockHandlerFunc           func(http.ResponseWriter, *http.Request)
		query                     map[string]string
		headers                   map[string]string
		givenBody                 io.Reader
		expectedBody              any
	}{
		{
			testname: "OK_JSON",
			method:   "GET",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				body, err := json.Marshal(VaultHeader{Added: "sometime", Header: "c2VjcmV0IG1lc3NhZ2U=", KeyVersion: 42})
				switch {
				case err != nil:
					http.Error(rw, "Error 404", 404)
				case req.Method == "GET":
					_, _ = rw.Write(body)
				default:
					rw.WriteHeader(http.StatusBadRequest)
				}
			},
			expectedBody: VaultHeader{Added: "sometime", Header: "c2VjcmV0IG1lc3NhZ2U=", KeyVersion: 42},
		},
		{
			testname: "OK_JSON_2",
			method:   "PUT",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if !reflect.DeepEqual(req.Header["Some"], []string{"other thing"}) {
					t.Errorf("Header 'Some' has incorrect value\nExpected=%v\nReceived=%v", []string{"other thing"}, req.Header["Some"])
				}
				if !reflect.DeepEqual(req.Header["Authorization"], []string{"Bearer token"}) {
					t.Errorf("Header 'Authorization' has incorrect value\nExpected=%v\nReceived=%v", []string{"Bearer token"}, req.Header["Authorization"])
				}
				if !reflect.DeepEqual(req.Header["Csc-Password"], []string{"password"}) {
					t.Errorf("Header 'Csc-password' has incorrect value\nExpected=%v\nReceived=%v", []string{"password"}, req.Header["Csc-Password"])
				}
				queries := req.URL.Query()
				value := queries.Get("some")
				if value != "thing" {
					t.Errorf("Query parameter 'some' has incorrect value\nExpected=thing\nReceived=%v", value)
				}
				body, _ := io.ReadAll(req.Body)
				req.Body.Close()
				if string(body) != "secret message" {
					t.Errorf("Request body has incorrect value\nExpected=secret message\nReceived=%s", value)
				}
				body, err := json.Marshal(VaultHeader{Added: "sometime", Header: "c2VjcmV0IG1lc3NhZ2U=", KeyVersion: 42})
				if err != nil {
					http.Error(rw, "Error 404", 404)
				} else {
					_, _ = rw.Write(body)
				}
			},
			query:        map[string]string{"some": "thing"},
			headers:      map[string]string{"some": "other thing"},
			givenBody:    strings.NewReader("secret message"),
			expectedBody: VaultHeader{Added: "sometime", Header: "c2VjcmV0IG1lc3NhZ2U=", KeyVersion: 42},
		},
		{
			testname: "FAIL_JSON",
			method:   "HEAD",
			errText:  "unable to decode response: EOF",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				_, _ = rw.Write([]byte(""))
			},
			expectedBody: profile{Username: "me", ProjectName: "myproject", PI: true, S3Access: false},
		},
		{
			testname: "FAIL_ONCE",
			method:   "POST",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if req.Method == "POST" && handleCount > 0 {
					body, err := json.Marshal(profile{Username: "me", ProjectName: "myproject", PI: true, S3Access: false})
					if err != nil {
						http.Error(rw, "Error 404", 404)
					} else {
						_, _ = rw.Write(body)
					}
				} else {
					handleCount++
					http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
				}
			},
			expectedBody: profile{Username: "me", ProjectName: "myproject", PI: true, S3Access: false},
		},
		{
			testname: "FAIL_ALL",
			method:   "GET",
			errText:  "Get \"https://google.com\": Redirecting failed (as expected)",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if req.Method == "GET" {
					http.Redirect(rw, req, "https://google.com", http.StatusSeeOther)
				} else {
					rw.WriteHeader(http.StatusBadRequest)
				}
			},
		},
		{
			testname: "FAIL_409",
			method:   "PUT",
			errText:  "409 Conflict",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if req.Method == "PUT" {
					rw.WriteHeader(http.StatusConflict)
				} else {
					rw.WriteHeader(http.StatusBadRequest)
				}
			},
		},
		{
			testname: "FAIL_500",
			method:   "POST",
			errText:  "500 something went wrong\n",
			mockHandlerFunc: func(rw http.ResponseWriter, req *http.Request) {
				if req.Method == "POST" {
					http.Error(rw, "something went wrong", 500)
				} else {
					rw.WriteHeader(http.StatusBadRequest)
				}
			},
		},
	}

	origClient := ai.hi.client
	origToken := ai.token
	origPassword := ai.password
	defer func() {
		ai.hi.client = origClient
		ai.token = origToken
		ai.password = origPassword
	}()
	ai.token = "token"
	ai.password = "password"

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tt.mockHandlerFunc))
			ai.hi.client = srv.Client()
			t.Cleanup(func() { srv.Close() })

			// Causes client.Do() to fail when redirecting
			ai.hi.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("Redirecting failed (as expected)")
			}

			var err error
			var ret any
			switch tt.expectedBody.(type) {
			case VaultHeader:
				var objects VaultHeader
				err = makeRequest(tt.method, srv.URL, tt.query, tt.headers, tt.givenBody, &objects)
				ret = objects
			case profile:
				var objects profile
				err = makeRequest(tt.method, srv.URL, tt.query, tt.headers, tt.givenBody, &objects)
				ret = objects
			default:
				err = makeRequest(tt.method, srv.URL, tt.query, tt.headers, tt.givenBody, ret)
			}

			switch {
			case tt.errText != "":
				if err == nil {
					t.Errorf("Function did not return error")
				} else if err.Error() != tt.errText {
					t.Errorf("Function returned incorrect error\nExpected=%q\nReceived=%q", tt.errText, err.Error())
				}
			case err != nil:
				t.Errorf("Function returned unexpected error: %s", err.Error())
			case !reflect.DeepEqual(tt.expectedBody, ret):
				t.Errorf("Incorrect response body\nExpected=%v\nReceived=%v", tt.expectedBody, ret)
			}
		})
	}
}

func TestMakeRequest_PutRequestNil_And_ReadAll_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add("Content-Length", "10")
		rw.WriteHeader(http.StatusNotFound)
	}))
	ai.hi.client = srv.Client()
	ai.proxy = srv.URL

	errStr := "failed to read error response: unexpected EOF"
	err := makeRequest("GET", "/", nil, nil, nil, nil)
	if err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestMakeRequest_NewRequest_Error(t *testing.T) {
	buf := make([]byte, 5)
	buf[0] = 0x7f
	errText := fmt.Sprintf("creating request failed: parse %q: net/url: invalid control character in URL", string(buf))

	if err := makeRequest("GET", string(buf), nil, nil, nil, nil); err == nil {
		t.Error("Function did not return error with invalid URL")
	} else if err.Error() != errText {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
	}
}

func TestDeleteFileFromCache(t *testing.T) {
	nodes := []string{"path", "to", "object"}
	size := int64((1<<25)*3 + 100)

	origCache := downloadCache
	defer func() { downloadCache = origCache }()

	keys := map[string][]byte{
		"SD-Connect/path/to/object_0":         {4, 8, 9},
		"SD-Connect/path/to/object_33554432":  {74, 80, 0},
		"SD-Connect/path/to/object_67108864":  {3, 88, 6},
		"SD-Connect/path/to/object_100663296": {42, 23, 56},
	}

	storage := &mockCache{keys: keys}
	downloadCache = &cache.Ristretto{Cacheable: storage}

	DeleteFileFromCache(SDConnect, nodes, size)

	if len(storage.keys) > 0 {
		missedKeys := make([]string, 0, len(storage.keys))
		for key := range storage.keys {
			missedKeys = append(missedKeys, key)
		}
		t.Fatalf("Function did not delete the entire file from cache, missed %v", missedKeys)
	}
}

func handleConnection(conn net.Conn, expectedData string, errc chan<- error) {
	cmdPrefix := "zINSTREAM\x00"
	defer conn.Close()

	buf := make([]byte, len(cmdPrefix))
	if _, err := io.ReadFull(conn, buf); err != nil {
		errc <- fmt.Errorf("failed reading command: %w", err)

		return
	}
	if string(buf) != cmdPrefix {
		errc <- fmt.Errorf("expected zINSTREAM command, not %s", string(buf))

		return
	}

	var streamData bytes.Buffer

	for {
		var size uint32
		if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				errc <- errors.New("incomplete stream")
			} else {
				errc <- fmt.Errorf("read error: %w", err)
			}

			return
		}

		if size == 0 {
			if strings.Compare(streamData.String(), expectedData) != 0 {
				errc <- fmt.Errorf("server received incorrect data\nExpected=%s\nReceived=%s", expectedData, streamData.String())
			}
			if strings.Contains(streamData.String(), "VIRUS123") {
				_, _ = conn.Write([]byte("Virus FOUND\n"))
			} else {
				_, _ = conn.Write([]byte("stream: OK\n"))
			}

			return
		}

		chunk := make([]byte, size)
		if _, err := io.ReadFull(conn, chunk); err != nil {
			errc <- fmt.Errorf("reading chunk failed: %w", err)

			return
		}

		streamData.Write(chunk)
	}

}

func TestScanForViruses(t *testing.T) {
	tmpdir := t.TempDir()
	socket, err := net.Listen("unix", tmpdir+"/test.sock")
	if err != nil {
		t.Fatalf("Could not create a Unix socket: %s", err.Error())
	}
	defer socket.Close()

	origScan := ai.scan
	origAddress := ai.ui.address
	defer func() {
		ai.scan = origScan
		ai.ui.address = origAddress
	}()

	data := []byte("I am good data")

	errc := make(chan error, 1)
	go func() {
		conn, err := socket.Accept()
		if err != nil {
			t.Errorf("Socket failed to accept a connection: %s", err.Error())

			return
		}

		handleConnection(conn, string(data), errc)
	}()

	testCh := make(chan bool)
	ai.scan = testCh
	ai.ui.address = tmpdir + "/test.sock"

	done := make(chan bool)
	go func() {
		scanForViruses(data, "test.txt")
		done <- true
	}()

	select {
	case v := <-testCh:
		t.Errorf("Scanning channel returned %t", v)
	case <-done:
		break
	}

	select {
	case err := <-errc:
		t.Errorf("Scanning failed: %s", err.Error())
	default:
		break
	}
}

func TestScanForViruses_Virus(t *testing.T) {
	tmpdir := t.TempDir()
	socket, err := net.Listen("unix", tmpdir+"/test.sock")
	if err != nil {
		t.Fatalf("Could not create a Unix socket: %s", err.Error())
	}
	defer socket.Close()

	origScan := ai.scan
	origAddress := ai.ui.address
	defer func() {
		ai.scan = origScan
		ai.ui.address = origAddress
	}()

	data := []byte("I am bad data... VIRUS123")

	errc := make(chan error, 1)
	go func() {
		conn, err := socket.Accept()
		if err != nil {
			t.Errorf("Socket failed to accept a connection: %s", err.Error())

			return
		}

		handleConnection(conn, string(data), errc)
	}()

	testCh := make(chan bool)
	ai.scan = testCh
	ai.ui.address = tmpdir + "/test.sock"

	done := make(chan bool)
	go func() {
		scanForViruses(data, "test.txt")
		done <- true
	}()

	select {
	case v := <-testCh:
		if !v {
			t.Errorf("Scanning channel returned false")
		}
	case <-done:
		t.Errorf("Scanning channel returned nothing")
	}

	select {
	case err := <-errc:
		t.Errorf("Scanning failed: %s", err.Error())
	default:
		break
	}
}

func TestScanForViruses_Closed(t *testing.T) {
	tmpdir := t.TempDir()
	socket, err := net.Listen("unix", tmpdir+"/test.sock")
	if err != nil {
		t.Fatalf("Could not create a Unix socket: %s", err.Error())
	}
	socket.Close()

	origScan := ai.scan
	origAddress := ai.ui.address
	origErrorf := logs.Errorf
	defer func() {
		ai.scan = origScan
		ai.ui.address = origAddress
		logs.Errorf = origErrorf
	}()

	errc := make(chan error, 1)
	logs.Errorf = func(format string, args ...any) {
		errc <- fmt.Errorf(format, args...)
	}

	testCh := make(chan bool)
	ai.scan = testCh
	ai.ui.address = tmpdir + "/test.sock"

	done := make(chan bool)
	go func() {
		data := []byte("I am bad data... VIRUS123")
		scanForViruses(data, "test.txt")
		done <- true
	}()

	select {
	case v := <-testCh:
		if v {
			t.Errorf("Scanning channel returned true")
		}
	case <-done:
		t.Errorf("Scanning channel returned nothing")
	}

	errStr := "failed to connect to clamav socket: dial unix " + tmpdir + "/test.sock" + ": connect: no such file or directory"
	select {
	case err := <-errc:
		if err.Error() != errStr {
			t.Errorf("Function logged incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
		}
	default:
		t.Errorf("Scanning logged no error")

		break
	}
}

type MockConn struct {
	errAt    int
	failRead bool
}

func (mc *MockConn) Read(b []byte) (n int, err error) {
	if mc.failRead {
		return 0, errors.New("mock read failure")
	}
	copy(b, []byte("hello"))

	return 5, io.EOF
}
func (mc *MockConn) Write(b []byte) (n int, err error) {
	if mc.errAt == 0 {
		return 0, errors.New("mock write failure")
	}
	mc.errAt--

	return len(b), nil
}
func (mc *MockConn) Close() error                       { return nil }
func (mc *MockConn) LocalAddr() net.Addr                { return &net.UnixAddr{Name: "local", Net: "unix"} }
func (mc *MockConn) RemoteAddr() net.Addr               { return &net.UnixAddr{Name: "remote", Net: "unix"} }
func (mc *MockConn) SetDeadline(t time.Time) error      { return nil }
func (mc *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (mc *MockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestScanForViruses_Interrupt(t *testing.T) {
	var tests = []struct {
		testname, errStr string
		errAt            int
	}{
		{"FAIL_1", "Failed to send command to ClamAV: mock write failure", 0},
		{"FAIL_2", "Failed to send size to ClamAV: mock write failure", 1},
		{"FAIL_3", "Failed to send file data to ClamAV: mock write failure", 2},
		{"FAIL_4", "Failed to end streaming to ClamAV: mock write failure", 3},
	}

	origScan := ai.scan
	origAddress := ai.ui.address
	origErrorf := logs.Errorf
	origDial := ai.ui.dial
	defer func() {
		ai.scan = origScan
		ai.ui.address = origAddress
		logs.Errorf = origErrorf
		ai.ui.dial = origDial
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			errc := make(chan error, 2)
			logs.Errorf = func(format string, args ...any) {
				errc <- fmt.Errorf(format, args...)
			}
			ai.ui.dial = func() (net.Conn, error) {
				return &MockConn{errAt: tt.errAt}, nil
			}

			testCh := make(chan bool)
			ai.scan = testCh
			ai.ui.address = "test.sock"

			done := make(chan bool)
			go func() {
				data := []byte("I am good data")
				scanForViruses(data, "test.txt")
				done <- true
			}()

			select {
			case v := <-testCh:
				if v {
					t.Errorf("Scanning channel returned true")
				}
			case <-done:
				t.Errorf("Scanning channel returned nothing")
			}

			select {
			case err := <-errc:
				if err.Error() != tt.errStr {
					t.Errorf("Function logged incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
				}
			default:
				t.Errorf("Scanning logged no error")

				break
			}
		})
	}
}

func TestScanForViruses_ReadError(t *testing.T) {
	origScan := ai.scan
	origAddress := ai.ui.address
	origErrorf := logs.Errorf
	origDial := ai.ui.dial
	defer func() {
		ai.scan = origScan
		ai.ui.address = origAddress
		logs.Errorf = origErrorf
		ai.ui.dial = origDial
	}()

	errc := make(chan error, 1)
	logs.Errorf = func(format string, args ...any) {
		errc <- fmt.Errorf(format, args...)
	}
	ai.ui.dial = func() (net.Conn, error) {
		return &MockConn{errAt: -1, failRead: true}, nil
	}

	testCh := make(chan bool)
	ai.scan = testCh
	ai.ui.address = "test.sock"

	done := make(chan bool)
	go func() {
		data := []byte("I am good data")
		scanForViruses(data, "test.txt")
		done <- true
	}()

	select {
	case v := <-testCh:
		if v {
			t.Errorf("Scanning channel returned true")
		}
	case <-done:
		t.Errorf("Scanning channel returned nothing")
	}

	errStr := "Failed to read ClamAV response: mock read failure"
	select {
	case err := <-errc:
		if err.Error() != errStr {
			t.Errorf("Function logged incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
		}
	default:
		t.Errorf("Scanning logged no error")

		break
	}
}

func TestScanForViruses_BadResponse(t *testing.T) {
	origScan := ai.scan
	origAddress := ai.ui.address
	origErrorf := logs.Errorf
	origDial := ai.ui.dial
	defer func() {
		ai.scan = origScan
		ai.ui.address = origAddress
		logs.Errorf = origErrorf
		ai.ui.dial = origDial
	}()

	errc := make(chan error, 1)
	logs.Errorf = func(format string, args ...any) {
		errc <- fmt.Errorf(format, args...)
	}
	ai.ui.dial = func() (net.Conn, error) {
		return &MockConn{errAt: -1, failRead: false}, nil
	}

	testCh := make(chan bool)
	ai.scan = testCh
	ai.ui.address = "test.sock"

	done := make(chan bool)
	go func() {
		data := []byte("I am good data")
		scanForViruses(data, "test.txt")
		done <- true
	}()

	select {
	case v := <-testCh:
		if v {
			t.Errorf("Scanning channel returned true")
		}
	case <-done:
		t.Errorf("Scanning channel returned nothing")
	}

	errStr := "ClamAV did not return a valid response for test.txt: hello"
	select {
	case err := <-errc:
		if err.Error() != errStr {
			t.Errorf("Function logged incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
		}
	default:
		t.Errorf("Scanning logged no error")

		break
	}
}
