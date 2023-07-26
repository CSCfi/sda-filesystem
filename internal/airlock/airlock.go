package airlock

import (
	"bytes"
	"crypto/md5" // #nosec (Can't be helped at the moment)
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"

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
	var encryptedFile *os.File
	var encryptedChecksum string
	var encryptedFileSize int64

	defer func() {
		if encryptedFile != nil {
			encryptedFile.Close()
			if !encrypted {
				os.Remove(encryptedFile.Name())
			}
		}
	}()

	if !encrypted {
		logs.Info("Encrypting file ", filename)
		encryptedFile, encryptedChecksum, encryptedFileSize, err = getFileDetailsEncrypt(filename)
	} else {
		logs.Info("File ", filename, " is already encrypted. Skipping encryption.")
		encryptedFile, encryptedChecksum, encryptedFileSize, err = getFileDetails(filename)
	}

	if err != nil {
		return fmt.Errorf("Failed to get details for file %s: %w", filename, err)
	}

	if !encrypted {
		filename += ".c4gh"
	}

	logs.Debugf("File size %v", encryptedFileSize)
	segmentSize := segmentSizeMb * uint64(minimumSegmentSize)
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

	if originalFilename != "" {
		file, originalChecksum, originalFilesize, err := getFileDetails(originalFilename)
		if file != nil {
			file.Close()
		}
		if err != nil {
			return fmt.Errorf("Failed to get details for file %s: %w", originalFilename, err)
		}

		query["filesize"] = strconv.FormatInt(originalFilesize, 10)
		query["checksum"] = originalChecksum
		query["encfilesize"] = strconv.FormatInt(encryptedFileSize, 10)
		query["encchecksum"] = encryptedChecksum
	}

	if journalNumber != "" {
		query["journal"] = journalNumber
	}

	// If number of segments is 1, do regular upload, else upload file in segments
	if segmentNro < 2 {
		err = put("", 1, 1, encryptedFile, query)
		if err != nil {
			return fmt.Errorf("Uploading file %s failed: %w", filepath.Base(filename), err)
		}
	} else {
		uploadDir := ".segments/" + object + "/"

		for i := uint64(0); i < segmentNro; i++ {
			segmentStart := int64(float64(i * segmentSize))
			segmentEnd := int64(math.Min(float64(encryptedFileSize),
				float64((i+1)*segmentSize)))

			thisSegmentSize := segmentEnd - segmentStart
			logs.Debugf("Segment start %v", segmentStart)
			logs.Debugf("Segment end %v", segmentEnd)

			logs.Infof("Uploading segment %v/%v", i+1, segmentNro)

			// Send thisSegmentSize number of bytes to airlock
			err = put(container+"/"+uploadDir, int(i+1), int(segmentNro),
				io.LimitReader(encryptedFile, thisSegmentSize), query)
			if err != nil {
				return fmt.Errorf("Uploading file %s failed: %w", filepath.Base(filename), err)
			}

		}
		logs.Info("Uploading manifest file")

		var empty *os.File
		err = put(container+"/"+uploadDir, -1, -1, empty, query)
		if err != nil {
			return fmt.Errorf("Uploading manifest file failed: %w", err)
		}
	}

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

// getFileDetails opens file, calculates its size and checksum, and returns them and the file pointer itself
var getFileDetails = func(filename string) (*os.File, string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, "", 0, err
	}

	fileInfo, _ := file.Stat()
	fileSize := fileInfo.Size()

	hash := md5.New() // #nosec
	_, err = io.Copy(hash, file)
	if err != nil {
		return file, "", 0, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Need to move file pointer to the beginning so that its content can be read again
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return file, "", 0, err
	}

	return file, checksum, fileSize, nil
}

var newCrypt4GHWriter = func(w io.Writer) (io.WriteCloser, error) {
	pubkeyList := [][chacha20poly1305.KeySize]byte{}
	pubkeyList = append(pubkeyList, ai.publicKey)

	return streaming.NewCrypt4GHWriterWithoutPrivateKey(w, pubkeyList, nil)
}

// getFileDetailsEncrypt is similar to getFileDetails() except the file has to be encrypted before it is read
var getFileDetailsEncrypt = func(filename string) (encFile *os.File, checksum string, bytes_written int64, err error) {
	var file *os.File
	if file, err = os.Open(filename); err != nil {
		return
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	fileSize := fileInfo.Size()

	memLeft, err := mountpoint.BytesAvailable(os.TempDir())
	if err != nil {
		return
	}
	if int64(memLeft) < fileSize {
		err = fmt.Errorf("Not enough space for airlock export operation, consider using a volume or splinting the file")

		return
	}

	encFile, err = os.CreateTemp("", "encrypted.*.c4gh")
	if err != nil {
		return
	}

	hash := md5.New() // #nosec
	mwr := io.MultiWriter(encFile, hash)

	c4ghWriter, err := newCrypt4GHWriter(mwr)
	if err != nil {
		return
	}
	if _, err = io.Copy(c4ghWriter, file); err != nil {
		return
	}
	if err = c4ghWriter.Close(); err != nil {
		return
	}

	checksum = hex.EncodeToString(hash.Sum(nil))

	if bytes_written, err = encFile.Seek(0, io.SeekCurrent); err != nil {
		return
	}
	if _, err = encFile.Seek(0, io.SeekStart); err != nil {
		return
	}

	return
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
