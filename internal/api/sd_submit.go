package api

import (
	"errors"
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
	datasets map[int][]Metadata // keys are url indexes (only the ones user can access)
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
	possibleRepositories[SDSubmit] = &sdSubmitInfo{fileIDs: make(map[string]string), datasets: make(map[int][]Metadata)}
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

func (s *sdSubmitInfo) validateLogin(auth ...string) error {
	count := 0

	for i := range s.urls {
		datasets, err := s.getSecondLevel(strconv.Itoa(i))
		if err != nil {
			var re *RequestError
			if errors.As(err, &re) && (re.StatusCode == 401 || re.StatusCode == 404) {
				logs.Warningf("You do not have permission to access API %s", s.urls[i])
			} else {
				logs.Warningf("Something went wrong when validating token for API %s: %w", s.urls[i], err)
			}
			count++
		} else {
			s.datasets[i] = datasets
		}
	}

	if count == len(s.urls) {
		return fmt.Errorf("Cannot receive responses from any of the %s APIs", SDSubmit)
	}
	return nil
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

func (s *sdSubmitInfo) strToIdx(str string) (int, error) {
	idx, err := strconv.Atoi(str)
	if err != nil {
		return -1, fmt.Errorf("%q should be an integer: %w", str, err)
	}
	return idx, nil
}

func (s *sdSubmitInfo) getFirstLevel() ([]Metadata, error) {
	var metadata []Metadata
	for idx := range s.datasets {
		metadata = append(metadata, Metadata{Name: strconv.Itoa(idx), Bytes: -1})
	}
	return metadata, nil
}

func (s *sdSubmitInfo) getSecondLevel(urlIdx string) ([]Metadata, error) {
	idx, err := s.strToIdx(urlIdx)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s datasets: %w", SDSubmit, err)
	}

	// So that datasets are called only once (datasets needed for fuse, login, gui)
	if data, ok := s.datasets[idx]; ok {
		return data, nil
	}

	// Request datasets
	var datasets []string
	err = makeRequest(strings.TrimSuffix(s.urls[idx], "/")+
		"/metadata/datasets", s.token, SDSubmit, nil, nil, &datasets)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s datasets: %w", SDSubmit, err)
	}

	var metadata []Metadata
	for i := range datasets {
		md := Metadata{Name: datasets[i], Bytes: -1}
		if u, err := url.ParseRequestURI(datasets[i]); err == nil {
			md.Name = strings.TrimLeft(strings.TrimPrefix(datasets[i], u.Scheme), ":/")
			md.OrigName = datasets[i]
		}
		metadata = append(metadata, md)
	}

	logs.Infof("Retrieved %d %s dataset(s) from API %s", len(datasets), SDSubmit, s.urls[idx])
	return metadata, nil
}

func (s *sdSubmitInfo) getThirdLevel(urlIdx, dataset string) ([]Metadata, error) {
	idx, err := s.strToIdx(urlIdx)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve files for dataset %q: %w", SDSubmit+"/"+dataset, err)
	}

	// Request files
	var files []file
	err = makeRequest(strings.TrimSuffix(s.urls[idx], "/")+
		"/metadata/datasets/"+
		url.PathEscape(dataset)+"/files", s.token, SDSubmit, nil, nil, &files)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve files for dataset %q: %w", SDSubmit+"/"+dataset, err)
	}

	var metadata []Metadata
	for i := range files {
		if files[i].FileStatus == "READY" {
			md := Metadata{Name: files[i].DisplayFileName, Bytes: files[i].DecryptedFileSize}

			if strings.HasSuffix(files[i].DisplayFileName, ".c4gh") {
				md.Name = strings.TrimSuffix(files[i].DisplayFileName, ".c4gh")
				md.OrigName = files[i].DisplayFileName
			}

			metadata = append(metadata, md)
			s.fileIDs[dataset+"_"+files[i].DisplayFileName] = url.PathEscape(files[i].FileID)
		}
	}

	logs.Infof("Retrieved files for %s dataset %s", SDSubmit, dataset)
	return metadata, nil
}

// Dummy function, not needed
func (s *sdSubmitInfo) updateAttributes(nodes []string, path string, attr interface{}) {
}

func (s *sdSubmitInfo) downloadData(nodes []string, buffer []byte, start, end int64) error {
	idx, err := s.strToIdx(nodes[0])
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
	return makeRequest(strings.TrimSuffix(s.urls[idx], "/")+
		"/files/"+s.fileIDs[finalPath], s.token, SDSubmit, query, nil, buffer)
}
