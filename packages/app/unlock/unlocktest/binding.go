// Package unlocktest gives tests PIN and presence bindings with no hardware behind them.
package unlocktest

import (
	"bytes"
	"crypto/rand"
	"errors"
	"sync"

	"github.com/dortanes/ravenpass/packages/vault"
)

// ErrRejected is what Derive fails with for a bound key the binding does not hold.
var ErrRejected = errors.New("the bound key is not held by this binding")

type heldKey struct {
	peerKey []byte
	salt    []byte
	secret  [vault.PINSecretSize]byte
}

// Binding stands for one device's hardware keys, held in memory.
type Binding struct {
	mu        sync.Mutex
	held      map[string]heldKey
	created   int
	createErr error
}

// NewBinding returns a Binding that holds no key.
func NewBinding() *Binding { return &Binding{held: make(map[string]heldKey)} }

// Create holds a new random key for salt.
func (b *Binding) Create(salt []byte) ([]byte, []byte, [vault.PINSecretSize]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var secret [vault.PINSecretSize]byte
	if b.createErr != nil {
		return nil, nil, secret, b.createErr
	}
	boundKey := make([]byte, 32)
	peerKey := make([]byte, 65)
	rand.Read(boundKey)
	rand.Read(peerKey)
	rand.Read(secret[:])
	b.held[string(boundKey)] = heldKey{peerKey: bytes.Clone(peerKey), salt: bytes.Clone(salt), secret: secret}
	b.created++
	return boundKey, peerKey, secret, nil
}

// Derive yields the secret of a held key with its own peer key and salt, else ErrRejected.
func (b *Binding) Derive(boundKey, peerKey, salt []byte) ([vault.PINSecretSize]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	held, known := b.held[string(boundKey)]
	if !known || !bytes.Equal(held.peerKey, peerKey) || !bytes.Equal(held.salt, salt) {
		return [vault.PINSecretSize]byte{}, ErrRejected
	}
	return held.secret, nil
}

// Created counts the keys Create made.
func (b *Binding) Created() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.created
}

// Reset drops every held key, as a reset of the hardware or a move to another device does.
func (b *Binding) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	clear(b.held)
}

// FailCreate makes every further Create fail with err.
func (b *Binding) FailCreate(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.createErr = err
}
