package aliasvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"testing"

	"golang.org/x/crypto/argon2"
)

// sealing is the header a test writes an .avex archive under, with Argon2id memory in KiB.
type sealing struct {
	format      string
	version     string
	kdf         string
	salt        string
	algorithm   string
	iterations  int
	memory      int
	parallelism int
}

// fastSealing has cheap Argon2id parameters; its salt is valid Base64 to catch a decoding reader.
var fastSealing = sealing{
	format:      "avex",
	version:     "1.0.0",
	kdf:         "Argon2Id",
	salt:        "c2FsdCBmb3IgdGVzdHM+Pz8/",
	algorithm:   "AES-256-GCM",
	iterations:  1,
	memory:      64,
	parallelism: 1,
}

// seal encrypts archive as AliasVault does: Argon2id over the salt text, then AES-256-GCM.
func (s sealing) seal(t *testing.T, password string, archive []byte) []byte {
	t.Helper()
	key := argon2.IDKey([]byte(password), []byte(s.salt), uint32(s.iterations), uint32(s.memory), uint8(s.parallelism), keyLength)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, nonceLength)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	return s.around(t, aead.Seal(nonce, nonce, archive, nil))
}

// around lays out an .avex file: indented JSON header, delimiter, payload.
func (s sealing) around(t *testing.T, payload []byte) []byte {
	t.Helper()
	head, err := json.MarshalIndent(map[string]any{
		"format":  s.format,
		"version": s.version,
		"kdf": map[string]any{
			"type": s.kdf,
			"salt": s.salt,
			"params": map[string]int{
				"DegreeOfParallelism": s.parallelism,
				"MemorySize":          s.memory,
				"Iterations":          s.iterations,
			},
		},
		"encryption": map[string]any{"algorithm": s.algorithm, "encryptedDataOffset": 0},
		"metadata":   map[string]any{"exportedAt": "2026-03-19T08:59:19.806Z", "exportedBy": "alex@example.test", "appVersion": "0.28.0"},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(append(head, sealedDelimiter...), payload...)
}
