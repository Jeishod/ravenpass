package bitwarden

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/dortanes/ravenpass/packages/importers"
)

// kdf is the kdfType number a password-protected export writes.
type kdf int

const (
	kdfPBKDF2   kdf = 0
	kdfArgon2id kdf = 1
)

// Bitwarden's own ranges for the derivation parameters it writes. Argon2id memory is in MiB.
const (
	minPBKDF2Iterations  = 5_000
	maxPBKDF2Iterations  = 2_000_000
	minArgon2Iterations  = 2
	maxArgon2Iterations  = 10
	minArgon2MemoryMiB   = 16
	maxArgon2MemoryMiB   = 1024
	minArgon2Parallelism = 1
	maxArgon2Parallelism = 16
)

// keyLength is the length of the derived key material and of each key stretched from it.
const keyLength = 32

// encStringType is the EncString type for AES-256-CBC authenticated with HMAC-SHA256.
const encStringType = "2"

var (
	errMACMismatch = errors.New("envelope authentication failed")
	errPadding     = errors.New("envelope padding is malformed")
)

type derivation struct {
	function    kdf
	iterations  int
	memoryMiB   int
	parallelism int
}

// supported checks Bitwarden's own ranges; PBKDF2 ignores any memory and parallelism named.
func (d derivation) supported() bool {
	switch d.function {
	case kdfPBKDF2:
		return between(d.iterations, minPBKDF2Iterations, maxPBKDF2Iterations)
	case kdfArgon2id:
		return between(d.iterations, minArgon2Iterations, maxArgon2Iterations) &&
			between(d.memoryMiB, minArgon2MemoryMiB, maxArgon2MemoryMiB) &&
			between(d.parallelism, minArgon2Parallelism, maxArgon2Parallelism)
	}
	return false
}

func between(value, least, most int) bool {
	return value >= least && value <= most
}

// derive uses the salt text bytes as written; Argon2id takes their SHA-256 as its salt.
func (d derivation) derive(password string, salt []byte) ([]byte, error) {
	if d.function == kdfArgon2id {
		secret := []byte(password)
		defer clear(secret)
		digest := sha256.Sum256(salt)
		return argon2.IDKey(secret, digest[:], uint32(d.iterations), uint32(d.memoryMiB)*1024, uint8(d.parallelism), keyLength), nil
	}
	material, err := pbkdf2.Key(sha256.New, password, salt, d.iterations, keyLength)
	if err != nil {
		return nil, importers.Unsupported(err)
	}
	return material, nil
}

type keys struct {
	encryption     []byte
	authentication []byte
}

// stretch expands key material with HKDF-Expand over SHA-256, with no extract step.
func stretch(material []byte) (keys, error) {
	encryption, err := hkdf.Expand(sha256.New, material, "enc", keyLength)
	if err != nil {
		return keys{}, importers.Unsupported(err)
	}
	authentication, err := hkdf.Expand(sha256.New, material, "mac", keyLength)
	if err != nil {
		clear(encryption)
		return keys{}, importers.Unsupported(err)
	}
	return keys{encryption: encryption, authentication: authentication}, nil
}

func (k keys) wipe() {
	clear(k.encryption)
	clear(k.authentication)
}

// encString is a Bitwarden EncString "2.<iv>|<data>|<mac>", each part in standard Base64.
type encString struct {
	iv   []byte
	data []byte
	mac  []byte
}

func parseEncString(value string) (encString, error) {
	kind, body, found := strings.Cut(value, ".")
	if !found || kind != encStringType {
		return encString{}, importers.ErrUnsupportedEncryption
	}
	parts := strings.Split(body, "|")
	if len(parts) != 3 {
		return encString{}, importers.ErrUnrecognized
	}
	decoded := make([][]byte, len(parts))
	for i, part := range parts {
		var err error
		if decoded[i], err = base64.StdEncoding.DecodeString(part); err != nil {
			return encString{}, importers.Unrecognized(err)
		}
	}
	parsed := encString{iv: decoded[0], data: decoded[1], mac: decoded[2]}
	if len(parsed.iv) != aes.BlockSize || len(parsed.mac) != sha256.Size || len(parsed.data) == 0 || len(parsed.data)%aes.BlockSize != 0 {
		return encString{}, importers.ErrUnrecognized
	}
	return parsed, nil
}

// open verifies the MAC over iv and data in constant time, and only then decrypts.
func (e encString) open(k keys) ([]byte, error) {
	tag := hmac.New(sha256.New, k.authentication)
	tag.Write(e.iv)
	tag.Write(e.data)
	if !hmac.Equal(tag.Sum(nil), e.mac) {
		return nil, errMACMismatch
	}
	block, err := aes.NewCipher(k.encryption)
	if err != nil {
		return nil, importers.Unsupported(err)
	}
	plaintext := make([]byte, len(e.data))
	cipher.NewCBCDecrypter(block, e.iv).CryptBlocks(plaintext, e.data)
	unpadded, err := unpad(plaintext)
	if err != nil {
		clear(plaintext)
		return nil, err
	}
	return unpadded, nil
}

// unpad strips PKCS#7 padding, RFC 5652 section 6.3.
func unpad(padded []byte) ([]byte, error) {
	length := int(padded[len(padded)-1])
	if length == 0 || length > aes.BlockSize {
		return nil, errPadding
	}
	for _, value := range padded[len(padded)-length:] {
		if int(value) != length {
			return nil, errPadding
		}
	}
	return padded[:len(padded)-length], nil
}

// sealed is a password-protected export awaiting its password.
type sealed struct {
	derivation derivation
	salt       []byte
	validation encString
	data       encString
}

func sealedFrom(head header) (*sealed, error) {
	keyDerivation := derivation{
		function:    kdf(head.KDFType),
		iterations:  head.KDFIterations,
		memoryMiB:   head.KDFMemory,
		parallelism: head.KDFParallelism,
	}
	if !keyDerivation.supported() {
		return nil, importers.ErrUnsupportedEncryption
	}
	validation, err := parseEncString(head.Validation)
	if err != nil {
		return nil, err
	}
	data, err := parseEncString(head.Data)
	if err != nil {
		return nil, err
	}
	return &sealed{derivation: keyDerivation, salt: []byte(head.Salt), validation: validation, data: data}, nil
}

// Unseal opens the unencrypted JSON export a password-protected export must hold.
func (s *sealed) Unseal(password string) (*importers.ExportFile, error) {
	plaintext, err := s.open(password)
	if err != nil {
		return nil, err
	}
	opened, err := openJSON(plaintext)
	if err != nil {
		return nil, importers.ErrUnrecognized
	}
	return opened, nil
}

// open checks the validation envelope first: a MAC failure there is a wrong password, one on the data a tampered file.
func (s *sealed) open(password string) ([]byte, error) {
	material, err := s.derivation.derive(password, s.salt)
	if err != nil {
		return nil, err
	}
	stretched, err := stretch(material)
	clear(material)
	if err != nil {
		return nil, err
	}
	defer stretched.wipe()
	check, err := s.validation.open(stretched)
	switch {
	case errors.Is(err, errMACMismatch):
		return nil, importers.ErrWrongPassword
	case err != nil:
		return nil, importers.Unrecognized(err)
	}
	clear(check)
	plaintext, err := s.data.open(stretched)
	if err != nil {
		return nil, importers.Unrecognized(err)
	}
	return plaintext, nil
}
