package test

import (
	"bytes"
	"io"
	"math/rand"
	"strings"
	"testing"

	"github.com/neicnordic/crypt4gh/keys"
	c4ghHeaders "github.com/neicnordic/crypt4gh/model/headers"
	"github.com/neicnordic/crypt4gh/streaming"
)

func GenerateRandomText(size int) []byte {
	var builder strings.Builder
	builder.Grow(int(size))

	r := rand.New(rand.NewSource(42)) // #nosec G404

	consonants := "bcdfghjklmnpqrstvwxyz"
	vowels := "aeiou"

	generateWord := func(length int) string {
		var b strings.Builder
		useVowel := rand.Intn(2) == 0 // #nosec G404
		for i := 0; i < length; i++ {
			if useVowel {
				b.WriteByte(vowels[r.Intn(len(vowels))]) // #nosec G404
			} else {
				b.WriteByte(consonants[r.Intn(len(consonants))]) // #nosec G404
			}
			useVowel = !useVowel
		}

		return b.String()
	}

	generateSentence := func() string {
		wordCount := rand.Intn(10) + 5 // #nosec G404
		var sentence strings.Builder
		for i := 0; i < wordCount; i++ {
			word := generateWord(r.Intn(6) + 3) // #nosec G404
			if i == 0 {
				word = strings.ToUpper(string(word[0])) + word[1:]
			}
			sentence.WriteString(word)
			if i < wordCount-1 {
				sentence.WriteByte(' ')
			}
		}
		sentence.WriteByte('.')
		sentence.WriteByte(' ')

		return sentence.String()
	}

	for builder.Len() < size {
		builder.WriteString(generateSentence())
	}

	return []byte(builder.String()[:size])
}

func EncryptData(t *testing.T, content []byte) ([]byte, []byte, [32]byte) {
	publicKey, privateKey, err := keys.GenerateKeyPair()
	if err != nil {
		t.Errorf("Failed to generate key pair: %s", err.Error())
	}

	encBuffer := bytes.Buffer{}
	encryptWriter, err := streaming.NewCrypt4GHWriterWithoutPrivateKey(&encBuffer, [][32]byte{publicKey}, nil)
	if err != nil {
		t.Fatalf("Failed to encrypt test data: %s", err.Error())
	}
	num, err := io.Copy(encryptWriter, bytes.NewBuffer(content))
	if err != nil {
		t.Fatalf("Failed to copy content for encryption: %s", err.Error())
	}
	encryptWriter.Close()
	if num != int64(len(content)) {
		t.Fatalf("Data has incorrect number of bytes. Expected=%d, received=%d", len(content), num)
	}

	encReader := bytes.NewReader(encBuffer.Bytes())
	headerBytes, err := c4ghHeaders.ReadHeader(encReader)
	if err != nil {
		t.Fatalf("Failed to read header from data: %s", err.Error())
	}
	encryptedContent, err := io.ReadAll(encReader)
	if err != nil {
		t.Fatalf("Failed to read rest of data: %s", err.Error())
	}

	return headerBytes, encryptedContent, privateKey
}
