package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"sda-filesystem/internal/logs"

	"github.com/neicnordic/crypt4gh/keys"
	"golang.org/x/crypto/chacha20poly1305"
)

const vaultService = "data-gateway"

var whitelistedProjects = make([]string, 0)

type vaultInfo struct {
	privateKey [chacha20poly1305.KeySize]byte
	publicKey  string // base64 encoded
	keyName    string
}

type keyResponse struct {
	Key64 string `json:"public_key_c4gh"`
}

type VaultHeaders struct {
	Headers       map[string]VaultHeader `json:"headers"`
	LatestVersion int                    `json:"latest_version"`
}

type VaultHeader struct {
	Added      string `json:"added"`
	Header     string `json:"header"`
	KeyVersion int    `json:"keyversion"`
}

var GetFileHeader = func(rep Repo, bucket, object, owner, id string) (string, error) {
	ep := ai.hi.endpoints.Vault.Headers
	ep.path += "/" + bucket

	logInsert := ""
	query := map[string]string{
		"object":        object,
		"vault_service": vaultService,
		"key":           ai.vi.keyName,
		"id":            id,
	}
	if owner != "" {
		logInsert = " shared project (" + owner + ")"
		query["owner"] = owner
	}

	if !slices.Contains(whitelistedProjects, owner) {
		logs.Debugf("Whitelisting key for %s%s", rep, logInsert)

		// Whitelist public key with which the headers will be reencrypted
		if err := whitelistKey(owner); err != nil {
			return "", fmt.Errorf("failed to whitelist public key for %s%s: %w", rep, logInsert, err)
		}

		whitelistedProjects = append(whitelistedProjects, owner)
	}

	var resp VaultHeaders
	if err := makeRequest("GET", ep, query, nil, nil, &resp); err != nil {
		var re *RequestError
		if errors.As(err, &re) && re.StatusCode == 404 {
			return "", nil
		}

		return "", err
	}

	return resp.Headers[strconv.Itoa(resp.LatestVersion)].Header, nil
}

var whitelistKey = func(owner string) error {
	body := `{
		"flavor": "crypt4gh",
		"pubkey": "%s"
	}`
	body = fmt.Sprintf(body, ai.vi.publicKey)
	ep := ai.hi.endpoints.Vault.Whitelist
	ep.path += vaultService + "/" + ai.vi.keyName

	query := make(map[string]string)
	if owner != "" {
		query["owner"] = owner
	}

	return makeRequest("POST", ep, query, nil, strings.NewReader(body), nil)
}

var DeleteWhitelistedKeys = func() {
	ep := ai.hi.endpoints.Vault.Whitelist
	ep.path += vaultService + "/" + ai.vi.keyName

	logs.Debug("Deleting whitelisted keys...")

	for _, pr := range whitelistedProjects {
		logInsert := ""
		query := make(map[string]string)
		if pr != "" {
			logInsert = " for " + pr
			query["owner"] = pr
		}

		if err := makeRequest("DELETE", ep, query, nil, nil, nil); err != nil {
			logs.Warningf("Could not delete whitelisted key%s: %w", logInsert, err)
		} else {
			logs.Debugf("Deleted whitelisted key%s", logInsert)
		}
	}

	whitelistedProjects = make([]string, 0)
}

// GetReencryptedHeader is for SD Connect objects that do not have their header in Vault.
// It returns the file's header re-encrypted with filesystem's own public key.
var GetReencryptedHeader = func(bucket, object string) (string, int64, error) {
	ep := ai.hi.endpoints.AllasHeader
	ep.path += bucket

	query := map[string]string{"object": object}
	headers := map[string]string{"Public-Key": ai.vi.publicKey}

	resp := struct {
		Header string `json:"header"`
		Offset int64  `json:"offset"`
	}{}
	err := makeRequest("GET", ep, query, headers, nil, &resp)
	if err != nil {
		var re *RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			ai.sessionExpiredFun()
		}
	}

	return resp.Header, resp.Offset, err
}

// PostHeader sends header of an encrypted object to be stored in Vault (only for SD Connect).
var PostHeader = func(header []byte, bucket, object string) error {
	body := `{
		"header": "%s"
	}`
	body = fmt.Sprintf(body, base64.StdEncoding.EncodeToString(header))
	query := map[string]string{"object": object}

	ep := ai.hi.endpoints.Vault.Headers
	ep.path += "/" + bucket

	return makeRequest("POST", ep, query, nil, strings.NewReader(body), nil)
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
