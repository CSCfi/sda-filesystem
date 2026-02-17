package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
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

type vaultBatchResponse struct {
	Data     BatchHeaderVersions `json:"data"`
	Warnings []string            `json:"warnings"`
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

type BatchHeaderVersions map[string]map[string]int

// GetHeaders gets all the file headers for the buckets, including headers for shared buckets.
var GetHeaderVersions = func(
	rep Repo,
	buckets []Metadata,
	sharedBuckets map[string]SharedBucketsMeta,
) (BatchHeaderVersions, error) {
	headers, err := getProjectHeaderVersions(rep, "", MetadataSlice(buckets))
	if err != nil {
		return nil, fmt.Errorf("failed to get header versions for %s: %w", rep, err)
	}

	for project := range sharedBuckets {
		logs.Debugf("Fetching headers for %s", project)
		sharedHeaders, err := getProjectHeaderVersions(rep, project, sharedBuckets[project])
		if err != nil {
			logs.Errorf("Failed to get header versions for %s: %w", project, err)

			continue
		}

		maps.Copy(headers, sharedHeaders)
	}

	return headers, nil
}

var getProjectHeaderVersions = func(rep Repo, project string, buckets Named) (BatchHeaderVersions, error) {
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

		return BatchHeaderVersions{}, nil
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

	body := `{"batch": "%s", "versions": true}`
	body = fmt.Sprintf(body, batchString)

	query := map[string]string{}
	if project != "" {
		query["owner"] = project
	}

	var resp vaultBatchResponse
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
			logs.Warningf("%s", warnings[i])
		}
	}

	return resp.Data, nil
}

var GetFileHeader = func(rep Repo, bucket, object, owner, id string, version int, path string) (string, error) {
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
		if err := whitelistKey(query); err != nil {
			return "", fmt.Errorf("failed to whitelist public key for %s%s: %w", rep, logInsert, err)
		}

		whitelistedProjects = append(whitelistedProjects, owner)
	}

	var resp VaultHeaders
	err := makeRequest("GET", ep, query, nil, nil, &resp)

	if resp.LatestVersion > version {
		logs.Warningf("File %s has a header version that is higher than expected which suggests the file has been modified after the creation of Data Gateway. You may need to refresh Data Gateway to be able to download the file", path)
	}

	return resp.Headers[strconv.Itoa(version)].Header, err
}

var whitelistKey = func(query map[string]string) error {
	body := `{
		"flavor": "crypt4gh",
		"pubkey": "%s"
	}`
	body = fmt.Sprintf(body, ai.vi.publicKey)
	ep := ai.hi.endpoints.Vault.Whitelist
	ep.path += vaultService + "/" + ai.vi.keyName

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
