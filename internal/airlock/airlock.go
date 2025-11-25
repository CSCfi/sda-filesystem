package airlock

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	c4ghHeaders "github.com/neicnordic/crypt4gh/model/headers"
	"github.com/neicnordic/crypt4gh/streaming"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/sync/errgroup"
)

const numRoutines = 4
const minSegmentSize int64 = 1 << 27
const maxParts int64 = 10000
const maxObjectSize int64 = 5 * (1 << 40)
const headerSize = 16 + 108 // Fixed size since we only have one public key

var isLowerAlphaNumericHyphen = regexp.MustCompile(`^[a-z0-9-]+$`).MatchString

var ai = airlockInfo{}

type airlockInfo struct {
	publicKey [chacha20poly1305.KeySize]byte
}

type walkPacket struct {
	file   string
	object string
}

type UploadSet struct {
	Bucket  string   `json:"bucket"`
	Files   []string `json:"files"`
	Objects []string `json:"objects"`
	Exists  []bool   `json:"exists"` // Used only in GUI
}

// ExportPossible indicates whether or not user the user is allowed to export files outside the VM.
// The user must be the project manager and have SD Connect enabled.
var ExportPossible = func() bool {
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

// WalkDirs receives a selection of files and folders, and returns all the files
// that can be found in this selection, including the files recursively found under the folders.
// The function also returns the bucket name and the objects that will be uploaded from the files.
func WalkDirs(selection, currentObjects []string, prefix string) (UploadSet, error) {
	bucket, subfolder, _ := strings.Cut(filepath.Clean(prefix), "/")
	if subfolder != "" {
		subfolder += "/"
	}

	g, ctx := errgroup.WithContext(context.Background())
	files := make([]string, 0, len(selection))
	objects := make([]string, 0, len(selection))
	filesChan := make(chan walkPacket)
	wait := make(chan any)

	go func() {
		for pkt := range filesChan {
			files = append(files, pkt.file)
			objects = append(objects, pkt.object)
		}

		wait <- nil
	}()

	for i := range selection {
		root := filepath.Clean(selection[i])

		g.Go(func() error {
			return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.Type().IsRegular() {
					if !d.IsDir() {
						logs.Warningf("%s is not a regular file or directory, skipping...", path)
					}

					return nil
				}

				var obj string
				if path == root {
					obj = subfolder + filepath.Base(path+".c4gh")
				} else {
					obj = subfolder + strings.TrimPrefix(path, filepath.Dir(root)+"/") + ".c4gh"
				}
				if slices.Contains(currentObjects, obj) {
					return errors.New("you have already selected files with similar object names")
				}

				select {
				case filesChan <- walkPacket{file: path, object: obj}:
				case <-ctx.Done():
					return ctx.Err()
				}

				return nil
			})
		})
	}

	if err := g.Wait(); err != nil {
		return UploadSet{}, err
	}
	close(filesChan)
	<-wait

	sortedObjects := slices.Clone(objects)
	slices.Sort(sortedObjects)

	if len(slices.Compact(sortedObjects)) != len(objects) {
		return UploadSet{}, errors.New("objects derived from the selection of files are not unique")
	}

	return UploadSet{bucket, files, objects, make([]bool, len(objects))}, nil
}

// ValidateBucket validates the bucket name and creates a valid bucket if it does not yet exist
// Called only from CLI
func ValidateBucket(bucket string) (bool, error) {
	if len(bucket) < 3 || len(bucket) > 63 {
		return false, fmt.Errorf("bucket name should be between 3 and 63 characters long")
	}
	if !isLowerAlphaNumericHyphen(bucket) {
		return false, fmt.Errorf("bucket name should only contain Latin letters (a-z), numbers (0-9) and hyphens (-)")
	}
	if bucket[0] == '-' {
		return false, fmt.Errorf("bucket name should start with a lowercase letter or a number")
	}

	exists, err := api.BucketExists(api.SDConnect, bucket)
	if err != nil {
		return false, err
	}
	if !exists {
		logs.Info("Creating bucket ", bucket)
		if err := api.CreateBucket(api.SDConnect, bucket); err != nil {
			return false, err
		}
	}

	return !exists, nil
}

// CheckObjectExistences checks if the files that are to be uploaded already have
// equivalent objects in S3 storage. If any objects exists, user is given a choice to either
// quit or overwrite the objects with the new files. Function assumes bucket exists.
func CheckObjectExistences(set *UploadSet, rd io.Reader) error {
	// these objects should already be sorted
	path := api.SDConnect.ForPath() + "/" + api.GetProjectName() + "/" + set.Bucket
	existingObjects, err := api.GetObjects(api.SDConnect, set.Bucket, path)
	if err != nil {
		return fmt.Errorf("could not determine if export will overwrite data: %w", err)
	}

	for i := range set.Objects {
		_, found := slices.BinarySearchFunc(existingObjects, set.Objects[i], func(meta api.Metadata, obj string) int {
			return strings.Compare(meta.Name, obj)
		})
		if found {
			set.Exists[i] = true
		}
	}

	if rd == nil {
		return nil
	}

	if slices.Contains(set.Exists, true) {
		r := bufio.NewReader(rd)

		for {
			fmt.Fprintf(os.Stdout, "Export will overwrite existing objects in Allas. Continue? [y/n] ")
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

	logs.Info("New objects will not override current objects")

	return nil
}

// Upload uploads files to a bucket with object names taken from the matching index in `objects`
func Upload(set UploadSet, metadata map[string]string) error {
	var err error
	ai.publicKey, err = api.GetPublicKey() // May rotate between uploads so have to fetch it each time
	if err != nil {
		return fmt.Errorf("failed to get project public key: %w", err)
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(numRoutines)
	for i := range set.Objects {
		filename := set.Files[i]
		object := set.Objects[i]
		g.Go(func() error {
			err := UploadObject(ctx, filename, object, set.Bucket, metadata)
			if err != nil {
				logs.Error(err)

				return errors.New("upload interrupted due to errors")
			}

			return nil
		})
	}

	return g.Wait()
}

// UploadObject uploads a file to SD Connect, and possibly to CESSNA
var UploadObject = func(ctx context.Context, filename, object, bucket string, metadata map[string]string) error {
	if err := ctx.Err(); err != nil {
		return nil // We don't need to print out the same context error over and over again
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

	segmentSize := minSegmentSize
	for maxParts*segmentSize < encryptedFileSize {
		segmentSize <<= 1
	}
	logs.Info("Encrypting file ", filename)
	logs.Debugf("Encrypted file size %v for %s", encryptedFileSize, filename)
	logs.Debugf("Segment size %v for %s", segmentSize, filename)

	errc1 := make(chan error, 1)
	errc2 := make(chan error, 2)
	pr, pw := io.Pipe() // So that 'c4ghWriter' can pass its contents to an io.Reader

	if api.GetProjectType() == "default" {
		go encrypt(file, pw, errc2)
	}
	go func() {
		errc1 <- uploadAllas(ctx, pr, bucket, object, segmentSize)
		pr.Close()
	}()

	err = nil
	if api.GetProjectType() != "default" {
		errc2 <- nil
		if err = uploadFindata(ctx, file, pw, bucket, object, segmentSize, metadata); err != nil {
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
		return fmt.Errorf("uploading file %s failed", filename)
	}
	logs.Info("Finished uploading file ", filename)

	return nil
}

// uploadAllas exports an encrypted file by uploading its header to Vault and
// its body to SD Connect. pr is assumed to contain an encrypted file.
var uploadAllas = func(ctx context.Context, pr io.Reader, bucket, object string, segmentSize int64) error {
	logs.Infof("Beginning to upload %s object %s to bucket %s", api.SDConnect, object, bucket)
	header, err := c4ghHeaders.ReadHeader(pr)
	if err != nil {
		return fmt.Errorf("failed to extract header from encrypted file: %w", err)
	}
	logs.Debugf("Uploading header of object %s to Vault", object)
	if err := api.PostHeader(header, bucket, object); err != nil {
		return fmt.Errorf("failed to upload header to vault: %w", err)
	}
	logs.Debugf("Uploading body of object %s to Allas", object)

	return api.UploadObject(ctx, pr, api.SDConnect, bucket, object, segmentSize, nil)
}

// uploadFindata uploads the selected file unencrypted to CESSNA. At the same time
// it sends the read file content through a Crypt4GHWriter, which will be read via the
// pipe reader in uploadAllas(). If upload fails, the pipe writer is closed with an error
// resulting in the AWS SDK to fail in uploadAllas().
func uploadFindata(
	ctx context.Context,
	file io.Reader,
	pw *io.PipeWriter,
	bucket, object string,
	segmentSize int64,
	metadata map[string]string,
) (err error) {
	object = strings.TrimSuffix(object, ".c4gh")
	logs.Infof("Beginning to upload %s object %s/%s", api.Findata, bucket, object)
	defer func() {
		pw.CloseWithError(err) // pw.Close() if err is nil
	}()

	c4ghWriter, err := newCrypt4GHWriter(pw)
	if err != nil {
		return err
	}

	r := io.TeeReader(file, c4ghWriter)
	err = api.UploadObject(ctx, r, api.Findata, bucket, object, segmentSize, metadata)
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

var getFileDetails = func(filename string) (io.ReadCloser, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	fileInfo, _ := file.Stat()

	return file, api.CalculateEncryptedSize(fileInfo.Size()) + headerSize, nil
}
