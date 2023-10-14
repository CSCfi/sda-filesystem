package api

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"sda-filesystem/internal/logs"
)

// This file contains structs and functions that are strictly for SD Submit
// We name it SD Apply as that is where the datasets access is registered
// However the datasets are mostly in SD Submit backend

const SDSubmit string = "SD-Apply"
const SDSubmitPrnt string = "SD Apply"

// This exists for unit test mocking
type submittable interface {
	getFiles(string, string, string) ([]Metadata, error)
	getDatasets(string) ([]string, error)
}

type submitter struct {
	lock    sync.RWMutex
	fileIDs map[string]string
}

type sdSubmitInfo struct {
	submittable
	urls     []string
	fileIDs  map[string]string
	datasets map[string]int
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
	su := &submitter{fileIDs: make(map[string]string)}
	sd := &sdSubmitInfo{submittable: su}
	sd.fileIDs = su.fileIDs
	allRepositories[SDSubmit] = sd
}

//
// Functions for submitter
//

func (s *submitter) getDatasets(urlStr string) ([]string, error) {
	var datasets []string
	err := MakeRequest(urlStr+"/metadata/datasets", nil, nil, nil, &datasets)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve %s datasets from API %s: %w", SDSubmitPrnt, urlStr, err)
	}

	logs.Infof("Retrieved %d %s dataset(s) from API %s", len(datasets), SDSubmitPrnt, urlStr)

	return datasets, nil
}

func (s *submitter) getFiles(fsPath, urlStr, dataset string) ([]Metadata, error) {
	var query map[string]string
	origDataset := dataset
	split := strings.Split(dataset, "://")
	if len(split) > 1 {
		query = map[string]string{"scheme": split[0]}
		dataset = strings.Join(split[1:], "://")
	}

	// Request files
	var files []file
	path := urlStr + "/metadata/datasets/" + url.PathEscape(dataset) + "/files"
	err := MakeRequest(path, query, nil, nil, &files)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve files for dataset %s: %w", fsPath, err)
	}

	var metadata []Metadata
	for i := range files {
		if files[i].FileStatus == "READY" {
			md := Metadata{Name: files[i].DisplayFileName, Bytes: files[i].DecryptedFileSize}
			metadata = append(metadata, md)

			s.lock.Lock()
			s.fileIDs[origDataset+"_"+files[i].DisplayFileName] = url.PathEscape(files[i].FileID)
			s.lock.Unlock()
		}
	}

	logs.Infof("Retrieved files for dataset %s", fsPath)

	return metadata, nil
}

//
// Functions for sdSubmitInfo
//

func (s *sdSubmitInfo) getEnvs() error {
	var err error
	urls, err := GetEnv("FS_SD_SUBMIT_API", false)
	if err != nil {
		return err
	}
	s.urls = []string{}
	for i, u := range strings.Split(urls, ",") {
		if err = validURL(u); err != nil {
			return fmt.Errorf("%s API not a valid URL: %w", SDSubmitPrnt, err)
		}
		s.urls = append(s.urls, strings.TrimRight(u, "/"))
		if err := testURL(s.urls[i]); err != nil {
			return fmt.Errorf("Cannot connect to %s registered API: %w", SDSubmitPrnt, err)
		}
	}

	return nil
}

func (s *sdSubmitInfo) authenticate(_ ...string) error {
	s.datasets = make(map[string]int)
	count, count500 := 0, 0

	for i := range s.urls {
		datasets, err := s.getDatasets(s.urls[i])
		if err != nil {
			var re *RequestError
			if errors.As(err, &re) && re.StatusCode == 401 {
				return fmt.Errorf("%s authorization failed", SDSubmitPrnt)
			}

			logs.Warning(err)
			if errors.As(err, &re) && re.StatusCode == 500 {
				count500++
			} else {
				count++
			}
		} else {
			for j := range datasets {
				s.datasets[datasets[j]] = i
			}
		}
	}

	if len(s.datasets) == 0 {
		switch {
		case count500 > 0:
			return fmt.Errorf("%s is not available, please contact CSC servicedesk", SDSubmitPrnt)
		case count > 0:
			return fmt.Errorf("%s APIs failed to retrieve any data", SDSubmitPrnt)
		default:
			return fmt.Errorf("No datasets found for %s", SDSubmitPrnt)
		}
	}

	return nil
}

func (s *sdSubmitInfo) levelCount() int {
	return 2
}

func (s *sdSubmitInfo) getNthLevel(fsPath string, nodes ...string) ([]Metadata, error) {
	switch len(nodes) {
	case 0:
		i := 0
		datasets := make([]Metadata, len(s.datasets))
		for ds := range s.datasets {
			datasets[i] = Metadata{Name: ds, Bytes: -1}
			i++
		}

		return datasets, nil
	case 1:
		idx, ok := s.datasets[nodes[0]]
		if !ok {
			return nil, fmt.Errorf("Tried to request files for invalid dataset %s", fsPath)
		}

		return s.getFiles(fsPath, s.urls[idx], nodes[0])
	default:
		return nil, nil
	}
}

// Dummy function, not needed
func (s *sdSubmitInfo) updateAttributes(_ []string, _ string, _ any) error {
	return nil
}

func (s *sdSubmitInfo) downloadData(nodes []string, buffer any, start, end int64) error {
	idx, ok := s.datasets[nodes[0]]
	if !ok {
		return fmt.Errorf("Tried to request content of %s file %s with invalid dataset %s", SDSubmitPrnt, nodes[1], nodes[0])
	}

	// Query params
	query := map[string]string{
		"startCoordinate": strconv.FormatInt(start, 10),
		"endCoordinate":   strconv.FormatInt(end, 10),
	}

	// Request data
	path := s.urls[idx] + "/files/" + s.fileIDs[nodes[0]+"_"+nodes[1]]

	return MakeRequest(path, query, nil, nil, buffer)
}
