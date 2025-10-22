package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"sda-filesystem/internal/logs"

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

// GetHeaders gets all the file headers for the buckets, including headers for shared buckets.
var GetHeaders = func(
	rep Repo,
	buckets []Metadata,
	sharedBuckets map[string]SharedBucketsMeta,
) (BatchHeaders, error) {
	headers, err := getProjectHeaders(rep, "", MetadataSlice(buckets))
	if err != nil {
		return nil, fmt.Errorf("failed to get headers for %s: %w", rep, err)
	}
	for project := range sharedBuckets {
		logs.Debugf("Fetching headers for %s", project)
		sharedHeaders, err := getProjectHeaders(rep, project, sharedBuckets[project])
		if err != nil {
			logs.Errorf("Failed to get headers for %s: %w", project, err)

			continue
		}

		maps.Copy(headers, sharedHeaders)
	}

	return headers, nil
}

var getProjectHeaders = func(rep Repo, project string, buckets Named) (BatchHeaders, error) {
	batch := map[string][]string{}
	for name := range buckets.GetNames() {
		batch[name] = []string{"**"} // Get all headers from bucket
	}

	logInsert := ""
	if project != "" {
		logInsert = " shared project (" + project + ")"
	}

	if len(batch) == 0 {
		logs.Debugf("No buckets to fetch headers for in %s%s", rep, logInsert)

		return BatchHeaders{}, nil
	}

	var err error
	var batchJSON []byte
	if rep == SDConnect {
		batchJSON, err = json.Marshal(batch)
	} else {
		batchJSON, err = json.Marshal(slices.Collect(maps.Keys(batch)))
	}
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

	query := make(map[string]string)
	if project != "" {
		query["owner"] = project
	}

	// Whitelist public key with which the headers will be reencrypted
	if err := whitelistKey(query); err != nil {
		return nil, fmt.Errorf("failed to whitelist public key: %w", err)
	}
	defer func() {
		if err := deleteWhitelistedKey(query); err != nil {
			logs.Warningf("Could not delete whitelisted key %s for %s%s: %w", ai.vi.keyName, rep, logInsert, err)
		}
	}()

	var resp vaultResponse
	err = makeRequest("GET", ai.hi.endpoints.Vault.Headers, query, nil, strings.NewReader(body), &resp)
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

	return resp.Data, nil
}

var whitelistKey = func(query map[string]string) error {
	body := `{
		"flavor": "crypt4gh",
		"pubkey": "%s"
	}`
	body = fmt.Sprintf(body, ai.vi.publicKey)
	path := ai.hi.endpoints.Vault.Whitelist + vaultService + "/" + ai.vi.keyName

	return makeRequest("POST", path, query, nil, strings.NewReader(body), nil)
}

var deleteWhitelistedKey = func(query map[string]string) error {
	path := ai.hi.endpoints.Vault.Whitelist + vaultService + "/" + ai.vi.keyName

	return makeRequest("DELETE", path, query, nil, nil, nil)
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
	path := ai.hi.endpoints.Vault.Headers + "/" + bucket

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
