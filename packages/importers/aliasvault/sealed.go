package aliasvault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/dortanes/ravenpass/packages/importers"
)

// sealedDelimiter ends the JSON header of an .avex archive; the payload follows it.
var sealedDelimiter = []byte("\n-----BEGIN ENCRYPTED DATA-----\n")

// Values an .avex header must name; the KDF name may be in any letter case.
const (
	sealedFormat  = "avex"
	sealedVersion = "1.0.0"
	sealedKDF     = "argon2id"
	sealedCipher  = "AES-256-GCM"
)

// Argon2id bounds an .avex header may name; memory is in KiB, at least 8 per lane (RFC 9106).
const (
	minIterations  = 1
	maxIterations  = 10
	minParallelism = 1
	maxParallelism = 16
	memoryPerLane  = 8
	maxMemoryKiB   = 1 << 20
)

// The payload is a nonce, then the ciphertext and its tag, sealed with a 256-bit key.
const (
	keyLength   = 32
	nonceLength = 12
	tagLength   = 16
)

type header struct {
	Format     string     `json:"format"`
	Version    string     `json:"version"`
	KDF        kdf        `json:"kdf"`
	Encryption encryption `json:"encryption"`
}

// kdf names the key derivation; the salt is Base64 text used as-is, never decoded.
type kdf struct {
	Type   string    `json:"type"`
	Salt   string    `json:"salt"`
	Params kdfParams `json:"params"`
}

// kdfParams are the Argon2id parameters; MemorySize is in KiB.
type kdfParams struct {
	DegreeOfParallelism int `json:"DegreeOfParallelism"`
	MemorySize          int `json:"MemorySize"`
	Iterations          int `json:"Iterations"`
}

type encryption struct {
	Algorithm string `json:"algorithm"`
}

func (p kdfParams) supported() bool {
	return p.Iterations >= minIterations && p.Iterations <= maxIterations &&
		p.DegreeOfParallelism >= minParallelism && p.DegreeOfParallelism <= maxParallelism &&
		p.MemorySize >= memoryPerLane*p.DegreeOfParallelism && p.MemorySize <= maxMemoryKiB
}

// sealed is an .avex archive awaiting its password.
type sealed struct {
	salt    string
	params  kdfParams
	payload []byte
}

// openSealed validates an .avex header before any password is asked for.
func openSealed(head, payload []byte) (*importers.ExportFile, error) {
	var written header
	if err := json.Unmarshal(head, &written); err != nil {
		return nil, importers.Unrecognized(err)
	}
	switch {
	case written.Format != sealedFormat:
		return nil, importers.ErrUnrecognized
	case written.Version != sealedVersion || !strings.EqualFold(written.KDF.Type, sealedKDF) || !written.KDF.Params.supported():
		return nil, importers.ErrUnsupportedEncryption
	case importers.Blank(written.KDF.Salt):
		return nil, importers.ErrUnrecognized
	case written.Encryption.Algorithm != sealedCipher:
		return nil, importers.ErrUnsupportedEncryption
	case len(payload) < nonceLength+tagLength:
		return nil, importers.ErrUnrecognized
	}
	locked := &sealed{salt: written.KDF.Salt, params: written.KDF.Params, payload: payload}
	return importers.NewLockedExportFile(importers.FormatEncryptedZIP, locked, 0), nil
}

// Unseal opens the .avux archive the .avex archive holds and clears the decrypted bytes.
func (s *sealed) Unseal(password string) (*importers.ExportFile, error) {
	archive, err := s.open(password)
	if err != nil {
		return nil, err
	}
	defer clear(archive)
	return openArchive(bytes.NewReader(archive), int64(len(archive)))
}

// open decrypts with AES-256-GCM and no associated data; a failed tag, wrong password or tampering alike, is ErrWrongPassword.
func (s *sealed) open(password string) ([]byte, error) {
	secret := []byte(password)
	key := argon2.IDKey(secret, []byte(s.salt), uint32(s.params.Iterations), uint32(s.params.MemorySize), uint8(s.params.DegreeOfParallelism), keyLength)
	clear(secret)
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, importers.Unsupported(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, importers.Unsupported(err)
	}
	archive, err := aead.Open(nil, s.payload[:nonceLength], s.payload[nonceLength:], nil)
	if err != nil {
		return nil, importers.ErrWrongPassword
	}
	return archive, nil
}
