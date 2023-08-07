package airlock

import (
	"bytes"
	"crypto/md5" // #nosec (Can't be helped at the moment)
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"github.com/neicnordic/crypt4gh/keys"
	"github.com/neicnordic/crypt4gh/model/headers"
	"github.com/neicnordic/crypt4gh/streaming"
	"golang.org/x/crypto/chacha20poly1305"
)

var ai = airlockInfo{}
var infoFile = "/etc/pam_userinfo/config.json"
var minimumSegmentSize = 1 << 20

type airlockInfo struct {
	publicKey  [chacha20poly1305.KeySize]byte
	proxy      string
	project    string
	overridden bool
}

type readCloser struct {
	io.Reader
	io.Closer

	hash hash.Hash
	errc chan error
}

var GetProjectName = func() string {
	return ai.project
}

// IsProjectManager uses file '/etc/pam_userinfo/config.json' and SDS AAI to determine if the user is the
// project manager of the SD Connect project in the current VM
var IsProjectManager = func(project string) (bool, error) {
	errStr := "Could not find user info"

	file, err := os.ReadFile(infoFile)
	if err != nil {
		return false, fmt.Errorf("%s: %w", errStr, err)
	}

	var data map[string]any
	if err = json.Unmarshal(file, &data); err != nil {
		return false, fmt.Errorf("%s: %w", errStr, err)
	}

	endpoint, ok := data["userinfo_endpoint"]
	if !ok {
		err = errors.New("Config file did not contain key 'userinfo_endpoint'")

		return false, fmt.Errorf("Could not determine endpoint for user info: %w", err)
	}

	if project != "" {
		ai.overridden = true
	} else {
		pr, ok := data["login_aud"]
		if !ok {
			err = errors.New("Config file did not contain key 'login_aud'")

			return false, fmt.Errorf("Could not determine to which project this Desktop belongs: %w", err)
		}
		project = fmt.Sprintf("project_%v", pr)
	}

	if err := api.MakeRequest(fmt.Sprintf("%v", endpoint), nil, nil, nil, &data); err != nil {
		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 400 {
			return false, fmt.Errorf("Invalid token")
		}

		return false, err
	}

	projectPI, ok := data["projectPI"]
	if !ok {
		return false, fmt.Errorf("Response body did not contain key 'projectPI'")
	}
	projects := strings.Split(fmt.Sprintf("%v", projectPI), " ")
	for i := range projects {
		if fmt.Sprintf("project_%s", projects[i]) == project {
			ai.project = project

			return true, nil
		}
	}

	return false, nil
}

// GetPublicKey retrieves the public key from the proxy URL and checks its validity
func GetPublicKey() error {
	var err error
	var publicKeySlice []byte

	ai.proxy, err = api.GetEnv("PROXY_URL", true)
	if err != nil {
		return err
	}

	errStr := "Failed to get public key for Airlock"
	url := ai.proxy + "/public-key/crypt4gh.pub"
	err = api.MakeRequest(url, nil, nil, nil, &publicKeySlice)
	if err != nil {
		return fmt.Errorf("%s: %w", errStr, err)
	}

	publicKeyStr := string(publicKeySlice)
	publicKeyStr = strings.TrimSuffix(publicKeyStr, "\n")
	parts := strings.Split(publicKeyStr, "\n")

	if len(parts) != 3 ||
		!strings.HasPrefix(parts[0], "-----BEGIN CRYPT4GH PUBLIC KEY-----") ||
		!strings.HasSuffix(parts[2], "-----END CRYPT4GH PUBLIC KEY-----") {
		return fmt.Errorf("Invalid public key format %q", publicKeyStr)
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("Could not decode public key: %w", err)
	}
	if len(data) < chacha20poly1305.KeySize {
		return fmt.Errorf("Invalid length of decoded public key (%v)", len(data))
	}

	logs.Debugf("Encryption key: %s", publicKeySlice)
	ai.publicKey, err = keys.ReadPublicKey(bytes.NewReader(publicKeySlice))
	if err != nil {
		return fmt.Errorf("%s: %w", errStr, err)
	}

	return nil
}

// Upload uploads a file to SD Connect
func Upload(filename, container string, segmentSizeMb uint64, journalNumber, originalFilename string, encrypted bool) error {
	var err error
	var encryptedRC *readCloser
	var encryptedFileSize int64

	if encrypted {
		logs.Info("File ", filename, " is already encrypted. Skipping encryption.")
	} else {
		logs.Info("Encrypting file ", filename)
	}

	encryptedRC, encryptedFileSize, err = getFileDetails(filename, !encrypted)
	if err != nil {
		return fmt.Errorf("Failed to get details for file %s: %w", filename, err)
	}
	defer encryptedRC.Close()

	if !encrypted {
		filename += ".c4gh"
	}

	segmentSize := segmentSizeMb * uint64(minimumSegmentSize)
	logs.Debugf("File size %v", encryptedFileSize)
	logs.Debugf("Segment size %v", segmentSize)

	// Get total number of segments
	segmentNro := uint64(math.Ceil(float64(encryptedFileSize) / float64(segmentSize)))

	object, container := reorderNames(filename, container)
	logs.Info("Beginning to upload object " + object + " to container " + container)

	query := map[string]string{
		"filename":  object,
		"bucket":    container,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if journalNumber != "" {
		query["journal"] = journalNumber
	}

	uploadDir := container + "_segments/" + object + "/"

	for i := uint64(0); i < segmentNro; i++ {
		segmentStart := int64(float64(i * segmentSize))
		segmentEnd := int64(math.Min(float64(encryptedFileSize),
			float64((i+1)*segmentSize)))

		thisSegmentSize := segmentEnd - segmentStart
		logs.Debugf("Segment start %v", segmentStart)
		logs.Debugf("Segment end %v", segmentEnd)

		logs.Infof("Uploading segment %v/%v", i+1, segmentNro)

		// Send thisSegmentSize number of bytes to airlock
		err = put(uploadDir, int(i+1), int(segmentNro), io.LimitReader(encryptedRC, thisSegmentSize), query)
		if err != nil {
			return fmt.Errorf("Uploading file %s failed: %w", filepath.Base(filename), err)
		}
	}

	logs.Info("Uploading manifest file")

	if originalFilename != "" {
		originalChecksum, originalFilesize, err := getOriginalFileDetails(originalFilename)
		if err != nil {
			return fmt.Errorf("Failed to get details for file %s: %w", originalFilename, err)
		}

		query["filesize"] = strconv.FormatInt(originalFilesize, 10)
		query["checksum"] = originalChecksum
		query["encfilesize"] = strconv.FormatInt(encryptedFileSize, 10)
		query["encchecksum"] = hex.EncodeToString(encryptedRC.hash.Sum(nil))
	}

	var empty *os.File
	err = put(uploadDir, -1, -1, empty, query)
	if err != nil {
		return fmt.Errorf("Uploading manifest file failed: %w", err)
	}

	if encryptedRC.errc != nil {
		if err = <-encryptedRC.errc; err != nil {
			return fmt.Errorf("Streaming file failed: %w", err)
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

// CheckEncryption checks if the file is encrypted
var CheckEncryption = func(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()

	_, err = headers.ReadHeader(file)

	return err == nil, nil
}

var newCrypt4GHWriter = func(w io.Writer) (io.WriteCloser, error) {
	pubkeyList := [][chacha20poly1305.KeySize]byte{}
	pubkeyList = append(pubkeyList, ai.publicKey)

	return streaming.NewCrypt4GHWriterWithoutPrivateKey(w, pubkeyList, nil)
}

var encrypt = func(file *os.File, pw *io.PipeWriter, errc chan error) {
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
	errc <- c4ghWriter.Close()
}

var encryptedSize = func(decryptedSize int64) int64 {
	var magicNumber int64 = 16
	var blockSize int64 = 65536
	var macSize int64 = 28
	var keySize int64 = 108
	var nKeys int64 = 1

	nBlocks := math.Ceil(float64(decryptedSize) / float64(blockSize))
	bodySize := decryptedSize + int64(nBlocks)*macSize
	headerSize := int64(magicNumber + nKeys*keySize)

	return headerSize + bodySize
}

var getFileDetails = func(filename string, performEncryption bool) (*readCloser, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	fileSize := fileInfo.Size()

	var source io.Reader = file
	var errc chan error = nil
	if performEncryption {
		errc = make(chan error, 1) // This error will be checked at the end of Upload()
		pr, pw := io.Pipe()        // So that 'c4ghWriter' can pass its contents to an io.Reader
		go encrypt(file, pw, errc) // Writing from file to 'c4ghWriter' will not happen until 'pr' is being read. Code will hang without goroutine.
		source = pr
		fileSize = encryptedSize(fileSize)
	}

	hash := md5.New() // #nosec
	tr := io.TeeReader(source, hash)

	return &readCloser{tr, file, hash, errc}, fileSize, nil
}

var getOriginalFileDetails = func(filename string) (string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := md5.New() // #nosec
	bytes_written, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(hash.Sum(nil)), bytes_written, nil
}

var put = func(manifest string, segmentNro, segment_total int,
	upload_data io.Reader, query map[string]string) error {

	headers := map[string]string{"SDS-Access-Token": api.GetSDSToken(),
		"SDS-Segment":       fmt.Sprintf("%d", segmentNro),
		"SDS-Total-Segment": fmt.Sprintf("%d", segment_total),
	}

	logs.Debug("Setting header: SDS-Access-Token << redacted >>")
	logs.Debugf("Setting header: SDS-Segment %d", segmentNro)
	logs.Debugf("Setting header: SDS-Total-Segment %d", segment_total)

	if segmentNro == -1 {
		headers["X-Object-Manifest"] = manifest
	}

	if ai.overridden {
		headers["Project-Name"] = ai.project
	}

	var bodyBytes []byte
	url := ai.proxy + "/airlock"
	if err := api.MakeRequest(url, query, headers, upload_data, &bodyBytes); err != nil {
		var re *api.RequestError
		if errors.As(err, &re) && string(bodyBytes) != "" {
			return errors.New(string(bodyBytes))
		}

		return err
	}

	return nil
}
