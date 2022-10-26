package airlock

import (
	"bufio"
	"bytes"
	"crypto/md5"
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

//timeout

var ai airlockInfo = airlockInfo{}

type airlockInfo struct {
	publicKey [32]byte
	proxy     string
	project   string
}

var IsProjectManager = func() (bool, error) {
	errStr := "Could not find user info"

	file, err := os.ReadFile("/etc/pam_userinfo/config.json")
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

	url := ai.proxy + "/public-key/crypt4gh.pub"
	err = api.MakeRequest(url, nil, nil, nil, &public_key_slice)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(string(public_key_slice),
		"-----BEGIN CRYPT4GH PUBLIC KEY-----") {
		return fmt.Errorf("Invalid public key %s", string(public_key_slice))
	}

	logs.Debugf("Encryption key: %s", public_key_slice)
	ai.publicKey, err = keys.ReadPublicKey(bytes.NewReader(public_key_slice))
	return err
}

func Upload(original_filename, filename, container, journal_number string,
	segment_size_mb uint64, force bool) error {

	if encrypted, err := checkEncryption(filename); err != nil {
		return err
	} else if encrypted {
		logs.Info("File ", filename, " is already encrypted. Skipping encryption.")
	} else {
		original_filename = filename
		if filename, err = encrypt(filename, force); err != nil {
			return err
		}
	}

	encrypted_checksum, encrypted_file_size, err := getFileDetails(filename)
	if err != nil {
		return err
	}

	logs.Debugf("File size %v", encrypted_file_size)
	segment_size := segment_size_mb * (1 << 20)
	logs.Debugf("Segment size %v", segment_size)

	// Get total number of segments
	segment_nro := uint64(math.Ceil(float64(encrypted_file_size) / float64(segment_size)))

	url := ai.proxy + "/airlock"

	logs.Info("Uploading file " + filename + " to container " + container)

	query := map[string]string{
		"filename":  filepath.Base(filename),
		"bucket":    container,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if original_filename != "" {
		original_checksum, original_filesize, err := getFileDetails(original_filename)
		if err != nil {
			return err
		}

		query["filesize"] = strconv.FormatInt(original_filesize, 10)
		query["checksum"] = original_checksum
		query["encfilesize"] = strconv.FormatInt(encrypted_file_size, 10)
		query["encchecksum"] = encrypted_checksum
	}

	if journal_number != "" {
		query["journal"] = journal_number
	}

	encrypted_file, err := os.Open(filename)
	if err != nil {
		return err
	}

	// If number of segments is 1, do regular upload, else upload file in segments
	if segment_nro < 2 {
		put(url, container, 1, 1, "", encrypted_file, query)
	} else {
		upload_dir := ".segments/" + filename + "/"

		for i := uint64(0); i < segment_nro; i++ {
			segment_start := int64(float64(i * segment_size))
			segment_end := int64(math.Min(float64(encrypted_file_size),
				float64((i+1)*segment_size)))

			this_segment_size := segment_end - segment_start
			logs.Debugf("Segment start %v", segment_start)
			logs.Debugf("Segment end %v", segment_end)

			// Move io reader pointer to correct location on orignal file
			encrypted_file.Seek(segment_start, 0)

			logs.Infof("Uploading segment %v/%v\n", i+1, segment_nro)

			// Send this_segment_size number of bytes to airlock
			put(url, container, int(i+1), int(segment_nro),
				upload_dir, io.LimitReader(encrypted_file, this_segment_size), query)

		}
		logs.Info("Uploading manifest file for " + filename + " to container " +
			container)

		put(url, container, -1, -1, upload_dir, nil, query)
	}
	encrypted_file.Close()
	return nil
}

func getFileDetails(filename string) (string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", 0, err
	}

	file_info, _ := file.Stat()
	file_size := file_info.Size()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}

	file.Close()
	return hex.EncodeToString(hash.Sum(nil)), file_size, nil
}

var separateHeader = func(reader io.Reader) ([]byte, error) {
	header, err := headers.ReadHeader(reader)
	return header, err
}

func checkEncryption(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()

	var reader io.Reader = file
	_, err = separateHeader(reader)
	if err != nil {
		return false, err
	}
	return true, nil
}

func encrypt(in_filename string, force bool) (string, error) {
	logs.Infof("Encrypting file %q", in_filename)

	out_filename := in_filename + ".c4gh"
	// Ask user confirmation if output file exists
	if !force {
		askOverwrite(out_filename, "File "+out_filename+" exists. Overwrite file")
	}

	in_file, err := os.Open(in_filename)
	if err != nil {
		return "", err
	}

	out_file, err := os.Create(out_filename)
	if err != nil {
		return "", err
	}

	c4gh_writer, err := streaming.NewCrypt4GHWriterWithoutPrivateKey(out_file, ai.publicKey, nil)
	if err != nil {
		return "", err
	}

	bytes_written, err := io.Copy(c4gh_writer, in_file)
	if err != nil {
		return "", err
	}

	err = in_file.Close()
	if err != nil {
		return "", err
	}

	err = c4gh_writer.Close()
	if err != nil {
		return "", err
	}

	err = out_file.Close()
	if err != nil {
		return "", err
	}

	logs.Info(bytes_written, "bytes encrypted to file:", out_filename, "\n")
	return out_filename, nil
}

func put(url, container string, segment_nro, segment_total int,
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
	api.MakeRequest(url, query, headers, upload_data, &bodyBytes)
	fmt.Print(string(bodyBytes))
	//fmt.Errorf("Failed to upload data: %w", errors.New(string(bodyBytes)))
	return nil
}

func askOverwrite(filename string, message string) error {
	if _, err := os.Stat(filename); err == nil {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(message, " [y/N]?")

		response, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			logs.Info("Not overwriting. Exiting.")
			os.Exit(0)
		}
	}
	return nil
}
