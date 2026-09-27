// Package keystore binds secrets to the device's Android Keystore, in StrongBox where the device has one.
package keystore

import (
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/vault"
)

var (
	ErrInvalid  = errors.New("a Keystore binding needs a salt, a bound key and a peer key")
	ErrNoReason = errors.New("a key that needs its owner is used only with a reason")
	ErrKey      = errors.New("the Keystore returned a key of the wrong type or size")
	ErrSecret   = errors.New("the Keystore returned a secret of the wrong size")
)

// Hardware is the device's Android Keystore; public keys cross it in X.509 SubjectPublicKeyInfo form.
type Hardware interface {
	// CreateKey makes a P-256 key agreement key under alias, or for presence an RSA decryption key that
	// Android uses only within the one owner verification each decryption asks for.
	CreateKey(alias string, presence bool) ([]byte, error)
	// Agree returns the raw ECDH secret of a key agreement key with peer.
	Agree(alias string, peer []byte) ([]byte, error)
	// Decrypt asks the owner for reason and returns the RSA-OAEP plaintext, failing with ownerauth errors.
	Decrypt(alias string, ciphertext []byte, reason string) ([]byte, error)
}

// presenceOAEP matches Android Keystore, whose OAEP takes MGF1 with SHA-1 alone before API 35.
var presenceOAEP = rsa.OAEPOptions{Hash: crypto.SHA256, MGFHash: crypto.SHA1}

const (
	// presenceSeedSize is the length of the random seed a presence key seals.
	presenceSeedSize = 32
	// minPresenceKeyBits is the smallest RSA modulus a presence key may have.
	minPresenceKeyBits = 2048
)

// policy is a binding's key alias prefix and HKDF info.
type policy struct {
	alias string
	info  string
}

var (
	pinPolicy      = policy{alias: "ravenpass.pin.", info: "ravenpass/pin/v1/android-keystore"}
	presencePolicy = policy{alias: "ravenpass.unlock.", info: "ravenpass/unlock/v2/android-keystore"}
)

var (
	_ unlock.Binding         = Binding{}
	_ unlock.PresenceBinding = PresenceBinding{}
)

// Binding binds a PIN's device secret to Keystore keys used without asking anyone.
type Binding struct{ Hardware Hardware }

// Create returns the key alias and the software key's public half; its private half must never be kept.
func (b Binding) Create(salt []byte) (boundKey, peerKey []byte, secret [vault.PINSecretSize]byte, err error) {
	if len(salt) == 0 {
		return nil, nil, secret, ErrInvalid
	}
	alias, err := newAlias(pinPolicy)
	if err != nil {
		return nil, nil, secret, err
	}
	encoded, err := b.Hardware.CreateKey(alias, false)
	if err != nil {
		return nil, nil, secret, err
	}
	bound, err := ecdhPublicKey(encoded)
	if err != nil {
		return nil, nil, secret, err
	}
	peer, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, secret, err
	}
	shared, err := peer.ECDH(bound)
	if err != nil {
		return nil, nil, secret, err
	}
	peerKey, err = x509.MarshalPKIXPublicKey(peer.PublicKey())
	if err != nil {
		clear(shared)
		return nil, nil, secret, err
	}
	secret, err = expand(shared, salt, pinPolicy)
	return []byte(alias), peerKey, secret, err
}

func (b Binding) Derive(boundKey, peerKey, salt []byte) ([vault.PINSecretSize]byte, error) {
	if len(boundKey) == 0 || len(peerKey) == 0 || len(salt) == 0 {
		return [vault.PINSecretSize]byte{}, ErrInvalid
	}
	shared, err := b.Hardware.Agree(string(boundKey), peerKey)
	if err != nil {
		return [vault.PINSecretSize]byte{}, err
	}
	return expand(shared, salt, pinPolicy)
}

// PresenceBinding binds Keystore keys that Android uses only within an owner verification made for that use.
type PresenceBinding struct{ Hardware Hardware }

// Create never prompts: it seals a random seed to the key's public half, and peerKey is that sealed seed.
func (b PresenceBinding) Create(salt []byte) (boundKey, peerKey []byte, secret [unlock.SecretSize]byte, err error) {
	if len(salt) == 0 {
		return nil, nil, secret, ErrInvalid
	}
	alias, err := newAlias(presencePolicy)
	if err != nil {
		return nil, nil, secret, err
	}
	encoded, err := b.Hardware.CreateKey(alias, true)
	if err != nil {
		return nil, nil, secret, err
	}
	public, err := rsaPublicKey(encoded)
	if err != nil {
		return nil, nil, secret, err
	}
	seed := make([]byte, presenceSeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, nil, secret, err
	}
	sealed, err := rsa.EncryptOAEPWithOptions(rand.Reader, public, seed, &presenceOAEP)
	if err != nil {
		clear(seed)
		return nil, nil, secret, err
	}
	secret, err = expand(seed, salt, presencePolicy)
	return []byte(alias), sealed, secret, err
}

func (b PresenceBinding) Derive(reason string, boundKey, peerKey, salt []byte) ([unlock.SecretSize]byte, error) {
	if reason == "" {
		return [unlock.SecretSize]byte{}, ErrNoReason
	}
	if len(boundKey) == 0 || len(peerKey) == 0 || len(salt) == 0 {
		return [unlock.SecretSize]byte{}, ErrInvalid
	}
	seed, err := b.Hardware.Decrypt(string(boundKey), peerKey, reason)
	if err != nil {
		return [unlock.SecretSize]byte{}, err
	}
	if len(seed) != presenceSeedSize {
		clear(seed)
		return [unlock.SecretSize]byte{}, ErrSecret
	}
	return expand(seed, salt, presencePolicy)
}

func newAlias(which policy) (string, error) {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return which.alias + hex.EncodeToString(suffix[:]), nil
}

// expand wipes keying once it has derived the secret.
func expand(keying, salt []byte, which policy) ([unlock.SecretSize]byte, error) {
	defer clear(keying)
	var secret [unlock.SecretSize]byte
	key, err := hkdf.Key(sha256.New, keying, salt, which.info, unlock.SecretSize)
	if err != nil {
		return secret, err
	}
	copy(secret[:], key)
	clear(key)
	return secret, nil
}

func ecdhPublicKey(encoded []byte) (*ecdh.PublicKey, error) {
	parsed, err := x509.ParsePKIXPublicKey(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKey, err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrKey
	}
	converted, err := key.ECDH()
	if err != nil || converted.Curve() != ecdh.P256() {
		return nil, ErrKey
	}
	return converted, nil
}

func rsaPublicKey(encoded []byte) (*rsa.PublicKey, error) {
	parsed, err := x509.ParsePKIXPublicKey(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKey, err)
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok || key.N.BitLen() < minPresenceKeyBits {
		return nil, ErrKey
	}
	return key, nil
}
