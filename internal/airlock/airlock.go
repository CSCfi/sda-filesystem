package airlock

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/neicnordic/crypt4gh/keys"
	c4ghHeaders "github.com/neicnordic/crypt4gh/model/headers"
	"github.com/neicnordic/crypt4gh/streaming"
	"golang.org/x/crypto/chacha20poly1305"
)

const minSegmentSize int64 = 1 << 27
const maxParts int64 = 10000
const maxObjectSize int64 = 5 * (1 << 40)

var ai = airlockInfo{}

type airlockInfo struct {
	publicKeys [][chacha20poly1305.KeySize]byte
}

type readCloser struct {
	io.Reader
	io.Closer

	errc chan error
}

// ExportPossible indicates whether or not user the user is allowed to export files outside the VM.
// The user must be the project manager and have SD Connect enabled.
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

// CheckObjectExistence checks if the file that is to be uploaded already has an
// equivalent object in S3 storage. If object exists, user if given a choice to either
// quit or override the object with the new file.
func CheckObjectExistence(filename, bucket string) error {
	object, bucket := reorderNames(filename, bucket)
	exists, err := api.BucketExists(api.SDConnect, bucket)
	if err != nil {
		return err
	}
	if !exists { // No such bucket, object cannot exist
		return nil
	}

	objects, err := api.GetObjects(api.SDConnect, bucket, api.SDConnect+"/"+api.GetProjectName()+"/"+bucket)
	if err != nil {
		return fmt.Errorf("could not determine if export will override data: %w", err)
	}
	exists = slices.ContainsFunc(objects, func(meta api.Metadata) bool {
		return meta.Name == object
	})
	if exists {
		r := bufio.NewReader(os.Stdin)

		for {
			fmt.Fprintf(os.Stderr, "Export will override existing object in Allas. Continue? [y/n] ")
			ans, err := r.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}

			ans = strings.ToLower(strings.TrimSpace(ans))

			if ans == "y" || ans == "yes" {
				return nil
			}
			if ans == "n" || ans == "no" {
				return fmt.Errorf("not permitted to override data")
			}
		}
	}

	logs.Info("New object will not override current object")

	return nil
}

// Upload uploads a file to SD Connect
func Upload(filename, bucket string) error {
	if err := getPublicKeys(); err != nil {
		return err
	}
	exists, err := api.BucketExists(api.SDConnect, bucket)
	if err != nil {
		return err
	}
	if !exists {
		logs.Info("Creating bucket ", bucket)
		if err := api.CreateBucket(api.SDConnect, bucket); err != nil {
			return err
		}
	}

	encryptedRC, encryptedFileSize, err := getFileDetails(filename)
	if err != nil {
		return fmt.Errorf("failed to get details for file %s: %w", filename, err)
	}
	defer encryptedRC.Close()

	// Save header for vault
	header, err := c4ghHeaders.ReadHeader(encryptedRC)
	if err != nil {
		return fmt.Errorf("failed to extract header from encrypted file: %w", err)
	}

	objectSize := encryptedFileSize - int64(len(header))
	if objectSize > maxObjectSize {
		return fmt.Errorf("file %s is too large (%d bytes)", filename, objectSize)
	}

	segmentSize := minSegmentSize
	for maxParts*segmentSize < encryptedFileSize {
		segmentSize <<= 1
	}

	object, bucket := reorderNames(filename, bucket)

	logs.Info("Encrypting file ", filename)
	logs.Info("Uploading header to vault")
	if err := api.PostHeader(header, bucket, object); err != nil {
		return fmt.Errorf("failed to upload header to vault: %w", err)
	}

	logs.Infof("Beginning to upload object %s to bucket %s", object, bucket)
	logs.Debugf("File size %v", encryptedFileSize)
	logs.Debugf("Segment size %v", segmentSize)

	err = api.UploadObject(encryptedRC, api.SDConnect, bucket, object, segmentSize)
	if err != nil {
		return fmt.Errorf("failed to upload object %s to bucket %s: %w", object, bucket, err)
	}
	if err = <-encryptedRC.errc; err != nil {
		logs.Debugf("Deleting object %s from bucket %s", object, bucket)
		if err2 := api.DeleteObject(api.SDConnect, bucket, object); err2 != nil {
			logs.Warningf("Data left in Allas after failed upload: %w", err2)
		}

		return fmt.Errorf("streaming file %s failed: %w", filename, err)
	}

	logs.Info("Upload complete")

	return nil
}

var extractKey = func(path string) ([32]byte, error) {
	encryptionKey := struct {
		Key64 string `json:"public_key_c4gh"`
	}{}

	err := api.MakeRequest("GET", path, nil, nil, nil, &encryptionKey)
	if err != nil {
		return [32]byte{}, err
	}

	logs.Debugf("Encryption key: %s", encryptionKey.Key64)

	return keys.ReadPublicKey(strings.NewReader(encryptionKey.Key64))
}

// getPublicKeys retrieves the necessary public keys and checks their validity
func getPublicKeys() error {
	ai.publicKeys = nil
	projectType := api.GetProjectType()

	if projectType != "findata" {
		allasKey, err := extractKey("/desktop/project-key")
		if err != nil {
			return fmt.Errorf("failed to get project public key: %w", err)
		}

		ai.publicKeys = append(ai.publicKeys, allasKey)
	}

	if projectType == "findata" || projectType == "registry" {
		findataKey, err := extractKey("/public-key")
		if err != nil {
			return fmt.Errorf("failed to get findata public key: %w", err)
		}

		ai.publicKeys = append(ai.publicKeys, findataKey)
	}

	return nil
}

func reorderNames(filename, directory string) (string, string) {
	before, after, _ := strings.Cut(strings.TrimRight(directory, "/"), "/")
	object := strings.TrimLeft(after+"/"+filepath.Base(filename+".c4gh"), "/")

	return object, before
}

var encrypt = func(file *os.File, pw *io.PipeWriter, errc chan error) {
	defer pw.Close()
	c4ghWriter, err := streaming.NewCrypt4GHWriterWithoutPrivateKey(pw, ai.publicKeys, nil)
	if err != nil {
		errc <- err

		return
	}
	defer c4ghWriter.Close()
	if _, err = io.Copy(c4ghWriter, file); err != nil {
		errc <- err

		return
	}
	errc <- nil
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
