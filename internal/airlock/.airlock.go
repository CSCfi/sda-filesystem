package airlock

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/neicnordic/crypt4gh/keys"
	"github.com/neicnordic/crypt4gh/streaming"
	"golang.org/x/crypto/chacha20poly1305"
)

const mebibyte = uint64(1 << 20)

var ai = airlockInfo{}

type airlockInfo struct {
	publicKeys [][chacha20poly1305.KeySize]byte
}

type readCloser struct {
	io.Reader
	io.Closer

	errc chan error
}

/*
func readPassword() (string, error) {
	password, passwordFromEnv := os.LookupEnv("CSC_PASSWORD")

	if passwordFromEnv {
		logs.Info("Using password from environment variable CSC_PASSWORD")
	} else {
		fmt.Println("Enter Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", fmt.Errorf("could not read password: %s", err.Error())
		}
	}

	return false, nil
}*/

func ExportPossible() bool {
	var insert string
	enabled := api.SDConnectEnabled()
	if !enabled {
		insert = "do not "
	}
	logs.Infof("You %shave SD Connect enabled", insert)

	insert = ""
	isManager := api.IsProjectManager()
	if !isManager {
		insert = "not "
	}
	logs.Infof("You are %sthe project manager", insert)

	return enabled && isManager
}

var extractKey = func(url string) ([32]byte, error) {
	encryptionKey := struct {
		Key64 string `json:"public_key_c4gh"`
	}{}

	err := api.MakeRequest("GET", url, nil, nil, nil, &encryptionKey)
	if err != nil {
		return [32]byte{}, err
	}

	logs.Debugf("Encryption key: %s", encryptionKey.Key64)

	return keys.ReadPublicKey(strings.NewReader(encryptionKey.Key64))
}

// getPublicKeys retrieves the public key from KrakenD and checks their validity
func getPublicKeys() error {
	ai.publicKeys = nil
	projectType := api.GetProjectType()

	if projectType != "findata" {
		allasKey, err := extractKey("/desktop/project-key")
		if err != nil {
			return fmt.Errorf("failed to get project public key for Airlock: %w", err)
		}

		ai.publicKeys = append(ai.publicKeys, allasKey)
	}

	if projectType == "findata" || projectType == "registry" {
		findataKey, err := extractKey("/public-key")
		if err != nil {
			return fmt.Errorf("failed to get findata public key for Airlock: %w", err)
		}

		ai.publicKeys = append(ai.publicKeys, findataKey)
	}

	return nil
}

// Upload uploads a file to SD Connect
func Upload(filename, container string, segmentSizeMb uint64, originalFilename string) error {
	var err error
	var encryptedRC *readCloser
	var encryptedFileSize int64

	logs.Info("Encrypting file ", filename)
	if err = getPublicKeys(); err != nil {
		return err
	}

	encryptedRC, encryptedFileSize, err = getFileDetails(filename)
	if err != nil {
		return fmt.Errorf("failed to get details for file %s: %w", filename, err)
	}
	defer encryptedRC.Close()

	segmentSize := segmentSizeMb * mebibyte
	logs.Debugf("File size %v", encryptedFileSize)
	logs.Debugf("Segment size %v", segmentSize)

	// Get total number of segments
	segmentNro := uint64(math.Ceil(float64(encryptedFileSize) / float64(segmentSize)))

	filename += ".c4gh"
	object, container := reorderNames(filename, container)
	logs.Info("Beginning to upload object " + object + " to container " + container)

	//ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(ai.hi.requestTimeout))
	ctx := context.Background()
	ctx = context.WithValue(ctx, api.EndpointKey{}, "/s3")
	//defer cancel()

	s3Client := api.GetS3Client()
	resp, err := s3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(container),
		Key:         aws.String(object),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("could not begin upload: %w", err)
	}

	var partNumber int32 = 1
	var completedParts []types.CompletedPart
	// This could have goroutines
	for i := uint64(0); i < segmentNro; i++ {
		segmentStart := int64(float64(i * segmentSize))
		segmentEnd := int64(math.Min(float64(encryptedFileSize),
			float64((i+1)*segmentSize)))

		thisSegmentSize := segmentEnd - segmentStart
		logs.Debugf("Segment start %v", segmentStart)
		logs.Debugf("Segment end %v", segmentEnd)

		logs.Infof("Uploading segment %v/%v", i+1, segmentNro)

		uploadResult, err := s3Client.UploadPart(context.TODO(), &s3.UploadPartInput{
			Body:          io.LimitReader(encryptedRC, thisSegmentSize),
			Bucket:        resp.Bucket,
			Key:           resp.Key,
			PartNumber:    aws.Int32(partNumber),
			UploadId:      resp.UploadId,
			ContentLength: aws.Int64(thisSegmentSize),
		})
		if err != nil {
			logs.Info("Aborting upload")
			_, abortErr := s3Client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
				Bucket:   resp.Bucket,
				Key:      resp.Key,
				UploadId: resp.UploadId,
			})
			if abortErr != nil {
				return fmt.Errorf("abort failed: %w", abortErr)
			}
			return fmt.Errorf("uploading segment %d failed: %w", partNumber, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadResult.ETag,
			PartNumber: aws.Int32(partNumber),
		})
	}

	sort.Slice(completedParts, func(i, j int) bool {
		return *completedParts[i].PartNumber < *completedParts[j].PartNumber
	})

	_, err = s3Client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   resp.Bucket,
		Key:      resp.Key,
		UploadId: resp.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		return fmt.Errorf("could not complete upload: %w", err)
	}

	// Don't know how this will behave with s3
	if encryptedRC.errc != nil {
		if err = <-encryptedRC.errc; err != nil {
			return fmt.Errorf("streaming file failed: %w", err)
		}
	}

	logs.Info("Upload complete")

	return nil
}

func reorderNames(filename, directory string) (string, string) {
	before, after, _ := strings.Cut(strings.TrimRight(directory, "/"), "/")
	object := strings.TrimLeft(after+"/"+filepath.Base(filename), "/")

	return object, before
}

var encrypt = func(file *os.File, pw *io.PipeWriter, errc chan error) {
	defer pw.Close()
	c4ghWriter, err := streaming.NewCrypt4GHWriterWithoutPrivateKey(pw, ai.publicKeys, nil)
	if err != nil {
		errc <- err

		return
	}
	if _, err = io.Copy(c4ghWriter, file); err != nil {
		errc <- err

		return
	}
	errc <- c4ghWriter.Close()
}

var encryptedSize = func(decryptedSize int64) int64 {
	var magicNumber int64 = 16
	var keySize int64 = 108
	var nKeys int64 = int64(len(ai.publicKeys))

	nBlocks := math.Ceil(float64(decryptedSize) / float64(api.BlockSize))
	bodySize := decryptedSize + int64(nBlocks)*api.MacSize
	headerSize := magicNumber + nKeys*keySize

	return headerSize + bodySize
}

var getFileDetails = func(filename string) (*readCloser, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	fileInfo, _ := file.Stat()
	fileSize := fileInfo.Size()

	errc := make(chan error, 1) // This error will be checked at the end of Upload()
	pr, pw := io.Pipe()         // So that 'c4ghWriter' can pass its contents to an io.Reader
	go encrypt(file, pw, errc)  // Writing from file to 'c4ghWriter' will not happen until 'pr' is being read. Code will hang without goroutine.
	fileSize = encryptedSize(fileSize)

	return &readCloser{pr, file, errc}, fileSize, nil
}
