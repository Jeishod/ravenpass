package unlock

import (
	"errors"
	"fmt"

	"github.com/dortanes/ravenpass/packages/vault"
)

// ErrUnbound reports a hardware key this device no longer accepts, as after a copy or a hardware reset.
var ErrUnbound = errors.New("this way of unlocking is bound to a key this device does not hold")

// Binding ties a PIN to a hardware key this device never releases; it stores nothing.
type Binding interface {
	// Create makes a new key and returns what reaches it again and its secret.
	Create(salt []byte) (boundKey, peerKey []byte, secret [vault.PINSecretSize]byte, err error)
	// Derive yields the secret again, failing where the hardware rejects the bound key.
	Derive(boundKey, peerKey, salt []byte) ([vault.PINSecretSize]byte, error)
}

// PIN is what a device keeps for a PIN, with its count of wrong tries in a row.
// Never persist the digits or the device secret; BoundKey and PeerKey yield the secret on the device alone.
type PIN struct {
	Salt     []byte `json:"salt"`
	BoundKey []byte `json:"boundKey"`
	PeerKey  []byte `json:"peerKey"`
	Envelope []byte `json:"envelope"`
	Failures int    `json:"failures"`
}

func (p *PIN) complete() bool {
	return len(p.Salt) == vault.PINSaltSize && len(p.BoundKey) > 0 && len(p.PeerKey) > 0 &&
		len(p.Envelope) > 0 && p.Failures >= 0 && p.Failures < MaxPINFailures
}

// Remaining is how many wrong attempts are left before the PIN is removed.
func (p *PIN) Remaining() int { return MaxPINFailures - p.Failures }

// PINKey turns a PIN, its salt and the device secret into the key wrapping a copy of the vault key.
type PINKey func(pin string, salt, deviceSecret []byte) ([32]byte, error)

// PINs binds each PIN to a new hardware key salted with the vault identity.
type PINs struct {
	binding Binding
	derive  PINKey
}

// NewPINs returns PINs over binding and fails when binding is nil; a nil derive uses vault.DerivePINKey.
func NewPINs(binding Binding, derive PINKey) (*PINs, error) {
	if binding == nil {
		return nil, errors.New("a PIN binding is required")
	}
	if derive == nil {
		derive = vault.DerivePINKey
	}
	return &PINs{binding: binding, derive: derive}, nil
}

// Set makes the record for pin on vaultID, bound to a new hardware key; the digits are not kept.
func (p *PINs) Set(pin string, vaultID vault.ID, wrap func(key []byte) ([]byte, error)) (*PIN, error) {
	if !vault.ValidPIN(pin) {
		return nil, vault.ErrInvalidPIN
	}
	salt, err := vault.NewPINSalt()
	if err != nil {
		return nil, err
	}
	boundKey, peerKey, secret, err := p.binding.Create(vaultID[:])
	if err != nil {
		return nil, fmt.Errorf("bind the PIN to this device: %w", err)
	}
	defer clear(secret[:])
	key, err := p.derive(pin, salt, secret[:])
	if err != nil {
		return nil, err
	}
	envelope, err := wrap(key[:])
	clear(key[:])
	if err != nil {
		return nil, err
	}
	return &PIN{Salt: salt, BoundKey: boundKey, PeerKey: peerKey, Envelope: envelope}, nil
}

// Key derives the wrapping key for an attempt at record's PIN; a rejected hardware key fails with ErrUnbound.
func (p *PINs) Key(record *PIN, pin string, vaultID vault.ID) ([32]byte, error) {
	if !vault.ValidPIN(pin) {
		return [32]byte{}, vault.ErrInvalidPIN
	}
	secret, err := p.secret(record, vaultID)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(secret[:])
	return p.derive(pin, record.Salt, secret[:])
}

// Bound reports whether this device accepts record's hardware key for the vault vaultID names.
func (p *PINs) Bound(record *PIN, vaultID vault.ID) bool {
	secret, err := p.secret(record, vaultID)
	clear(secret[:])
	return err == nil
}

func (p *PINs) secret(record *PIN, vaultID vault.ID) ([vault.PINSecretSize]byte, error) {
	secret, err := p.binding.Derive(record.BoundKey, record.PeerKey, vaultID[:])
	if err != nil {
		return [vault.PINSecretSize]byte{}, fmt.Errorf("%w: %w", ErrUnbound, err)
	}
	return secret, nil
}
