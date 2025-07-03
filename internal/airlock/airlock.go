package airlock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	c4ghHeaders "github.com/neicnordic/crypt4gh/model/headers"
	"github.com/neicnordic/crypt4gh/streaming"
	"golang.org/x/crypto/chacha20poly1305"
)

const minSegmentSize int64 = 1 << 27
const maxParts int64 = 10000
const maxObjectSize int64 = 5 * (1 << 40)
const headerSize = 16 + 108 // Fixed size since we only have one public key

var ai = airlockInfo{}

type airlockInfo struct {
	publicKey [chacha20poly1305.KeySize]byte
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
// equivalent object in S3 storage. If object exists, user is given a choice to either
// quit or override the object with the new file.
func CheckObjectExistence(filename, bucket string, rd io.Reader) error {
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
		r := bufio.NewReader(rd)

		for {
			fmt.Fprintf(os.Stdout, "Export will override existing object in Allas. Continue? [y/n] ")
			ans, err := r.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read user input: %w", err)
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

	logs.Info("New object will not override current objects")

	return nil
}

// Upload uploads a file to SD Connect, and possibly to CESSNA
func Upload(filename, bucket string) error {
	var err error
	ai.publicKey, err = api.GetPublicKey()
	if err != nil {
		return fmt.Errorf("failed to get project public key: %w", err)
	}

	object, bucket := reorderNames(filename, bucket)
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

	file, encryptedFileSize, err := getFileDetails(filename)
	if err != nil {
		return fmt.Errorf("failed to get details for file %s: %w", filename, err)
	}
	defer file.Close()

	objectSize := encryptedFileSize - headerSize
	if objectSize > maxObjectSize {
		return fmt.Errorf("file %s is too large (%d bytes)", filename, objectSize)
	}
	logs.Debugf("File size %v", encryptedFileSize)

	segmentSize := minSegmentSize
	for maxParts*segmentSize < encryptedFileSize {
		segmentSize <<= 1
	}
	logs.Debugf("Segment size %v", segmentSize)
	logs.Info("Encrypting file ", filename)

	errc1 := make(chan error, 1)
	errc2 := make(chan error, 2)
	pr, pw := io.Pipe() // So that 'c4ghWriter' can pass its contents to an io.Reader

	if api.GetProjectType() == "default" {
		go encrypt(file, pw, errc2)
	}
	go func() {
		errc1 <- uploadAllas(pr, bucket, object, segmentSize)
		pr.Close()
	}()

	err = nil
	if api.GetProjectType() != "default" {
		errc2 <- nil
		findataObject := api.GetProjectName() + "/" + bucket + "/" + object
		if err = uploadFindata(file, pw, findataObject, segmentSize); err != nil {
			logs.Error(err)
		}
	}

	err2 := <-errc2
	if err2 != nil {
		logs.Debugf("Deleting object %s from bucket %s", object, bucket)
		if delErr := api.DeleteObject(api.SDConnect, bucket, object); delErr != nil {
			logs.Warningf("Data left in Allas after failed upload: %w", delErr)
		}
		logs.Errorf("Streaming file %s failed: %w", filename, err2)
	}
	err1 := <-errc1
	if err1 != nil {
		logs.Error(err1)
	}
	if err != nil || err1 != nil || err2 != nil {
		return errors.New("uploading failed")
	}

	return nil
}

// uploadAllas exports an encrypted file by uploading its header to Vault and
// its body to SD Connect. pr is assumed to contain an encrypted file.
func uploadAllas(pr io.Reader, bucket, object string, segmentSize int64) error {
	logs.Infof("Beginning to upload %s object %s to bucket %s", api.ToPrint(api.SDConnect), object, bucket)
	header, err := c4ghHeaders.ReadHeader(pr)
	if err != nil {
		return fmt.Errorf("failed to extract header from encrypted file: %w", err)
	}
	logs.Infof("Uploading header of object %s to Vault", object)
	if err := api.PostHeader(header, bucket, object); err != nil {
		return fmt.Errorf("failed to upload header to vault: %w", err)
	}

	return api.UploadObject(pr, api.SDConnect, bucket, object, segmentSize)
}

// uploadFindata uploads the selected file unencrypted to CESSNA. At the same time
// it sends the read file content through a Crypt4GHWriter, which will be read via the
// pipe reader in uploadAllas(). If upload fails, the pipe writer is closed with an error
// resulting in the AWS SDK to fail in uploadAllas().
func uploadFindata(file io.Reader, pw *io.PipeWriter, object string, segmentSize int64) (err error) {
	logs.Infof("Beginning to upload %s object %s", api.Findata, object)
	defer func() {
		pw.CloseWithError(err) // pw.Close() if err is nil
	}()

	c4ghWriter, err := newCrypt4GHWriter(pw)
	if err != nil {
		return err
	}

	r := io.TeeReader(file, c4ghWriter)
	err = api.UploadObject(r, api.Findata, "", object, segmentSize)
	if err != nil {
		err = fmt.Errorf("failed to upload %s object: %w", api.Findata, err)

		return
	}
	c4ghWriter.Close()

	return
}

var newCrypt4GHWriter = func(wr io.Writer) (*streaming.Crypt4GHWriter, error) {
	c4ghWriter, err := streaming.NewCrypt4GHWriterWithoutPrivateKey(wr, [][32]byte{ai.publicKey}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create crypt4gh writer: %w", err)
	}

	return c4ghWriter, nil
}

func reorderNames(filename, directory string) (string, string) {
	before, after, _ := strings.Cut(strings.TrimRight(directory, "/"), "/")
	object := strings.TrimLeft(after+"/"+filepath.Base(filename+".c4gh"), "/")

	return object, before
}

var encrypt = func(file io.Reader, pw *io.PipeWriter, errc chan error) {
	defer pw.Close()
	c4ghWriter, err := newCrypt4GHWriter(pw)
	if err != nil {
		errc <- err

		return
	}
	if _, err = io.Copy(c4ghWriter, file); err != nil {
		errc <- err

		return
	}
	c4ghWriter.Close()
	errc <- nil
}

var encryptedSize = func(decryptedSize int64) int64 {
	nBlocks := math.Ceil(float64(decryptedSize) / float64(api.BlockSize))
	bodySize := decryptedSize + int64(nBlocks)*api.MacSize

	return headerSize + bodySize
}

var getFileDetails = func(filename string) (io.ReadCloser, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	fileInfo, _ := file.Stat()

	return file, encryptedSize(fileInfo.Size()), nil
}
