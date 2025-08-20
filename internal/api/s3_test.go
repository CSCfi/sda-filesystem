package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sda-filesystem/internal/cache"
	"sda-filesystem/test"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func TestBucketExists(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	origProfile := ai.userProfile
	origPassword := ai.password
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
		ai.userProfile = origProfile
		ai.password = origPassword
	}()

	ai.proxy = "http://localhost:8080"
	ai.hi.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // #nosec G402
			},
		},
	}
	ai.hi.endpoints = testConfig
	ai.userProfile.DesktopToken = "desktop-token"
	ai.password = "my-secret-password"

	// This tests is also used to test certificates in S3 client
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
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "GET" {
			t.Errorf("Request has incorrect method\nExpected=GET\nReceived=%v", r.Method)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer desktop-token" {
			t.Errorf("Header parameter 'Authorization' has incorrect value\nExpected=Bearer desktop-token\nReceived=%s", auth)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		pass := r.Header.Get("CSC-Password")
		if pass != "my-secret-password" {
			t.Errorf("Header parameter 'CSC-Passwors' has incorrect value\nExpected=my-secret-password\nReceived=%s", pass)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if r.URL.Path == "/s3-head-endpoint/sd-connect/my-bucket" {
			return
		}
		if r.URL.Path == "/s3-head-endpoint/sd-apply/my-bucket-2" {
			w.WriteHeader(http.StatusNotFound)

			return

		}

		t.Errorf("Request has incorrect path %v", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
	}))
	srv.TLS = &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  certpool,
		MinVersion: tls.VersionTLS13,
	}
	srv.StartTLS()
	t.Cleanup(func() { srv.Close() })

	ai.proxy = srv.URL

	if err := initialiseS3Client(); err != nil {
		t.Fatalf("Failed to initialize S3 client: %v", err.Error())
	}

	exists, err := BucketExists(SDConnect, "my-bucket")
	if err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if !exists {
		t.Errorf("Function says bucket 'my-bucket' does not exist")
	}

	exists, err = BucketExists(SDApply, "my-bucket-2")
	if err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if exists {
		t.Errorf("Function says bucket 'my-bucket-2' does exist")
	}
}

func TestBucketExists_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}

	var tests = []struct {
		testname, errStr string
		code             int
	}{
		{
			"FAIL_1",
			"bucket my-bucket is already in use by another project",
			http.StatusForbidden,
		},
		{
			"FAIL_2",
			"could not determine if bucket my-bucket exists: api error InternalServerError: Internal Server Error",
			http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
			}))
			ai.proxy = srv.URL
			t.Cleanup(func() { srv.Close() })

			if err := initialiseS3Client(); err != nil {
				t.Errorf("Failed to initialize S3 client: %v", err.Error())
			} else if _, err := BucketExists(SDConnect, "my-bucket"); err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestResolveEndpoint_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Fatalf("Failed to initialize S3 client: %v", err.Error())
	}

	errStr := "endpoint context not valid"
	_, err := ai.hi.s3Client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String("bucket"),
	})
	if err == nil {
		t.Fatalf("Function did not return error")
	}
	var oe *smithy.OperationError
	if errors.As(err, &oe) {
		err = oe.Err
		for errors.Unwrap(err) != nil {
			err = errors.Unwrap(err)
		}
	} else {
		t.Fatal("Could not convert error to smithy.OperationError")
	}
	if err.Error() != "endpoint context not valid" {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestCreateBucket(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	origProfile := ai.userProfile
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
		ai.userProfile = origProfile
	}()

	created := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method == "PUT" && r.URL.Path == "/s3-default-endpoint/my-repo/my-bucket" {
			created = true

			return
		}
		if r.Method == "GET" && r.URL.Path == "/s3-head-endpoint/my-repo/my-bucket" {
			if created {
				return
			}
			t.Errorf("Trying to check that bucket exists before bucket was created")
			w.WriteHeader(http.StatusNotFound)

			return
		}

		t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if err := CreateBucket("MY-REPO", "my-bucket"); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	}
}

func TestCreateBucket_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, errStr        string
		createdCode, existsCode int
	}{
		{
			"FAIL_1",
			"could not create bucket my-bucket: api error MethodNotAllowed: Method Not Allowed",
			http.StatusMethodNotAllowed, http.StatusOK,
		},
		{
			"FAIL_2",
			"failed to wait for bucket my-bucket to exist: api error Forbidden: Forbidden",
			http.StatusOK, http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				r.Body.Close()

				if r.Method == "PUT" && r.URL.Path == "/s3-default-endpoint/repository/my-bucket" {
					w.WriteHeader(tt.createdCode)

					return
				}
				if r.Method == "GET" && r.URL.Path == "/s3-head-endpoint/repository/my-bucket" {
					w.WriteHeader(tt.existsCode)

					return
				}

				t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}))
			ai.proxy = srv.URL
			t.Cleanup(func() { srv.Close() })

			if err := initialiseS3Client(); err != nil {
				t.Errorf("Failed to initialize S3 client: %v", err.Error())
			} else if err := CreateBucket("Repository", "my-bucket"); err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestGetBuckets(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "GET" || r.URL.Path != "/s3-default-endpoint/my-repo" {
			t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult>
   <Buckets>
      <Bucket>
         <BucketRegion>us-east-1</BucketRegion>
         <CreationDate>2023-11-10T13:39:02.211Z</CreationDate>
         <Name>bucket42</Name>
      </Bucket>
	  <Bucket>
         <BucketRegion>us-east-1</BucketRegion>
         <CreationDate>2023-10-25T08:38:55.107Z</CreationDate>
         <Name>bucket256</Name>
      </Bucket>
   </Buckets>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListAllMyBucketsResult>`

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(xmlData))
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	expectedBuckets := []Metadata{
		{Name: "bucket42", Size: 0, LastModified: nil},
		{Name: "bucket256", Size: 0, LastModified: nil},
	}
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if buckets, err := GetBuckets("My-Repo"); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if !reflect.DeepEqual(buckets, expectedBuckets) {
		t.Errorf("Function returned incorrect buckets\nExpected=%v\nReceived=%v", expectedBuckets, buckets)
	}
}

func TestGetBuckets_MultiplePages(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	token := "i-am-a-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "GET" || r.URL.Path != "/s3-default-endpoint/my-repo" {
			t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		w.Header().Set("Content-Type", "application/xml")

		var xmlData string
		if token == "" {
			xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult>
   <Buckets>
      <Bucket>
         <BucketRegion>us-east-1</BucketRegion>
         <CreationDate>2023-11-10T13:39:02.211Z</CreationDate>
         <Name>bucket2000</Name>
      </Bucket>
	  <Bucket>
         <BucketRegion>us-east-1</BucketRegion>
         <CreationDate>2023-10-25T08:38:55.107Z</CreationDate>
         <Name>bucket1234</Name>
      </Bucket>
   </Buckets>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListAllMyBucketsResult>`
			_, _ = w.Write([]byte(xmlData))
		} else {
			xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult>
   <Buckets>
      <Bucket>
         <BucketRegion>us-east-1</BucketRegion>
         <CreationDate>2023-11-10T13:39:02.211Z</CreationDate>
         <Name>bucket42</Name>
      </Bucket>
	  <Bucket>
         <BucketRegion>us-east-1</BucketRegion>
         <CreationDate>2023-10-25T08:38:55.107Z</CreationDate>
         <Name>bucket256</Name>
      </Bucket>
   </Buckets>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
   <ContinuationToken>%s</ContinuationToken>
</ListAllMyBucketsResult>`
			fmt.Fprintf(w, xmlData, token)
		}

		token = ""
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	expectedBuckets := []Metadata{
		{Name: "bucket42", Size: 0, LastModified: nil},
		{Name: "bucket256", Size: 0, LastModified: nil},
		{Name: "bucket2000", Size: 0, LastModified: nil},
		{Name: "bucket1234", Size: 0, LastModified: nil},
	}
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if buckets, err := GetBuckets("My-Repo"); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if !reflect.DeepEqual(buckets, expectedBuckets) {
		t.Errorf("Function returned incorrect buckets\nExpected=%v\nReceived=%v", expectedBuckets, buckets)
	}
}

func TestGetBuckets_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, errStr string
		code             int
	}{
		{
			"FAIL_1",
			"no buckets found for SD Apply",
			http.StatusOK,
		},
		{
			"FAIL_2",
			"failed to list buckets for SD Apply: api error Forbidden: Forbidden",
			http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				r.Body.Close()

				if r.Method == "GET" && r.URL.Path == "/s3-default-endpoint/sd-apply" {
					w.WriteHeader(tt.code)

					return
				}

				t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}))
			ai.proxy = srv.URL
			t.Cleanup(func() { srv.Close() })

			if err := initialiseS3Client(); err != nil {
				t.Errorf("Failed to initialize S3 client: %v", err.Error())
			} else if _, err := GetBuckets(SDApply); err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestGetObjects(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "GET" || r.URL.Path != "/s3-default-endpoint/sd-apply/bucket234" {
			t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		prefix := r.URL.Query().Get("prefix")
		if prefix != "prefix" {
			t.Errorf("Server was called with incorrect prefix. Expected=prefix, received=%s", prefix)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
   <IsTruncated>false</IsTruncated>
   <Contents>
      <Key>object456</Key>
      <LastModified>2009-10-12T17:50:30.000Z</LastModified>
      <Size>63965</Size>
   </Contents>
   <Contents>
      <Key>object0890</Key>
      <LastModified>2024-06-23T10:55:00.000Z</LastModified>
      <Size>67</Size>
   </Contents>
   <Name>bucket234</Name>
   <KeyCount>2</KeyCount>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListBucketResult>`

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(xmlData))
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	time1, _ := time.Parse(time.RFC3339, "2009-10-12T17:50:30.000Z")
	time2, _ := time.Parse(time.RFC3339, "2024-06-23T10:55:00.000Z")
	expectedObjects := []Metadata{
		{Name: "object456", Size: 63965, LastModified: &time1},
		{Name: "object0890", Size: 67, LastModified: &time2},
	}
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if objects, err := GetObjects(SDApply, "bucket234", "", "prefix"); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if !reflect.DeepEqual(objects, expectedObjects) {
		t.Errorf("Function returned incorrect objects\nExpected=%v\nReceived=%v", expectedObjects, objects)
	}
}

func TestGetObjects_MultiplePages(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	token := "i-am-a-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "GET" || r.URL.Path != "/s3-default-endpoint/sd-connect/bucket23" {
			t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		var xmlData string
		if token == "" {
			xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
   <IsTruncated>false</IsTruncated>
   <Contents>
      <Key>object456</Key>
      <LastModified>2009-10-12T17:50:30.000Z</LastModified>
      <Size>63965</Size>
   </Contents>
   <Contents>
      <Key>object0890</Key>
      <LastModified>2024-06-23T10:55:00.000Z</LastModified>
      <Size>67</Size>
   </Contents>
   <Name>bucket234</Name>
   <KeyCount>2</KeyCount>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListBucketResult>`
			_, _ = w.Write([]byte(xmlData))
		} else {
			xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
   <IsTruncated>true</IsTruncated>
   <NextContinuationToken>%s</NextContinuationToken>
   <Contents>
      <Key>object7485</Key>
      <LastModified>2000-11-09T17:50:30.000Z</LastModified>
      <Size>0</Size>
   </Contents>
   <Contents>
      <Key>object1111</Key>
      <LastModified>2025-01-01T10:24:55.000Z</LastModified>
      <Size>5280009</Size>
   </Contents>
   <Name>bucket23</Name>
   <KeyCount>2</KeyCount>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListBucketResult>`
			fmt.Fprintf(w, xmlData, token)
		}

		token = ""
		w.Header().Set("Content-Type", "application/xml")
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	time1, _ := time.Parse(time.RFC3339, "2000-11-09T17:50:30.000Z")
	time2, _ := time.Parse(time.RFC3339, "2025-01-01T10:24:55.000Z")
	time3, _ := time.Parse(time.RFC3339, "2009-10-12T17:50:30.000Z")
	time4, _ := time.Parse(time.RFC3339, "2024-06-23T10:55:00.000Z")
	expectedObjects := []Metadata{
		{Name: "object7485", Size: 0, LastModified: &time1},
		{Name: "object1111", Size: 5280009, LastModified: &time2},
		{Name: "object456", Size: 63965, LastModified: &time3},
		{Name: "object0890", Size: 67, LastModified: &time4},
	}
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if objects, err := GetObjects(SDConnect, "bucket23", ""); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if !reflect.DeepEqual(objects, expectedObjects) {
		t.Errorf("Function returned incorrect objects\nExpected=%v\nReceived=%v", expectedObjects, objects)
	}
}

func TestGetObjects_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, errStr string
		code             int
	}{
		{
			"FAIL_1",
			"failed to list objects for SD-Apply/some-bucket: bad request: bucket name \"some-bucket\" may not be S3 compatible",
			http.StatusBadRequest,
		},
		{
			"FAIL_2",
			"failed to list objects for SD-Apply/some-bucket: api error Unauthorized: Unauthorized",
			http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				r.Body.Close()

				if r.Method == "GET" && r.URL.Path == "/s3-default-endpoint/sd-apply/some-bucket" {
					w.WriteHeader(tt.code)

					return
				}

				t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}))
			ai.proxy = srv.URL
			t.Cleanup(func() { srv.Close() })

			if err := initialiseS3Client(); err != nil {
				t.Errorf("Failed to initialize S3 client: %v", err.Error())
			} else if _, err := GetObjects(SDApply, "some-bucket", "SD-Apply/some-bucket"); err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestGetSegmentedObjects(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "GET" || r.URL.Path != "/s3-default-endpoint/sd-apply/bucket234" {
			t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
   <IsTruncated>false</IsTruncated>
   <Contents>
      <Key>object456</Key>
      <LastModified>2009-10-12T17:50:30.000Z</LastModified>
      <Size>63965</Size>
   </Contents>
   <Contents>
      <Key>object0890</Key>
      <LastModified>2024-06-23T10:55:00.000Z</LastModified>
      <Size>67</Size>
   </Contents>
   <Name>bucket234</Name>
   <KeyCount>2</KeyCount>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListBucketResult>`

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(xmlData))
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	time1, _ := time.Parse(time.RFC3339, "2009-10-12T17:50:30.000Z")
	time2, _ := time.Parse(time.RFC3339, "2024-06-23T10:55:00.000Z")
	expectedObjects := []Metadata{
		{Name: "object456", Size: 63965, LastModified: &time1},
		{Name: "object0890", Size: 67, LastModified: &time2},
	}
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if objects, err := GetSegmentedObjects(SDApply, "bucket234"); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if !reflect.DeepEqual(objects, expectedObjects) {
		t.Errorf("Function returned incorrect objects\nExpected=%v\nReceived=%v", expectedObjects, objects)
	}
}

func TestGetSegmentedObjects_MultiplePages(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	token := "i-am-a-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "GET" || r.URL.Path != "/s3-default-endpoint/sd-connect/bucket23" {
			t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		var xmlData string
		if token == "" {
			xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
   <IsTruncated>false</IsTruncated>
   <Contents>
      <Key>object456</Key>
      <LastModified>2009-10-12T17:50:30.000Z</LastModified>
      <Size>63965</Size>
   </Contents>
   <Contents>
      <Key>object0890</Key>
      <LastModified>2024-06-23T10:55:00.000Z</LastModified>
      <Size>67</Size>
   </Contents>
   <Name>bucket234</Name>
   <KeyCount>2</KeyCount>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListBucketResult>`
			_, _ = w.Write([]byte(xmlData))
		} else {
			xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
   <IsTruncated>true</IsTruncated>
   <NextContinuationToken>%s</NextContinuationToken>
   <Contents>
      <Key>object7485</Key>
      <LastModified>2000-11-09T17:50:30.000Z</LastModified>
      <Size>0</Size>
   </Contents>
   <Contents>
      <Key>object1111</Key>
      <LastModified>2025-01-01T10:24:55.000Z</LastModified>
      <Size>5280009</Size>
   </Contents>
   <Name>bucket23</Name>
   <KeyCount>2</KeyCount>
   <Owner>
      <DisplayName>project_2001234</DisplayName>
      <ID>fir67cor68oct79vptr6i7rc</ID>
   </Owner>
</ListBucketResult>`
			fmt.Fprintf(w, xmlData, token)
		}

		token = ""
		w.Header().Set("Content-Type", "application/xml")
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	time1, _ := time.Parse(time.RFC3339, "2000-11-09T17:50:30.000Z")
	time2, _ := time.Parse(time.RFC3339, "2025-01-01T10:24:55.000Z")
	time3, _ := time.Parse(time.RFC3339, "2009-10-12T17:50:30.000Z")
	time4, _ := time.Parse(time.RFC3339, "2024-06-23T10:55:00.000Z")
	expectedObjects := []Metadata{
		{Name: "object7485", Size: 0, LastModified: &time1},
		{Name: "object1111", Size: 5280009, LastModified: &time2},
		{Name: "object456", Size: 63965, LastModified: &time3},
		{Name: "object0890", Size: 67, LastModified: &time4},
	}
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if objects, err := GetSegmentedObjects(SDConnect, "bucket23"); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if !reflect.DeepEqual(objects, expectedObjects) {
		t.Errorf("Function returned incorrect objects\nExpected=%v\nReceived=%v", expectedObjects, objects)
	}
}

func TestGetSefmentedObjects_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, errStr string
		code             int
	}{
		{
			"FAIL_1",
			"failed to list objects for container some-bucket in SD Apply: bad request: bucket name \"some-bucket\" may not be S3 compatible",
			http.StatusBadRequest,
		},
		{
			"FAIL_2",
			"failed to list objects for container some-bucket in SD Apply: api error Unauthorized: Unauthorized",
			http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				r.Body.Close()

				if r.Method == "GET" && r.URL.Path == "/s3-default-endpoint/sd-apply/some-bucket" {
					w.WriteHeader(tt.code)

					return
				}

				t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}))
			ai.proxy = srv.URL
			t.Cleanup(func() { srv.Close() })

			if err := initialiseS3Client(); err != nil {
				t.Errorf("Failed to initialize S3 client: %v", err.Error())
			} else if _, err := GetSegmentedObjects(SDApply, "some-bucket"); err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestDownloadData(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	origPrivateKey := ai.vi.privateKey
	origCache := downloadCache
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
		ai.vi.privateKey = origPrivateKey
		downloadCache = origCache
	}()

	storage := &mockCache{}
	downloadCache = &cache.Ristretto{Cacheable: storage}

	decryptedSize := 80*1024*1024 + 10
	encryptedSize := decryptedSize + 35992
	content := test.GenerateRandomText(decryptedSize) // 3 chunks

	headerBytes, encryptedContent, privateKey := test.EncryptData(t, content)
	if len(headerBytes)+len(encryptedContent) != encryptedSize {
		t.Fatalf("Failed to split data correctly. Split data has header size %d and body size %d. They should sum up to %d", len(headerBytes), len(encryptedContent), encryptedSize)
	}
	header64 := base64.StdEncoding.EncodeToString(headerBytes)

	ai.vi.privateKey = privateKey
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.hi.endpoints = testConfig

	var tests = []struct {
		testname, bucket, object      string
		header                        []string
		byteStart, byteEnd, offset    int64
		alreadyCachedIdxs, cachedIdxs []int64
		decryptable                   bool
	}{
		{
			"OK_NIL_HEADER", "new-bucket", "new-object.txt.c4gh",
			nil, 345, 1000, 0,
			nil, nil, false,
		},
		{
			"OK_DECRYPTED", "another-bucket", "subfolder/another-object.txt.c4gh",
			[]string{header64}, 345, 1000, 0,
			nil, []int64{0}, true,
		},
		{
			"OK_ENTIRE_CHUNK", "myBucket", "subfolder/another-dir/hello.txt.c4gh",
			[]string{header64}, 33554432, 67108864, 0,
			nil, []int64{33554432}, true,
		},
		{
			"OK_TWO_CHUNKS_1", "bucket24601", "file.txt.c4gh",
			[]string{header64}, 33554000, 33564437, 0,
			nil, []int64{0, 33554432}, true,
		},
		{
			"OK_TWO_CHUNKS_2", "bucket24601", "file.txt.c4gh",
			[]string{header64}, 67100860, 67109862, 0,
			nil, []int64{33554432, 67108864}, true,
		},
		{
			"OK_FILE_START", "bucket42", "what-are-you-looking-at.c4gh",
			[]string{header64}, 0, 2341, 0,
			nil, []int64{0}, true,
		},
		{
			"OK_FILE_END", "buketti", "banana.pdf.c4gh",
			[]string{header64}, 83885978, 83886090, 0,
			nil, []int64{67108864}, true,
		},
		{
			"OK_CANNOT_DECRYPT", "buketti", "omena.docx.c4gh",
			[]string{""}, 6486076, 6487002, 0,
			nil, []int64{0}, false,
		},
		{
			"OK_CACHED_1", "cached-bucket", "file.txt",
			[]string{""}, 74106760, 74107964, 0,
			[]int64{67108864}, []int64{67108864}, false,
		},
		{
			"OK_CACHED_2", "cached-bucket", "tiedosto.c4gh",
			[]string{header64}, 33550099, 33556008, 0,
			[]int64{0, 33554432}, []int64{0, 33554432}, true,
		},
		{
			"OK_PARTLY_CACHED", "cached-bucket", "tiedosto-2.c4gh",
			[]string{header64}, 33550099, 33556008, 0,
			[]int64{0}, []int64{0, 33554432}, true,
		},
		{
			"OK_OFFSET_1", "old-bucket", "myobject.c4gh",
			[]string{header64}, 0, 426, 28357,
			nil, []int64{0}, true,
		},
		{
			"OK_OFFSET_2", "old-bucket-2", "myobject-2.c4gh",
			[]string{header64}, 53108864, 53109961, 157,
			nil, []int64{33554432}, true,
		},
		{
			"OK_OFFSET_3", "old-bucket-3", "myobject-3.c4gh",
			[]string{header64}, 83885978, 83886090, 473,
			nil, []int64{67108864}, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				r.Body.Close()

				if r.Method != "GET" || r.URL.Path != "/s3-default-endpoint/sd-connect/"+tt.bucket+"/" {
					t.Errorf("Server was called with unexpected method %s or path %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusBadRequest)

					return
				}
				object := r.URL.Query().Get("object")
				if object != tt.object {
					t.Errorf("Query parameter 'object' has incorrect value\nExpected=%s\nReceived=%s", tt.object, object)
					w.WriteHeader(http.StatusBadRequest)

					return
				}

				var byteStart, byteEnd int
				byteRange := r.Header["Range"]
				_, err := fmt.Sscanf(byteRange[0], "bytes=%d-%d", &byteStart, &byteEnd)
				if err != nil {
					t.Errorf("Header Range (value: %v) was in the incorrect format: %s", byteRange, err.Error())
				}

				for i := range tt.alreadyCachedIdxs {
					key := fmt.Sprintf("/SD-Connect/project/%s/%s_%d", tt.bucket, tt.object, tt.alreadyCachedIdxs[i])

					if byteStart/chunkSize == i {
						t.Errorf("Should not have called server since key %s should have been in cache", key)
					}
				}

				_, _ = w.Write(encryptedContent[byteStart : byteEnd+1])
			}))
			ai.proxy = srv.URL
			t.Cleanup(func() { srv.Close() })

			if err := initialiseS3Client(); err != nil {
				t.Fatalf("Failed to initialize S3 client: %v", err.Error())
			}

			nodes := []string{"", SDConnect, "project", tt.bucket}
			nodes = append(nodes, strings.Split(tt.object, "/")...)
			var header *string = nil
			if tt.header != nil {
				header = &tt.header[0]
			}
			// add mock offset to encrypted data
			encryptedContent = append([]byte(strings.Repeat("A", int(tt.offset))), encryptedContent...)
			source := content
			if !tt.decryptable {
				source = encryptedContent
			}
			t.Cleanup(func() { encryptedContent = encryptedContent[tt.offset:] })

			storage.keys = make(map[string][]byte)
			for _, idx := range tt.alreadyCachedIdxs {
				key := strings.Join(nodes, "/") + "_" + fmt.Sprintf("%d", idx)
				ok := downloadCache.Set(key, source[idx:min(idx+chunkSize, int64(len(source)))], 0, time.Minute)
				if !ok {
					t.Fatalf("Failed to add key %s to cache", key)
				}
			}

			data, err := DownloadData(nodes, "", header, tt.byteStart, tt.byteEnd, tt.offset, int64(decryptedSize))
			if err != nil {
				t.Fatalf("Request to mock server failed: %v", err)
			}
			expectedData := source[tt.byteStart:tt.byteEnd]
			if !reflect.DeepEqual(expectedData, data) {
				t.Fatalf("Function returned incorrect data\nExpected=%v\nReceived=%v", string(expectedData), string(data))
			}
			for _, idx := range tt.cachedIdxs {
				key := strings.Join(nodes, "/") + "_" + fmt.Sprintf("%d", idx)
				cachedData, ok := downloadCache.Get(key)
				if !ok {
					t.Fatalf("Data for key %s was not present in cache", key)
				}
				expectedData := source[idx:min(idx+chunkSize, int64(len(source)))]
				if !reflect.DeepEqual(expectedData, cachedData) {
					t.Fatalf("Cache did not contain correct data at key %s", key)
				}
				downloadCache.Del(key)
			}
			if len(storage.keys) > 0 {
				t.Fatalf("Data with keys %v were stored in cache even though they shoudn't've been", slices.Collect(maps.Keys(storage.keys)))
			}
		})
	}
}

func TestDownloadData_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	origPrivateKey := ai.vi.privateKey
	origCache := downloadCache
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
		ai.vi.privateKey = origPrivateKey
		downloadCache = origCache
	}()

	storage := &mockCache{}
	downloadCache = &cache.Ristretto{Cacheable: storage}

	decryptedSize := 80*1024*1024 + 10
	encryptedSize := decryptedSize + 35992
	content := test.GenerateRandomText(decryptedSize) // 3 chunks

	headerBytes, encryptedContent, privateKey := test.EncryptData(t, content)
	if len(headerBytes)+len(encryptedContent) != encryptedSize {
		t.Fatalf("Failed to split data correctly. Split data has header size %d and body size %d. They should sum up to %d", len(headerBytes), len(encryptedContent), encryptedSize)
	}
	header64 := base64.StdEncoding.EncodeToString(headerBytes)

	ai.vi.privateKey = privateKey
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}

	var tests = []struct {
		testname, errStr   string
		header             []string
		errorByte, maxByte int
	}{
		{
			"FAIL_SERVER_1",
			"failed to get data chunk: failed to retrieve object from Allas for path: api error InternalServerError: Internal Server Error",
			nil, 0, 60000000,
		},
		{
			"FAIL_SERVER_2",
			"failed to get second data chunk: failed to retrieve object from Allas for path: api error InternalServerError: Internal Server Error",
			[]string{header64}, 33568768, 60000000,
		},
		{
			"FAIL_INVALID_HEADER",
			"failed to get data chunk: failed to construct reader: not a Crypt4GH file",
			[]string{"SS1hbS1hLWhlYWRlcg=="}, -1, 60000000,
		},
		{
			"FAIL_HEADER_DECODE",
			"failed to get data chunk: failed to decode header: illegal base64 data at input byte 0",
			[]string{"H"}, -1, 60000000,
		},
		{
			"FAIL_READ",
			"failed to get second data chunk: failed to read file chunk [33554432, 67108864): data segment can't be decrypted with any of header keys",
			[]string{header64}, -1, 40000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				r.Body.Close()

				var byteStart, byteEnd int
				byteRange := r.Header["Range"]
				_, err := fmt.Sscanf(byteRange[0], "bytes=%d-%d", &byteStart, &byteEnd)
				if err != nil {
					t.Errorf("Header Range (value: %v) was in the incorrect format: %s", byteRange, err.Error())
				}

				if byteStart == int(tt.errorByte) {
					w.WriteHeader(http.StatusInternalServerError)

					return
				}

				byteEnd = min(byteEnd, tt.maxByte)
				_, _ = w.Write(encryptedContent[byteStart : byteEnd+1])
			}))
			ai.proxy = srv.URL
			t.Cleanup(func() { srv.Close() })

			if err := initialiseS3Client(); err != nil {
				t.Fatalf("Failed to initialize S3 client: %v", err.Error())
			}

			nodes := []string{"", SDConnect, "project", "bucket", "obj.txt.c4gh"}
			var header *string = nil
			if tt.header != nil {
				header = &tt.header[0]
			}

			storage.keys = make(map[string][]byte)
			_, err := DownloadData(nodes, "path", header, 33554000, 33564437, 0, int64(decryptedSize))
			if err == nil {
				t.Errorf("Function did not return error")
			} else if err.Error() != tt.errStr {
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errStr, err.Error())
			}
		})
	}
}

func TestUploadObject(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	var receivedData strings.Builder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method != "PUT" {
			t.Errorf("Request has incorrect method\nExpected=PUT\nReceived=%v", r.Method)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if r.URL.Path != "/s3-default-endpoint/some-repository/bucket1000/" {
			t.Errorf("Request has incorrect path %v", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		object := r.URL.Query().Get("object")
		if object != "obj.txt" {
			t.Errorf("Query parameter 'object' has incorrect value\nExpected=obj.txt\nReceived=%s", object)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		hdr1 := r.Header.Get("X-Amz-Meta-Key")
		if hdr1 != "value" {
			t.Errorf("Header parameter 'X-Amz-Meta-Key' has incorrect value\nExpected=value\nReceived=%s", hdr1)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		hdr2 := r.Header.Get("X-Amz-Meta-Another-key")
		if hdr2 != "another value" {
			t.Errorf("Header parameter 'X-Amz-Meta-Another-key' has incorrect value\nExpected=another value\nReceived=%s", hdr2)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		nextChunk, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %s", err.Error())
		} else {
			_, _ = receivedData.Write(nextChunk)
		}
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	uploadedData := test.GenerateRandomText(10003)
	reader := bytes.NewReader(uploadedData)
	metadata := map[string]string{"key": "value", "another-key": "another value"}

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if err := UploadObject(context.Background(), reader, "some-repository", "bucket1000", "obj.txt", 1024*1024*5, metadata); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else if receivedData.String() != string(uploadedData) {
		t.Errorf("Function uploaded incorrect data\nExpected=%s\nReceived=%s", string(uploadedData), receivedData.String())
	}
}

func TestUploadObject_Multipart(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	uploadID := ""
	receivedData := make(map[string][]byte)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.URL.Path != "/s3-default-endpoint/repo/bucket1000/" {
			t.Errorf("Request has incorrect path %v", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		object := r.URL.Query().Get("object")
		if object != "obj.txt" {
			t.Errorf("Query parameter 'object' has incorrect value\nExpected=obj.txt\nReceived=%s", object)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if r.Method == "POST" && r.URL.Query().Has("uploads") {
			// CreateMultipartUpload
			uploadID = "i-am-an-ID"

			xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<InitiateMultipartUploadResult>
	<Bucket>bucket1000</Bucket>
	<Key>obj.txt</Key>
	<UploadId>%s</UploadId>
</InitiateMultipartUploadResult>`
			fmt.Fprintf(w, xmlData, uploadID)

			return
		}
		receivedID := r.URL.Query().Get("uploadId")
		if receivedID != uploadID {
			t.Errorf("Query parameter 'uploadId' has incorrect value\nExpected=%s\nReceived=%s", uploadID, receivedID)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		if r.Method == "POST" {
			// CompleteMultipartUpload
			xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUploadResult>
	<Bucket>bucket1000</Bucket>
	<Key>obj.txt</Key>
</CompleteMultipartUploadResult>`
			_, _ = w.Write([]byte(xmlData))

			return
		}

		if r.Method != "PUT" {
			t.Errorf("Request has incorrect method %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		partNum := r.URL.Query().Get("partNumber")
		if partNum == "" {
			t.Error("Request has empty part number")
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		nextChunk, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %s", err.Error())
		} else {
			receivedData[partNum] = nextChunk
		}
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	uploadedData := test.GenerateRandomText(1024*1024*18 + 573746)
	reader := bytes.NewReader(uploadedData)

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if err := UploadObject(context.Background(), reader, "repo", "bucket1000", "obj.txt", 1024*1024*5, nil); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	} else {
		receivedDataStr := ""
		for i := 1; ; i++ {
			key := strconv.Itoa(i)
			nextPart, ok := receivedData[key]
			if !ok {
				break
			}
			receivedDataStr += string(nextPart)
		}

		if receivedDataStr != string(uploadedData) {
			t.Errorf("Function uploaded incorrect data\nExpected=%s\nReceived=%s", string(uploadedData), receivedDataStr)
		}
	}
}

func TestUploadObject_TooLarge(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	uploadedData := test.GenerateRandomText(1024)
	reader := bytes.NewReader(uploadedData)

	errStr := "failed to upload object obj.txt to bucket bucket1000: object is too large"
	if err := initialiseS3Client(); err != nil {
		t.Fatalf("Failed to initialize S3 client: %v", err.Error())
	} else if err := UploadObject(context.Background(), reader, "repo", "bucket1000", "obj.txt", 1024*1024*5, nil); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Fatalf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUploadObject_ContextCancel(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
	}))

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	uploadedData := test.GenerateRandomText(1024)
	reader := bytes.NewReader(uploadedData)

	errStr := "failed to upload object obj.txt to bucket bucket1000: canceled, context canceled" // `canceled,` comes from aws sdk
	if err := initialiseS3Client(); err != nil {
		t.Fatalf("Failed to initialize S3 client: %v", err.Error())
	} else if err := UploadObject(ctx, reader, "repo", "bucket1000", "obj.txt", 1024*1024*5, nil); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Fatalf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}

func TestUploadObject_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	deleted := false
	uploadID := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method == "POST" && r.URL.Query().Has("uploads") {
			// CreateMultipartupload
			uploadID = "i-am-an-ID"

			xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<InitiateMultipartUploadResult>
	<Bucket>bucket1000</Bucket>
	<Key>obj.txt</Key>
	<UploadId>%s</UploadId>
</InitiateMultipartUploadResult>`
			fmt.Fprintf(w, xmlData, uploadID)

			return
		}

		receivedID := r.URL.Query().Get("uploadId")
		if receivedID != uploadID {
			t.Errorf("Query parameter 'uploadId' has incorrect value\nExpected=%s\nReceived=%s", uploadID, receivedID)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		switch r.Method {
		case "PUT":
			w.WriteHeader(http.StatusInternalServerError)
		case "DELETE":
			deleted = true
		}
	}))

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	uploadedData := test.GenerateRandomText(1024*1024*18 + 573746)
	reader := bytes.NewReader(uploadedData)

	errStr := "failed to upload object objekti.txt to bucket bucket574: api error InternalServerError: Internal Server Error"
	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if err := UploadObject(context.Background(), reader, "repo", "bucket574", "objekti.txt", 1024*1024*5, nil); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	} else if !deleted {
		t.Error("Object was not deleted")
	}
}

func TestDeleteBucket(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		if r.Method != "DELETE" {
			t.Errorf("Request has incorrect method\nExpected=DELETE\nReceived=%v", r.Method)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if r.URL.Path != "/s3-default-endpoint/secret-repository/bucket007/" {
			t.Errorf("Request has incorrect path %v", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		object := r.URL.Query().Get("object")
		if object != "obj.txt" {
			t.Errorf("Query parameter 'object' has incorrect value\nExpected=obj.txt\nReceived=%s", object)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
	}))

	ai.hi.endpoints = testConfig
	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if err := DeleteObject("SECRET-repository", "bucket007", "obj.txt"); err != nil {
		t.Errorf("Request to mock server failed: %v", err)
	}
}

func TestDeleteBucket_Error(t *testing.T) {
	origClient := ai.hi.client
	origProxy := ai.proxy
	origS3Client := ai.hi.s3Client
	defer func() {
		ai.hi.client = origClient
		ai.proxy = origProxy
		ai.hi.s3Client = origS3Client
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	ai.hi.client = &http.Client{Transport: http.DefaultTransport}
	ai.proxy = srv.URL
	errStr := "failed to delete object obj.txt in bucket my-bucket: api error InternalServerError: Internal Server Error"
	t.Cleanup(func() { srv.Close() })

	if err := initialiseS3Client(); err != nil {
		t.Errorf("Failed to initialize S3 client: %v", err.Error())
	} else if err := DeleteObject(SDConnect, "my-bucket", "obj.txt"); err == nil {
		t.Error("Function did not return error")
	} else if err.Error() != errStr {
		t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", errStr, err.Error())
	}
}
