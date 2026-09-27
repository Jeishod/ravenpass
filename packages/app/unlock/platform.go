package unlock

import (
	"errors"
	"fmt"

	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/vault"
)

// SecretSize is the length of the secret a presence binding yields, which wraps the vault key.
const SecretSize = 32

// PresenceBinding ties a secret to a hardware key usable only after device authentication; it stores nothing.
type PresenceBinding interface {
	// Create makes a new key without asking the owner and returns what reaches it again and its secret.
	Create(salt []byte) (boundKey, peerKey []byte, secret [SecretSize]byte, err error)
	// Derive asks the owner for reason and yields the secret; an unverified owner fails with an ownerauth error.
	Derive(reason string, boundKey, peerKey, salt []byte) ([SecretSize]byte, error)
}

// PlatformCredential is a hardware key's opaque form, its peer key, and the vault key wrapped for their secret.
type PlatformCredential struct {
	BoundKey []byte `json:"boundKey"`
	PeerKey  []byte `json:"peerKey"`
	Envelope []byte `json:"envelope"`
}

func (c *PlatformCredential) complete() bool {
	return len(c.BoundKey) > 0 && len(c.PeerKey) > 0 && len(c.Envelope) > 0
}

// PlatformCredentials binds each credential to a new hardware key salted with the vault identity.
type PlatformCredentials struct {
	binding PresenceBinding
}

// NewPlatformCredentials returns PlatformCredentials over binding and fails when binding is nil.
func NewPlatformCredentials(binding PresenceBinding) (*PlatformCredentials, error) {
	if binding == nil {
		return nil, errors.New("a presence binding is required")
	}
	return &PlatformCredentials{binding: binding}, nil
}

// Set binds a new hardware key for vaultID and wraps the vault key for its secret, without asking the owner.
func (p *PlatformCredentials) Set(vaultID vault.ID, wrap func(key []byte) ([]byte, error)) (*PlatformCredential, error) {
	boundKey, peerKey, secret, err := p.binding.Create(vaultID[:])
	if err != nil {
		return nil, fmt.Errorf("bind the device's authentication: %w", err)
	}
	defer clear(secret[:])
	envelope, err := wrap(secret[:])
	if err != nil {
		return nil, err
	}
	return &PlatformCredential{BoundKey: boundKey, PeerKey: peerKey, Envelope: envelope}, nil
}

// Key asks the owner for reason and derives record's wrapping key; a rejected hardware key fails with ErrUnbound.
func (p *PlatformCredentials) Key(record *PlatformCredential, vaultID vault.ID, reason string) ([SecretSize]byte, error) {
	secret, err := p.binding.Derive(reason, record.BoundKey, record.PeerKey, vaultID[:])
	switch {
	case err == nil:
		return secret, nil
	case errors.Is(err, ownerauth.ErrCanceled), errors.Is(err, ownerauth.ErrFailed),
		errors.Is(err, ownerauth.ErrUnavailable):
		return [SecretSize]byte{}, err
	default:
		return [SecretSize]byte{}, fmt.Errorf("%w: %w", ErrUnbound, err)
	}
}
