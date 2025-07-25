package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sda-filesystem/internal/logs"
	"strings"

	"github.com/neicnordic/crypt4gh/keys"
	"golang.org/x/crypto/chacha20poly1305"
)

const vaultService = "data-gateway"

type vaultInfo struct {
	privateKey [chacha20poly1305.KeySize]byte
	publicKey  string // base64 encoded
	keyName    string
}

type vaultResponse struct {
	Data     BatchHeaders `json:"data"`
	Warnings []string     `json:"warnings"`
}

type keyResponse struct {
	Key64 string `json:"public_key_c4gh"`
}

type VaultHeaderVersions struct {
	Headers       map[string]VaultHeader `json:"headers"`
	LatestVersion int                    `json:"latest_version"`
}

type VaultHeader struct {
	Added      string `json:"added"`
	Header     string `json:"header"`
	KeyVersion int    `json:"keyversion"`
}

type BatchHeaders map[string]map[string]VaultHeaderVersions
type Headers map[string]map[string]string

// GetHeaders gets all the file headers from header storage for bucket
var GetHeaders = func(rep string, buckets []Metadata) (BatchHeaders, error) {
	if err := whitelistKey(rep); err != nil {
		return nil, fmt.Errorf("failed to whitelist public key: %w", err)
	}

	batchMap := map[string][]string{}
	for i := range buckets {
		batchMap[buckets[i].Name] = []string{"**"} // Get all headers from bucket
	}
	batchJSON, err := json.Marshal(batchMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch json: %w", err)
	}
	batchString := base64.StdEncoding.EncodeToString(batchJSON)

	body := `{
		"batch": "%s",
		"service": "%s",
		"key": "%s"
	}`
	body = fmt.Sprintf(body, batchString, vaultService, ai.vi.keyName)

	var resp vaultResponse
	err = makeRequest("GET", ai.hi.endpoints.Vault.Headers, nil, nil, strings.NewReader(body), &resp)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	warnings := []string{}
	for i := range resp.Warnings {
		if strings.HasPrefix(resp.Warnings[i], "No matches found for") {
			logs.Debug(resp.Warnings[i])
		} else {
			warnings = append(warnings, resp.Warnings[i])
		}
	}
	if len(warnings) > 0 {
		logs.Warningf("The request for file headers was not entirely successful.")
		for i := range warnings {
			logs.Warningf(warnings[i])
		}
	}

	if err = deleteWhitelistedKey(rep); err != nil {
		logs.Warningf("Could not delete key %s: %w", ai.vi.keyName, err)
	}

	return resp.Data, nil
}

var whitelistKey = func(rep string) error {
	_ = rep // Once SD Apply uses S3 and can be integrated here, this variable will become useful
	body := `{
		"flavor": "crypt4gh",
		"pubkey": "%s"
	}`
	body = fmt.Sprintf(body, ai.vi.publicKey)
	path := ai.hi.endpoints.Vault.Whitelist + vaultService + "/" + ai.vi.keyName

	return makeRequest("POST", path, nil, nil, strings.NewReader(body), nil)
}

var deleteWhitelistedKey = func(rep string) error {
	_ = rep // Once SD Apply uses S3 and can be integrated here, this variable will become useful
	path := ai.hi.endpoints.Vault.Whitelist + vaultService + "/" + ai.vi.keyName

	return makeRequest("DELETE", path, nil, nil, nil, nil)
}

// GetReencryptedHeader is for SD Connect objects that do not have their header in Vault.
// It returns the file's header re-encrypted with filesystem's own public key.
var GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
	path := ai.hi.endpoints.AllasHeader + bucket
	query := map[string]string{"object": object}
	headers := map[string]string{"Public-Key": ai.vi.publicKey}

	resp := struct {
		Header string `json:"header"`
		Offset int64  `json:"offset"`
	}{}
	err := makeRequest("GET", path, query, headers, nil, &resp)

	return resp.Header, resp.Offset, err
}

// PostHeader sends header of an encrypted object to be stored in Vault.
var PostHeader = func(header []byte, bucket, object string) error {
	body := `{
		"header": "%s"
	}`
	body = fmt.Sprintf(body, base64.StdEncoding.EncodeToString(header))
	query := map[string]string{"object": object}
	path := fmt.Sprintf("%s/%s", ai.hi.endpoints.Vault.Headers, bucket)

	return makeRequest("POST", path, query, nil, strings.NewReader(body), nil)
}

var GetPublicKey = func() ([32]byte, error) {
	var encryptionKey keyResponse
	err := makeRequest("GET", ai.hi.endpoints.Vault.Key, nil, nil, nil, &encryptionKey)
	if err != nil {
		return [32]byte{}, err
	}

	logs.Debugf("Encryption key: %s", encryptionKey.Key64)

	return keys.ReadPublicKey(strings.NewReader(encryptionKey.Key64))
}
