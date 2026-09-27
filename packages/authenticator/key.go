package authenticator

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
)

// ErrInvalidKey reports a private key that is not PKCS #8 DER of an ECDSA P-256 key.
var ErrInvalidKey = errors.New("passkey private key is not an ECDSA P-256 key")

const credentialIDSize = 16

// Key is a new passkey's key pair and credential ID.
type Key struct {
	CredentialID []byte
	// PrivateKey is PKCS #8 DER of an ECDSA P-256 key.
	PrivateKey []byte
}

// NewKey generates an ES256 key and a random 16-byte credential ID.
func NewKey() (Key, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Key{}, err
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return Key{}, err
	}
	credentialID := make([]byte, credentialIDSize)
	if _, err := rand.Read(credentialID); err != nil {
		return Key{}, err
	}
	return Key{CredentialID: credentialID, PrivateKey: encoded}, nil
}

func parseKey(privateKey []byte) (*ecdsa.PrivateKey, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, ErrInvalidKey
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, ErrInvalidKey
	}
	return key, nil
}
