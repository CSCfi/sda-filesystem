package api

import (
	"fmt"
	"net/url"
	"sda-filesystem/internal/logs"
	"strconv"
	"strings"
)

// This file contains structs and functions that are strictly for SD-Submit

const SDSubmit string = "SD-Submit"

type sdSubmitInfo struct {
	certPath string
	token    string
	urls     []string
	fileIDs  map[string]string
}

type file struct {
	FileID                    string `json:"fileId"`
	DatasetID                 string `json:"datasetId"`
	DisplayFileName           string `json:"displayFileName"`
	FileName                  string `json:"fileName"`
	FileSize                  int64  `json:"fileSize"`
	DecryptedFileSize         int64  `json:"decryptedFileSize"`
	DecryptedFileChecksum     string `json:"decryptedFileChecksum"`
	DecryptedFileChecksumType string `json:"decryptedFileChecksumType"`
	FileStatus                string `json:"fileStatus"`
}

func init() {
	possibleRepositories[SDSubmit] = &sdSubmitInfo{fileIDs: make(map[string]string)}
}

func (s *sdSubmitInfo) getEnvs() error {
	var err error
	s.token, err = getEnv("SDS_ACCESS_TOKEN", false)
	if err != nil {
		return err
	}
	s.certPath, err = getEnv("FS_SD_SUBMIT_CERTS", false)
	if err != nil {
		return err
	}
	urls, err := getEnv("FS_SD_SUBMIT_API", false)
	if err != nil {
		return err
	}
	for _, url := range strings.Split(urls, ",") {
		if err = validURL(url); err != nil {
			return err
		}
		s.urls = append(s.urls, url)
	}
	return nil
}

func (s *sdSubmitInfo) validateLogin(auth ...string) (err error) {
	_, err = s.getSecondLevel("0")
	return
}

func (s *sdSubmitInfo) getCertificatePath() string {
	return s.certPath
}

func (s *sdSubmitInfo) testURLs() error {
	for _, url := range s.urls {
		if err := testURL(url); err != nil {
			return fmt.Errorf("Cannot connect to SD-Submit API: %w", err)
		}
	}
	return nil
}

func (s *sdSubmitInfo) getToken() string {
	return s.token
}

func (s *sdSubmitInfo) isHidden() bool {
	return true
}

func (s *sdSubmitInfo) idxToURL(str string) (string, error) {
	idx, err := strconv.Atoi(str)
	if err != nil {
		return "", fmt.Errorf("%q should be an integer: %w", str, err)
	}
	return s.urls[idx], nil
}

func (s *sdSubmitInfo) getFirstLevel() ([]Metadata, error) {
	var metadata []Metadata
	for i := range s.urls {
		metadata = append(metadata, Metadata{Name: strconv.Itoa(i), Bytes: -1})
	}
	return metadata, nil
}

func (s *sdSubmitInfo) getSecondLevel(urlIdx string) ([]Metadata, error) {
	url, err := s.idxToURL(urlIdx)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s datasets: %w", SDSubmit, err)
	}

	// Request datasets
	var datasets []string
	err = makeRequest(strings.TrimSuffix(url, "/")+
		"/metadata/datasets", s.token, SDSubmit, nil, nil, &datasets)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s datasets: %w", SDSubmit, err)
	}

	var metadata []Metadata
	for i := range datasets {
		metadata = append(metadata, Metadata{Name: datasets[i], Bytes: -1})
	}

	logs.Debugf("Retrieved %d %s dataset(s) from API %s", len(datasets), SDSubmit, url)
	return metadata, nil
}

func (s *sdSubmitInfo) getThirdLevel(urlIdx, dataset string) ([]Metadata, error) {
	u, err := s.idxToURL(urlIdx)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve files for dataset %q: %w", SDSubmit+"/"+dataset, err)
	}

	// Request files
	var files []file
	err = makeRequest(strings.TrimSuffix(u, "/")+
		"/metadata/datasets/"+
		url.PathEscape(dataset)+"/files", s.token, SDSubmit, nil, nil, &files)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve files for dataset %q: %w", SDSubmit+"/"+dataset, err)
	}

	var metadata []Metadata
	for i := range files {
		if files[i].FileStatus == "READY" {
			displayName := files[i].DisplayFileName //strings.TrimSuffix(files[i].DisplayFileName, ".c4gh")
			metadata = append(metadata, Metadata{Name: displayName, Bytes: files[i].DecryptedFileSize})
			s.fileIDs[dataset+"_"+displayName] = url.PathEscape(files[i].FileID)
		}
	}

	logs.Infof("Retrieved files for %s dataset %s", SDSubmit, dataset)
	return metadata, nil
}

// Dummy function, not needed
func (s *sdSubmitInfo) updateAttributes(nodes []string, path string, attr interface{}) {
}

func (s *sdSubmitInfo) downloadData(nodes []string, buffer []byte, start, end int64) error {
	url, err := s.idxToURL(nodes[0])
	if err != nil {
		return err
	}

	// Query params
	query := map[string]string{
		"startCoordinate": strconv.FormatInt(start, 10),
		"endCoordinate":   strconv.FormatInt(end, 10),
	}

	finalPath := nodes[1] + "_" + nodes[2]

	// Request data
	return makeRequest(strings.TrimSuffix(url, "/")+"/files/"+s.fileIDs[finalPath], s.token, SDSubmit, query, nil, buffer)
}
