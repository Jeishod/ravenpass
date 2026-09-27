//go:build android

package bridge

import "github.com/dortanes/ravenpass/apps/mobile/internal/keystore"

var _ keystore.Hardware = Keystore{}

// Keystore keeps keys in the Android Keystore, usable only while the device is unlocked.
type Keystore struct{}

// CreateKey returns the new key's X.509 SubjectPublicKeyInfo; a presence key needs a device that verifies its owner.
func (Keystore) CreateKey(alias string, presence bool) ([]byte, error) {
	s, key := createKey(alias, presence)
	if err := keystoreError(s); err != nil {
		return nil, err
	}
	return key, nil
}

// Agree returns the raw ECDH secret with peer's SubjectPublicKeyInfo.
func (Keystore) Agree(alias string, peer []byte) ([]byte, error) {
	s, secret := agree(alias, peer)
	return secretOf(s, secret)
}

// Decrypt shows the owner a prompt for reason that authorizes this one decryption alone.
func (Keystore) Decrypt(alias string, ciphertext []byte, reason string) ([]byte, error) {
	prompting <- struct{}{}
	defer func() { <-prompting }()
	s, plaintext := decrypt(alias, ciphertext, reason)
	return secretOf(s, plaintext)
}

func secretOf(s status, secret []byte) ([]byte, error) {
	if err := keystoreError(s); err != nil {
		clear(secret)
		return nil, err
	}
	// An empty secret would derive a key anyone can compute.
	if len(secret) == 0 {
		return nil, ErrKeystore
	}
	return secret, nil
}
