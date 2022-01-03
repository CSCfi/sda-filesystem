package api

import (
	"errors"
	"fmt"
	"net/url"
	"sda-filesystem/internal/logs"
	"strconv"
	"strings"
	"sync"
)

// This file contains structs and functions that are strictly for SD-Submit

const SDSubmit string = "SD Submit"

type submittable interface {
	getFiles(string) ([]Metadata, error)
	getDatasets(int64) ([]Metadata, error)
}

type submitter struct {
}

type sdSubmitInfo struct {
	submittable
	token    string
	urls     []string
	lock     sync.RWMutex
	fileIDs  map[string]string
	datasets map[string]Metadata
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
	urls, err := getEnv("FS_SD_SUBMIT_API", false)
	if err != nil {
		return err
	}
	s.urls = []string{}
	for i, u := range strings.Split(urls, ",") {
		if err = validURL(u); err != nil {
			return err
		}
		s.urls = append(s.urls, strings.TrimRight(u, "/"))
		if err := testURL(s.urls[i]); err != nil {
			return fmt.Errorf("Cannot connect to SD-Submit API: %w", err)
		}
	}
	return nil
}

func (s *sdSubmitInfo) getLoginMethod() LoginMethod {
	return Token
}

func (s *sdSubmitInfo) validateLogin(auth ...string) error {
	s.datasets = make(map[string]Metadata)
	count := 0

	for i := range s.urls {
		datasets, err := s.getDatasets(int64(i))
		if err != nil {
			var re *RequestError
			if errors.As(err, &re) && (re.StatusCode == 401 || re.StatusCode == 404) {
				return err
			}
			logs.Warningf("Something went wrong when fetching %s datasets from API %s: %w", SDSubmit, s.urls[i], err)
			count++
		} else {
			for j := range datasets {
				s.datasets[datasets[j].OrigName] = datasets[j]
			}
		}
	}

	if count == len(s.urls) {
		return fmt.Errorf("Cannot receive responses from any of the %s APIs", SDSubmit)
	}
	if len(s.datasets) == 0 {
		return fmt.Errorf("No datasets found for %s", SDSubmit)
	}
	return nil
}

func (s *sdSubmitInfo) getToken() string {
	return s.token
}

func (s *sdSubmitInfo) levelCount() int {
	return 2
}

func (s *sdSubmitInfo) getNthLevel(nodes ...string) ([]Metadata, error) {
	switch len(nodes) {
	case 0:
		i := 0
		datasets := make([]Metadata, len(s.datasets))
		for _, meta := range s.datasets {
			datasets[i] = Metadata{Name: meta.Name, OrigName: meta.OrigName, Bytes: -1}
			i++
		}
		return datasets, nil
	case 1:
		return s.getFiles(nodes[0])
	default:
		return nil, nil
	}
}

func (s *sdSubmitInfo) getDatasets(idx int64) ([]Metadata, error) {
	var datasets []string
	err := makeRequest(s.urls[idx]+"/metadata/datasets", s.token, SDSubmit, nil, nil, &datasets)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s datasets: %w", SDSubmit, err)
	}

	var metadata []Metadata
	for i := range datasets {
		md := Metadata{Name: datasets[i], Bytes: idx} // Here Bytes represents url idx just because it's a convenient int field
		if u, err := url.ParseRequestURI(datasets[i]); err == nil {
			md.Name = strings.TrimLeft(strings.TrimPrefix(datasets[i], u.Scheme), ":/")
			md.OrigName = datasets[i]
		}
		metadata = append(metadata, md)
	}

	logs.Infof("Retrieved %d %s dataset(s) from API %s", len(datasets), SDSubmit, s.urls[idx])
	return metadata, nil
}

func (s *sdSubmitInfo) getFiles(dataset string) ([]Metadata, error) {
	meta, ok := s.datasets[dataset]
	if !ok {
		return nil, fmt.Errorf("Tried to request %s files for invalid dataset %s", SDSubmit, dataset)
	}
	idx := meta.Bytes

	// Request files
	var files []file
	path := s.urls[idx] + "/metadata/datasets/" + url.PathEscape(dataset) + "/files"
	err := makeRequest(path, s.token, SDSubmit, nil, nil, &files)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve files for dataset %q: %w", SDSubmit+"/"+dataset, err)
	}

	var metadata []Metadata
	for i := range files {
		if files[i].FileStatus == "READY" {
			md := Metadata{Name: files[i].DisplayFileName, Bytes: files[i].DecryptedFileSize}
			if strings.HasSuffix(md.Name, ".c4gh") {
				md.Name = strings.TrimSuffix(files[i].DisplayFileName, ".c4gh")
				md.OrigName = files[i].DisplayFileName
			}
			metadata = append(metadata, md)

			s.lock.Lock()
			s.fileIDs[dataset+"_"+files[i].DisplayFileName] = url.PathEscape(files[i].FileID)
			s.lock.Unlock()
		}
	}

	logs.Infof("Retrieved files for %s dataset %q", SDSubmit, dataset)
	return metadata, nil
}

// Dummy function, not needed
func (s *sdSubmitInfo) updateAttributes(nodes []string, path string, attr interface{}) {
}

func (s *sdSubmitInfo) downloadData(nodes []string, buffer interface{}, start, end int64) error {
	meta, ok := s.datasets[nodes[0]]
	if !ok {
		return fmt.Errorf("Tried to request content of %s file %q with invalid dataset %q", SDSubmit, nodes[1], nodes[0])
	}
	idx := meta.Bytes

	// Query params
	query := map[string]string{
		"startCoordinate": strconv.FormatInt(start, 10),
		"endCoordinate":   strconv.FormatInt(end, 10),
	}

	finalPath := nodes[0] + "_" + nodes[1]

	// Request data
	path := s.urls[idx] + "/files/" + s.fileIDs[finalPath]
	return makeRequest(path, s.token, SDSubmit, query, nil, buffer)
}
