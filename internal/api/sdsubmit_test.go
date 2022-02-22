package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const constantToken = "token"
const constantError = "some error"

type mockSubmitter struct {
	submittable
	mockUrlOK    string
	mockDatasets []string
	mockFiles    []Metadata
	mockError    error
}

func (s *mockSubmitter) getDatasets(urlStr string) ([]string, error) {
	if urlStr == s.mockUrlOK {
		return s.mockDatasets, nil
	} else {
		return nil, s.mockError
	}
}

func (s *mockSubmitter) getFiles(fsPath, urlStr, dataset string) ([]Metadata, error) {
	if urlStr == s.mockUrlOK {
		return s.mockFiles, nil
	} else {
		return nil, s.mockError
	}
}

func TestGetDatasets_Fail(t *testing.T) {

	// Mock
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		return errors.New(constantError)
	}

	// Test
	expectedError := "Failed to retrieve SD-Submit datasets: some error"
	testToken := constantToken
	s := submitter{token: &testToken}
	_, err := s.getDatasets("url")

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestGetDatasets_Fail failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestGetDatasets_Pass(t *testing.T) {

	// Mock
	expectedBody := []string{"dataset1", "dataset2", "dataset3"}
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		_ = json.NewDecoder(bytes.NewReader([]byte(`["dataset1","dataset2","dataset3"]`))).Decode(ret)
		return nil
	}

	// Test
	testToken := constantToken
	s := submitter{token: &testToken}
	datasets, err := s.getDatasets("url")

	if err != nil {
		t.Errorf("TestGetDatasets_Fail failed, expected no error, received=%v", err)
	}
	if !reflect.DeepEqual(datasets, expectedBody) {
		t.Errorf("TestGetDatasets_Pass failed, expected=%s, received=%s", expectedBody, datasets)
	}

}

func TestGetFiles_Fail(t *testing.T) {

	// Mock
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		return errors.New(constantError)
	}

	// Test
	expectedError := "Failed to retrieve files for dataset \"fspath\": some error"
	testToken := constantToken
	s := submitter{token: &testToken}
	_, err := s.getFiles("fspath", "url", "dataset1")

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestGetFiles_Fail failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestGetFiles_Pass(t *testing.T) {

	// Mock
	testFile := []file{
		{
			FileID:                "file1",
			DatasetID:             "dataset1",
			DisplayFileName:       "file1.txt",
			FileName:              "file1.txt",
			FileSize:              10,
			DecryptedFileSize:     10,
			DecryptedFileChecksum: "abc123",
			FileStatus:            "READY",
		},
		{
			FileID:                "file2",
			DatasetID:             "dataset2",
			DisplayFileName:       "file2.txt",
			FileName:              "file2.txt",
			FileSize:              10,
			DecryptedFileSize:     10,
			DecryptedFileChecksum: "abc123",
			FileStatus:            "PENDING",
		},
	}
	testFileJSON, _ := json.Marshal(testFile)
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		_ = json.NewDecoder(bytes.NewReader(testFileJSON)).Decode(ret)
		return nil
	}

	// Test
	testToken := constantToken
	s := submitter{token: &testToken, fileIDs: make(map[string]string)}
	meta, err := s.getFiles("fspath", "url", "dataset1")

	if err != nil {
		t.Errorf("TestGetFiles_Pass failed, expected no error, received=%v", err)
	}
	if len(meta) != 1 {
		// We must get only one file, because only one file is ready, and the other one is pending
		t.Errorf("TestGetFiles_Pass failed, expected=%d, received=%d", 1, len(meta))
	}
	if meta[0].Name != testFile[0].DisplayFileName {
		t.Errorf("TestGetFiles_Pass failed, expected=%s, received=%s", testFile[0].DisplayFileName, meta[0].Name)
	}
	if s.fileIDs["dataset1_file1.txt"] != "file1" {
		t.Errorf("TestGetFiles_Pass failed, expected=%s, received=%s", "file1", s.fileIDs["dataset1_file1.txt"])
	}

}

func TestGetEnvs_Fail_AccessToken(t *testing.T) {

	// Mock
	expectedError := constantError
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()
	getEnv = func(name string, verifyURL bool) (string, error) {
		return "", errors.New(expectedError)
	}
	s := sdSubmitInfo{token: constantToken}

	// Test
	err := s.getEnvs()

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestGetEnvs_Fail_AccessToken failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestGetEnvs_Fail_SubmitAPI(t *testing.T) {

	// Mock
	expectedError := constantError
	origGetEnv := getEnv
	defer func() { getEnv = origGetEnv }()
	getEnv = func(name string, verifyURL bool) (string, error) {
		if name == "SDS_ACCESS_TOKEN" {
			return "token", nil
		} else {
			return "", errors.New(expectedError)
		}
	}
	s := sdSubmitInfo{token: constantToken}

	// Test
	err := s.getEnvs()

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestGetEnvs_Fail_SubmitAPI failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestGetEnvs_Fail_ValidURL(t *testing.T) {

	// Mock
	expectedError := constantError
	origGetEnv := getEnv
	origValidURL := validURL
	defer func() {
		getEnv = origGetEnv
		validURL = origValidURL
	}()
	getEnv = func(name string, verifyURL bool) (string, error) {
		return constantToken, nil
	}
	validURL = func(env string) error {
		return errors.New(constantError)
	}
	s := sdSubmitInfo{token: constantToken}

	// Test
	err := s.getEnvs()

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestGetEnvs_Fail_ValidURL failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestGetEnvs_Fail_TestURL(t *testing.T) {

	// Mock
	expectedError := "Cannot connect to SD-Submit API: some error"
	origGetEnv := getEnv
	origValidURL := validURL
	origTestURL := testURL
	defer func() {
		getEnv = origGetEnv
		validURL = origValidURL
		testURL = origTestURL
	}()
	getEnv = func(name string, verifyURL bool) (string, error) {
		return constantToken, nil
	}
	validURL = func(env string) error {
		return nil
	}
	testURL = func(url string) error {
		return errors.New(constantError)
	}
	s := sdSubmitInfo{token: constantToken}

	// Test
	err := s.getEnvs()

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestGetEnvs_Fail_TestURL failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestGetEnvs_Pass(t *testing.T) {

	// Mock
	expectedUrls := "url1,url2,url3"
	origGetEnv := getEnv
	origValidURL := validURL
	origTestURL := testURL
	defer func() {
		getEnv = origGetEnv
		validURL = origValidURL
		testURL = origTestURL
	}()
	getEnv = func(name string, verifyURL bool) (string, error) {
		if name == "SDS_ACCESS_TOKEN" {
			return constantToken, nil
		} else {
			return expectedUrls, nil
		}
	}
	validURL = func(env string) error {
		return nil
	}
	testURL = func(url string) error {
		return nil
	}
	s := sdSubmitInfo{token: constantToken, urls: make([]string, 0)}

	// Test
	err := s.getEnvs()

	if err != nil {
		t.Errorf("TestGetEnvs_Pass failed, expected no error, received=%v", err)
	}
	if s.token != constantToken {
		t.Errorf("TestGetEnvs_Pass failed, expected=%s, received=%s", constantToken, s.token)
	}
	if us := strings.Join(s.urls, ","); us != expectedUrls {
		t.Errorf("TestGetEnvs_Pass failed, expected=%s, received=%s", expectedUrls, us)
	}

}

func TestLoginMethod(t *testing.T) {

	s := &sdSubmitInfo{}
	loginMethod := s.getLoginMethod()
	if loginMethod != 1 {
		t.Errorf("TestGetLoginMethod failed expected=%d, received=%d", 1, loginMethod)
	}

}

func TestValidateLogin_Fail(t *testing.T) {

	// Mock
	ms := &mockSubmitter{mockUrlOK: "good", mockError: &RequestError{http.StatusUnauthorized}}
	s := &sdSubmitInfo{submittable: ms, urls: []string{"bad"}}

	// Test
	expectedError := "API responded with status 401 Unauthorized"
	err := s.validateLogin()

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestValidateLogin_Fail failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestValidateLogin_Pass_Found(t *testing.T) {

	// Mock
	ms := &mockSubmitter{mockUrlOK: "good", mockDatasets: []string{"dataset1", "dataset2", "dataset3"}}
	s := &sdSubmitInfo{submittable: ms, urls: []string{"good"}}

	// Test
	expectedDatasets := map[string]int{"dataset1": 0, "dataset2": 1, "dataset3": 2}
	err := s.validateLogin()

	if err != nil {
		t.Errorf("TestValidateLogin_Pass_Found failed, expected no error, received=%v", err)
	}
	if reflect.DeepEqual(s.datasets, expectedDatasets) {
		t.Errorf("TestValidateLogin_Pass_Found failed, expected=%v, received=%v", expectedDatasets, s.datasets)
	}

}

func TestValidateLogin_Pass_None(t *testing.T) {

	// Mock
	ms := &mockSubmitter{mockUrlOK: "good", mockDatasets: []string{}}
	s := &sdSubmitInfo{submittable: ms, urls: []string{"good"}}

	// Test
	expectedError := "No datasets found for SD-Submit"
	err := s.validateLogin()

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestValidateLogin_Pass_Found failed, expected=%s, received=%v", expectedError, err)
		}
	}
	if len(s.datasets) > 0 {
		t.Errorf("TestValidateLogin_Pass_Found failed, expected no datasets, received=%d", len(s.datasets))
	}

}

func TestGetToken_SDSubmit(t *testing.T) {
	s := sdSubmitInfo{token: constantToken}
	token := s.getToken()
	if token != constantToken {
		t.Errorf("TestGetToken_SDSubmit failed, expected=%s, received=%s", constantToken, token)
	}
}

func TestLevelCount_SDSubmit(t *testing.T) {
	s := sdSubmitInfo{}
	lc := s.levelCount()
	if lc != 2 {
		t.Errorf("TestLevelCount_SDSubmit failed, expected=%d, received=%d", 2, lc)
	}
}

func TestGetNthLevel_SDSubmit_Pass_0(t *testing.T) {

	// Mock
	s := &sdSubmitInfo{datasets: map[string]int{"dataset1": 1, "dataset2": 2}}

	// Test
	expectedDatasets := []Metadata{
		{
			Name:  "dataset1",
			Bytes: -1,
		},
		{
			Name:  "dataset2",
			Bytes: -1,
		},
	}
	datasets, err := s.getNthLevel("irrelevant")

	if err != nil {
		t.Errorf("TestGetNthLevel_SDSubmit_Pass_0 failed, expected no error, received=%v", err)
	}
	if !reflect.DeepEqual(datasets, expectedDatasets) {
		t.Errorf("TestGetNthLevel_SDSubmit_Pass_0 failed, expected=%v, received=%v", expectedDatasets, datasets)
	}

}

func TestGetNthLevel_SDSubmit_Fail_1(t *testing.T) {

	// Mock
	s := &sdSubmitInfo{datasets: map[string]int{"dataset1": 1}}

	// Test
	expectedError := "Tried to request files for invalid dataset \"fspath\""
	_, err := s.getNthLevel("fspath", "dataset2")

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestGetNthLevel_SDSubmit_Fail_1 failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestGetNthLevel_SDSubmit_Pass_1(t *testing.T) {

	// Mock
	ms := &mockSubmitter{mockUrlOK: "someurl", mockFiles: []Metadata{{Name: "file1.txt"}}}
	s := &sdSubmitInfo{
		submittable: ms,
		datasets:    map[string]int{"dataset1": 0, "dataset2": 1},
		urls:        []string{"someurl"},
	}

	// Test
	files, err := s.getNthLevel("fspath", "dataset1")

	if err != nil {
		t.Errorf("TestGetNthLevel_SDSubmit_Pass_1 failed, expected no error, received=%v", err)
	}
	if files[0].Name != "file1.txt" {
		t.Errorf("TestGetNthLevel_SDSubmit_Pass_1 failed, expected=%s, received=%s", "file1.txt", files[0].Name)
	}

}

func TestGetNthLevel_SDSubmit_Default(t *testing.T) {

	// Mock
	s := &sdSubmitInfo{}

	// Test
	files, err := s.getNthLevel("fspath", "node1", "node2")

	if err != nil {
		t.Errorf("TestGetNthLevel_SDSubmit_Default failed, expected no error, received=%v", err)
	}
	if files != nil {
		t.Errorf("TestGetNthLevel_SDSubmit_Default failed, expected no files, received=%v", files)
	}

}

func TestDownloadData_SDSubmit_Fail(t *testing.T) {

	// Mock
	s := sdSubmitInfo{datasets: map[string]int{"something": 0}}

	// Test
	expectedError := "Tried to request content of SD-Submit file \"file1\" with invalid dataset \"missing\""
	buf := []byte{}
	err := s.downloadData([]string{"missing", "file1"}, buf, 0, 0)

	if err != nil {
		if err.Error() != expectedError {
			t.Errorf("TestDownloadData_SDSubmit_Fail failed, expected=%s, received=%v", expectedError, err)
		}
	}

}

func TestDownloadData_SDSubmit_Pass(t *testing.T) {

	// Mock
	expectedData := []byte("hellothere")
	origMakeRequest := makeRequest
	defer func() { makeRequest = origMakeRequest }()
	makeRequest = func(url, token, repository string, query, headers map[string]string, ret interface{}) error {
		_, _ = io.ReadFull(bytes.NewReader(expectedData), ret.([]byte))
		return nil
	}
	s := sdSubmitInfo{
		token:    constantToken,
		urls:     []string{"url"},
		datasets: map[string]int{"dataset1": 0},
		fileIDs:  map[string]string{"dataset1_file1": "file1.txt"},
	}

	// Test
	buf := make([]byte, 10)
	err := s.downloadData([]string{"dataset1", "file1"}, buf, 0, 10)

	if err != nil {
		t.Errorf("TestDownloadData_SDSubmit_Pass failed, expected no error, received=%v", err)
	}
	if !bytes.Equal(buf, expectedData) {
		t.Errorf("TestDownloadData_SDSubmit_Pass failed, expected=%s, received=%s", string(expectedData), string(buf))
	}

}
