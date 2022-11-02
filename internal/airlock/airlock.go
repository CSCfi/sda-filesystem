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

const keySize = 32

type airlockInfo struct {
	publicKey [keySize]byte
	proxy     string
	project   string
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

func Upload(original_filename, filename, container, journal_number string,
	segment_size_mb uint64, performEncryption bool) error {

	var err error
	var encrypted_file io.Reader
	var encrypted_checksum string
	var encrypted_file_size int64
	logs.Info("Beggining uploading file ", filename)

	if performEncryption {
		logs.Info("Encrypting file ", original_filename)
		encrypted_file, encrypted_checksum, encrypted_file_size, err = getFileDetailsAndEncrypt(original_filename)
	} else {
		encrypted_file, encrypted_checksum, encrypted_file_size, err = getFileDetails(original_filename)
	}

	if err != nil {
		return fmt.Errorf("Failed to get details for file %s: %w", original_filename, err)
	}

	logs.Debugf("File size %v", encrypted_file_size)
	segment_size := segment_size_mb * (1 << 20)
	logs.Debugf("Segment size %v", segment_size)

	// Get total number of segments
	segment_nro := uint64(math.Ceil(float64(encrypted_file_size) / float64(segment_size)))

	url := ai.proxy + "/airlock"

	object, container := ReorderNames(filename, container)
	logs.Info("Uploading object " + object + " to container " + container)

	query := map[string]string{
		"filename":  object,
		"bucket":    container,
		"timestamp": currentTime().Format(time.RFC3339),
	}

	if original_filename != "" {
		_, original_checksum, original_filesize, err := getFileDetails(original_filename)
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
		err = put(url, container, 1, 1, "", encrypted_file, query)
		if err != nil {
			return err
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
			err = put(url, container, int(i+1), int(segment_nro),
				upload_dir, io.LimitReader(encrypted_file, this_segment_size), query)
			if err != nil {
				return err
			}

		}
		logs.Info("Uploading manifest file for object " + object + " to container " +
			container)

		var empty *os.File = nil
		err = put(url, container, -1, -1, upload_dir, empty, query)
		if err != nil {
			return err
		}
	}

	return nil
}

func ReorderNames(filename, directory string) (string, string) {
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

var getFileDetails = func(filename string) (io.Reader, string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, "", 0, err
	}
	defer file.Close() //?

	file_info, _ := file.Stat()
	file_size := file_info.Size()

	hash := md5.New() // #nosec
	tr := io.TeeReader(file, hash)
	return tr, hex.EncodeToString(hash.Sum(nil)), file_size, nil
}

var getFileDetailsAndEncrypt = func(filename string) (io.Reader, string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, "", 0, err
	}
	defer file.Close()

	r, w := io.Pipe()
	c4gh_writer, err := streaming.NewCrypt4GHWriterWithoutPrivateKey(w, ai.publicKey, nil)
	if err != nil {
		return nil, "", 0, err
	}

	bytes_written, err := io.Copy(c4gh_writer, file)
	if err != nil {
		return nil, "", 0, err
	}

	hash := md5.New() // #nosec
	tr := io.TeeReader(r, hash)

	err = c4gh_writer.Close()
	if err != nil {
		return nil, "", 0, err
	}

	return tr, hex.EncodeToString(hash.Sum(nil)), bytes_written, nil
}

var put = func(url, container string, segment_nro, segment_total int,
	upload_dir string, upload_data io.Reader, query map[string]string) error {

	headers := map[string]string{"SDS-Access-Token": api.GetSDSToken(),
		"SDS-Segment":       fmt.Sprintf("%d", segment_nro),
		"SDS-Total-Segment": fmt.Sprintf("%d", segment_total)}

	logs.Debug("Setting header: SDS-Access-Token << redacted >>")
	logs.Debugf("Setting header: SDS-Segment %d", segment_nro)
	logs.Debugf("Setting header: SDS-Total-Segment %d", segment_total)

	if segment_nro == -1 {
		headers["X-Object-Manifest"] = container + "/" + upload_dir
	}

	var bodyBytes []byte
	if err := api.MakeRequest(url, query, headers, upload_data, &bodyBytes); err != nil {
		errStr := "Failed to upload data to container"
		var re *api.RequestError
		if errors.As(err, &re) && string(bodyBytes) != "" {
			return fmt.Errorf("%s %s: %w", errStr, container, errors.New(string(bodyBytes)))
		}
		return fmt.Errorf("%s %s: %w", errStr, container, err)
	}
	return nil
}
