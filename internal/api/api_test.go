package api

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
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
	data []byte
	key  string
}

func (c *mockCache) Get(key string) ([]byte, bool) {
	if c.key == key && c.data != nil {
		return c.data, true
	}

	return nil, false
}

func (c *mockCache) Set(key string, value []byte, _ int64, _ time.Duration) bool {
	c.key = key
	c.data = value

	return true
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
	origNewRistretto := cache.NewRistrettoCache
	origLoadCertificates := loadCertificates
	origInitialiseS3Client := initialiseS3Client
	origProxy := ai.proxy
	origToken := ai.token
	defer func() {
		GetEnv = origGetEnv
		cache.NewRistrettoCache = origNewRistretto
		loadCertificates = origLoadCertificates
		initialiseS3Client = origInitialiseS3Client
		ai.proxy = origProxy
		ai.token = origToken
	}()

	GetEnv = func(name string, verifyURL bool) (string, error) {
		if name == "PROXY_URL" {
			return "test_url", nil
		} else if name == "SDS_ACCESS_TOKEN" {
			return "test_token", nil
		} else {
			return "", fmt.Errorf("unknown env %s", name)
		}
	}
	newCache := &cache.Ristretto{Cacheable: &mockCache{}}
	cache.NewRistrettoCache = func() (*cache.Ristretto, error) {
		return newCache, nil
	}
	mockFiles := embed.FS{}
	mockCerts := []tls.Certificate{{Certificate: [][]byte{[]byte("fake-cert")}}}
	loadCertificates = func(certFiles []FileReader) ([]tls.Certificate, error) {
		if !reflect.DeepEqual(certFiles[0], mockFiles) {
			return nil, fmt.Errorf("loadCertificates() received invalid argument")
		}

		return mockCerts, nil
	}
	initialiseS3Client = func(certs []tls.Certificate) error {
		if !reflect.DeepEqual(certs, mockCerts) {
			return fmt.Errorf("initialiseS3Client() received incorrect certificates")
		}

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
	if downloadCache != newCache {
		t.Errorf("downloadCache does not point to new cache")
	}
	publicKey := keys.DerivePublicKey(ai.vi.privateKey)
	publicKey64 := base64.StdEncoding.EncodeToString(publicKey[:])
	if publicKey64 != ai.vi.publicKey {
		t.Errorf("Public key has incorrect value\nExpected=%v\nReceived=%v", publicKey, ai.vi.publicKey)
	}
}

func TestSetup_Error(t *testing.T) {
	var tests = []struct {
		testname, errText                            string
		proxyErr, tokenErr, cacheErr, certErr, s3Err error
	}{
		{"FAIL_1", "required environment variables missing", errExpected, nil, nil, nil, nil},
		{"FAIL_2", "required environment variables missing", nil, errExpected, nil, nil, nil},
		{"FAIL_3", "failed to create cache", nil, nil, errExpected, nil, nil},
		{"FAIL_4", "failed to load certficates", nil, nil, nil, errExpected, nil},
		{"FAIL_5", "failed to initialise S3 client", nil, nil, nil, nil, errExpected},
	}

	origGetEnv := GetEnv
	origNewRistretto := cache.NewRistrettoCache
	origLoadCertificates := loadCertificates
	origInitialiseS3Client := initialiseS3Client
	origProxy := ai.proxy
	origToken := ai.token
	defer func() {
		GetEnv = origGetEnv
		cache.NewRistrettoCache = origNewRistretto
		loadCertificates = origLoadCertificates
		initialiseS3Client = origInitialiseS3Client
		ai.proxy = origProxy
		ai.token = origToken
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			GetEnv = func(name string, verifyURL bool) (string, error) {
				if name == "PROXY_URL" {
					return "", tt.proxyErr
				} else if name == "SDS_ACCESS_TOKEN" {
					return "", tt.tokenErr
				} else {
					return "", fmt.Errorf("unknown env %s", name)
				}
			}
			cache.NewRistrettoCache = func() (*cache.Ristretto, error) {
				return nil, tt.cacheErr
			}
			loadCertificates = func(certFiles []FileReader) ([]tls.Certificate, error) {
				return nil, tt.certErr
			}
			initialiseS3Client = func(certs []tls.Certificate) error {
				return tt.s3Err
			}

			err := Setup()
			errText := tt.errText + ": " + errExpected.Error()
			if err.Error() != errText {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
			}
		})
	}
}

func TestGetProfile(t *testing.T) {
	origMakeRequest := MakeRequest
	origRepositories := ai.repositories
	defer func() {
		ai.repositories = origRepositories
		MakeRequest = origMakeRequest
	}()

	access := true
	MakeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if path != "/profile" {
			return fmt.Errorf("Incorrect path\nExpected=/profile\nReceived=%s", path)
		}

		switch v := ret.(type) {
		case *profile:
			v.DesktopToken = "test_token"
			v.S3Access = access
			v.SDConnect = true
			access = false

			return nil
		default:
			return fmt.Errorf("ret has incorrect type %v, expected profile", reflect.TypeOf(v))
		}
	}
	ai.repositories = []string{"mock-repo"}
	finalRepositories := []string{"mock-repo", SDConnect}

	if access, err := GetProfile(); err != nil {
		t.Errorf("First call returned error: %s", err.Error())
	} else if !access {
		t.Errorf("User should have access")
	} else if ai.token != "test_token" {
		t.Errorf("Incorrect token\nExpected=test_token\nReceived=%s", ai.token)
	} else if !reflect.DeepEqual(ai.repositories, finalRepositories) {
		t.Errorf("Incorrect repositories\nExpected=%v\nReceived=%v", finalRepositories, ai.repositories)
	}

	if access, err := GetProfile(); err != nil {
		t.Errorf("Second call returned error: %s", err.Error())
	} else if access {
		t.Errorf("User should not have access")
	} else if ai.token != "test_token" {
		t.Errorf("Incorrect token\nExpected=test_token\nReceived=%s", ai.token)
	} else if !reflect.DeepEqual(ai.repositories, finalRepositories) {
		t.Errorf("Incorrect repositories\nExpected=%v\nReceived=%v", finalRepositories, ai.repositories)
	}
}

func TestGetProfile_Error(t *testing.T) {
	origMakeRequest := MakeRequest
	origRepositories := ai.repositories
	defer func() {
		ai.repositories = origRepositories
		MakeRequest = origMakeRequest
	}()

	MakeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		return errExpected
	}
	errText := "failed to get user profile: " + errExpected.Error()

	if _, err := GetProfile(); err == nil {
		t.Error("Function should have returned error")
	} else if err.Error() != errText {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
	}
}

func TestAuthenticate(t *testing.T) {
	origMakeRequest := MakeRequest
	origPassword := ai.password
	defer func() {
		ai.password = origPassword
		MakeRequest = origMakeRequest
	}()

	password := "passw0rd"

	MakeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
		if path != "/credentials/check" {
			return fmt.Errorf("Incorrect path\nExpected=/credentials/check\nReceived=%s", path)
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

	origMakeRequest := MakeRequest
	origPassword := ai.password
	defer func() {
		ai.password = origPassword
		MakeRequest = origMakeRequest
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			MakeRequest = func(method, path string, query, headers map[string]string, reqBody io.Reader, ret any) error {
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
				InsecureSkipVerify: true,
			},
		},
	}

	caPEM, mockReader, err := setupCerts("localhost")
	if err != nil {
		t.Fatalf("Could not setup certificates: %s", err.Error())
	}
	expectedCert, _ := pem.Decode(mockReader.Files["localhost.crt"])

	certs, err := loadCertificates([]FileReader{mockReader})
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if len(certs) != 1 {
		t.Fatalf("Function returned a list of certificates with %d entries, expected 1", len(certs))
	}
	if !bytes.Equal(certs[0].Leaf.Raw, expectedCert.Bytes) {
		t.Errorf("Function returned incorrect certificate\nExpected=%d\nReceived=%d", expectedCert.Bytes, certs[0].Leaf.Raw)
	}

	// Create handler that manually checks client certificate
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "no client certificate", http.StatusBadRequest)
			return
		}
		clientCert := r.TLS.PeerCertificates[0]
		if !bytes.Equal(clientCert.Raw, expectedCert.Bytes) {
			http.Error(w, "invalid client certificate", http.StatusUnauthorized)
			return
		}
	})

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(caPEM)

	srv := httptest.NewUnstartedServer(handler)
	srv.TLS = &tls.Config{
		ClientAuth: tls.RequireAnyClientCert,
		RootCAs:    certpool,
	}
	srv.StartTLS()
	defer srv.Close()

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

func TestLoadCertificates_NoFileReader(t *testing.T) {
	certs, err := loadCertificates(nil)
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if certs != nil {
		t.Errorf("Function should have returned a nil slice of certificates")
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

	_, err = loadCertificates([]FileReader{mockReader})
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

	certs, err := loadCertificates([]FileReader{mockReader})
	if err != nil {
		t.Fatalf("Function returned unexpected error: %s", err.Error())
	}
	if certs != nil {
		t.Errorf("Function should have returned a nil slice of certificates")
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

	_, err = loadCertificates([]FileReader{mockReader})
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
				if err != nil {
					http.Error(rw, "Error 404", 404)
				} else if req.Method == "GET" {
					_, _ = rw.Write(body)
				} else {
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
				defer req.Body.Close()
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
			errText:  "unable to decode response: unexpected end of JSON input",
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
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandlerFunc))
			ai.hi.client = server.Client()

			// Causes client.Do() to fail when redirecting
			ai.hi.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("Redirecting failed (as expected)")
			}

			var err error
			var ret any
			switch tt.expectedBody.(type) {
			case VaultHeader:
				var objects VaultHeader
				err = MakeRequest(tt.method, server.URL, tt.query, tt.headers, tt.givenBody, &objects)
				ret = objects
			case profile:
				var objects profile
				err = MakeRequest(tt.method, server.URL, tt.query, tt.headers, tt.givenBody, &objects)
				ret = objects
			default:
				err = MakeRequest(tt.method, server.URL, tt.query, tt.headers, tt.givenBody, ret)
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

			server.Close()
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

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add("Content-Length", "10")
		rw.WriteHeader(http.StatusCreated)
	}))
	ai.hi.client = server.Client()
	ai.proxy = server.URL

	errStr := "failed to read response: unexpected EOF"
	err := MakeRequest("GET", "/", nil, nil, nil, nil)
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

	if err := MakeRequest("GET", string(buf), nil, nil, nil, nil); err == nil {
		t.Error("Function did not return error with invalid URL")
	} else if err.Error() != errText {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errText, err.Error())
	}
}

type mockStorage struct {
	cache.Cacheable

	keys map[string]bool
}

func (ms *mockStorage) Del(key string) {
	delete(ms.keys, key)
}

func TestDeleteFileFromCache(t *testing.T) {
	nodes := []string{"path", "to", "object"}
	size := int64((1<<25)*3 + 100)

	origCache := downloadCache
	defer func() { downloadCache = origCache }()

	keys := map[string]bool{
		"path/to/object_0":         true,
		"path/to/object_33554432":  true,
		"path/to/object_67108864":  true,
		"path/to/object_100663296": true,
	}

	storage := &mockStorage{keys: keys}
	downloadCache = &cache.Ristretto{Cacheable: storage}

	DeleteFileFromCache(nodes, size)

	if len(storage.keys) > 0 {
		missedKeys := make([]string, 0, len(storage.keys))
		for key := range storage.keys {
			missedKeys = append(missedKeys, key)
		}
		t.Fatalf("Function did not delete the entire file from cache, missed %v", missedKeys)
	}
}
