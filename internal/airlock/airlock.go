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

	"github.com/neicnordic/crypt4gh/keys"
	"github.com/neicnordic/crypt4gh/model/headers"
	"github.com/neicnordic/crypt4gh/streaming"
)

var ai airlockInfo = airlockInfo{}
var infoFile = "/etc/pam_userinfo/config.json"
var minimumSegmentSize = 1 << 20

const keySize = 32

type airlockInfo struct {
	publicKey [keySize]byte
	proxy     string
	project   string
}

type readCloser struct {
	io.Reader
	io.Closer

	errc chan error
}

var GetProjectName = func() string {
	return ai.project
}

var currentTime = func() time.Time {
	return time.Now()
}

var IsProjectManager = func() (bool, error) {
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

	pr, ok := data["login_aud"]
	if !ok {
		err = errors.New("Config file did not contain key 'login_aud'")
		return false, fmt.Errorf("Could not determine to which project this Desktop belongs: %w", err)
	}

	if err := api.MakeRequest(fmt.Sprintf("%v", endpoint), nil, nil, nil, &data); err != nil {
		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 400 {
			return false, fmt.Errorf("Invalid token")
		} else {
			return false, err
		}
	}

	if projectPI, ok := data["projectPI"]; !ok {
		return false, fmt.Errorf("Response body did not contain key 'projectPI'")
	} else {
		projects := strings.Split(fmt.Sprintf("%v", projectPI), " ")
		for i := range projects {
			if projects[i] == fmt.Sprintf("%v", pr) {
				ai.project = fmt.Sprintf("project_%v", pr)
				return true, nil
			}
		}
		return false, nil
	}
}

func GetPublicKey() error {
	var err error
	var public_key_slice []byte

	ai.proxy, err = api.GetEnv("PROXY_URL", true)
	if err != nil {
		return err
	}

	errStr := "Failed to get public key for Airlock"
	url := ai.proxy + "/public-key/crypt4gh.pub"
	err = api.MakeRequest(url, nil, nil, nil, &public_key_slice)
	if err != nil {
		return fmt.Errorf("%s: %w", errStr, err)
	}

	public_key_str := string(public_key_slice)
	public_key_str = strings.TrimSuffix(public_key_str, "\n")
	parts := strings.Split(public_key_str, "\n")

	if len(parts) != 3 ||
		!strings.HasPrefix(parts[0], "-----BEGIN CRYPT4GH PUBLIC KEY-----") ||
		!strings.HasSuffix(parts[2], "-----END CRYPT4GH PUBLIC KEY-----") {
		return fmt.Errorf("Invalid public key format %q", public_key_str)
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("Could not decode public key: %w", err)
	}
	if len(data) != keySize {
		return fmt.Errorf("Invalid length of decoded public key (%v)", len(data))
	}

	logs.Debugf("Encryption key: %s", public_key_slice)
	ai.publicKey, err = keys.ReadPublicKey(bytes.NewReader(public_key_slice))
	if err != nil {
		return fmt.Errorf("%s: %w", errStr, err)
	}
	return nil
}

func Upload(original_filename, filename, container, journal_number string, segment_size_mb uint64) error {
	var err error
	var encrypted_file io.ReadCloser
	var encrypted_checksum string
	var encrypted_file_size int64

	if encrypted, err := CheckEncryption(filename); err != nil {
		return err
	} else if encrypted {
		logs.Info("Encrypting file ", original_filename)
		encrypted_file, encrypted_checksum, encrypted_file_size, err = getFileDetailsAndEncrypt(original_filename)
	} else {
		logs.Info("File ", filename, " is already encrypted. Skipping encryption.")
		encrypted_file, encrypted_checksum, encrypted_file_size, err = getFileDetails(filename)
	}

	if err != nil {
		return fmt.Errorf("Failed to get details for file: %w", err)
	}
	defer encrypted_file.Close()

	logs.Debugf("File size %v", encrypted_file_size)
	segment_size := segment_size_mb * uint64(minimumSegmentSize)
	logs.Debugf("Segment size %v", segment_size)

	// Get total number of segments
	segment_nro := uint64(math.Ceil(float64(encrypted_file_size) / float64(segment_size)))

	url := ai.proxy + "/airlock"

	object, container := reorderNames(filename, container)
	logs.Info("Uploading object " + object + " to container " + container)

	query := map[string]string{
		"filename":  object,
		"bucket":    container,
		"timestamp": currentTime().Format(time.RFC3339),
	}

	if original_filename != "" {
		rc, original_checksum, original_filesize, err := getFileDetails(original_filename)
		rc.Close()
		if err != nil {
			return fmt.Errorf("Failed to get details for file %s: %w", original_filename, err)
		}

		query["filesize"] = strconv.FormatInt(original_filesize, 10)
		query["checksum"] = original_checksum
		query["encfilesize"] = strconv.FormatInt(encrypted_file_size, 10)
		query["encchecksum"] = encrypted_checksum
	}

	if journal_number != "" {
		query["journal"] = journal_number
	}

	// If number of segments is 1, do regular upload, else upload file in segments
	if segment_nro < 2 {
		err = put(url, "", 1, 1, encrypted_file, query)
		if err != nil {
			return fmt.Errorf("Uploading file failed: %w", err)
		}
	} else {
		upload_dir := ".segments/" + object + "/"

		for i := uint64(0); i < segment_nro; i++ {
			segment_start := int64(float64(i * segment_size))
			segment_end := int64(math.Min(float64(encrypted_file_size),
				float64((i+1)*segment_size)))

			this_segment_size := segment_end - segment_start
			logs.Debugf("Segment start %v", segment_start)
			logs.Debugf("Segment end %v", segment_end)

			logs.Infof("Uploading segment %v/%v", i+1, segment_nro)

			// Send this_segment_size number of bytes to airlock
			err = put(url, container+"/"+upload_dir, int(i+1), int(segment_nro),
				io.LimitReader(encrypted_file, this_segment_size), query)
			if err != nil {
				return fmt.Errorf("Uploading file failed: %w", err)
			}

		}
		logs.Info("Uploading manifest file for object " + object + " to container " +
			container)

		var empty *os.File = nil
		err = put(url, container+"/"+upload_dir, -1, -1, empty, query)
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

func CheckEncryption(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, fmt.Errorf("Failed to check if file is encrypted: %w", err)
	}
	defer file.Close()

	_, err = headers.ReadHeader(file)
	return err == nil, nil
}

var getFileDetails = func(filename string) (io.ReadCloser, string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, "", 0, err
	}

	file_info, _ := file.Stat()
	file_size := file_info.Size()

	hash := md5.New() // #nosec
	tr := io.TeeReader(file, hash)
	return readCloser{tr, file, nil}, hex.EncodeToString(hash.Sum(nil)), file_size, nil
}

func encrypt(file *os.File, pw io.WriteCloser, errc chan error) {
	defer pw.Close()
	c4gh_writer, err := streaming.NewCrypt4GHWriterWithoutPrivateKey(pw, ai.publicKey, nil)
	if err != nil {
		errc <- err
		return
	}
	if _, err = io.Copy(c4gh_writer, file); err != nil {
		errc <- err
		return
	}
	errc <- c4gh_writer.Close()
}

var getFileDetailsAndEncrypt = func(filename string) (rc io.ReadCloser, checksum string, bytes_written int64, err error) {
	var file *os.File
	if file, err = os.Open(filename); err != nil {
		return
	}

	errc := make(chan error, 1)
	pr, pw := io.Pipe()
	go encrypt(file, pw, errc)

	hash := md5.New() // #nosec
	if bytes_written, err = io.Copy(hash, pr); err != nil {
		return
	}
	checksum = hex.EncodeToString(hash.Sum(nil))
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return
	}
	if err = <-errc; err != nil {
		return
	}

	errc = make(chan error, 1)
	pr, pw = io.Pipe()
	go encrypt(file, pw, errc)

	rc = readCloser{pr, file, errc}
	return
}

var put = func(url, manifest string, segment_nro, segment_total int,
	upload_data io.Reader, query map[string]string) error {

	headers := map[string]string{"SDS-Access-Token": api.GetSDSToken(),
		"SDS-Segment":       fmt.Sprintf("%d", segment_nro),
		"SDS-Total-Segment": fmt.Sprintf("%d", segment_total)}

	logs.Debug("Setting header: SDS-Access-Token << redacted >>")
	logs.Debugf("Setting header: SDS-Segment %d", segment_nro)
	logs.Debugf("Setting header: SDS-Total-Segment %d", segment_total)

	if segment_nro == -1 {
		headers["X-Object-Manifest"] = manifest
	}

	var bodyBytes []byte
	if err := api.MakeRequest(url, query, headers, upload_data, &bodyBytes); err != nil {
		var re *api.RequestError
		if errors.As(err, &re) && string(bodyBytes) != "" {
			return errors.New(string(bodyBytes))
		}
		return err
	}
	return nil
}
