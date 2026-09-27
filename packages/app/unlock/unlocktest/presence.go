package unlocktest

import (
	"sync"

	"github.com/dortanes/ravenpass/packages/vault"
)

// PresenceBinding is a Binding whose every Derive is an owner prompt answered as Answer sets, allowing by default.
type PresenceBinding struct {
	hardware *Binding
	mu       sync.Mutex
	answer   error
	reasons  []string
}

// NewPresenceBinding returns a PresenceBinding that holds no key.
func NewPresenceBinding() *PresenceBinding { return &PresenceBinding{hardware: NewBinding()} }

// Create holds a new random key for salt.
func (p *PresenceBinding) Create(salt []byte) ([]byte, []byte, [vault.PINSecretSize]byte, error) {
	return p.hardware.Create(salt)
}

// Derive records a prompt for reason and yields the held secret unless Answer set an error.
func (p *PresenceBinding) Derive(reason string, boundKey, peerKey, salt []byte) ([vault.PINSecretSize]byte, error) {
	p.mu.Lock()
	p.reasons = append(p.reasons, reason)
	answer := p.answer
	p.mu.Unlock()
	if answer != nil {
		return [vault.PINSecretSize]byte{}, answer
	}
	return p.hardware.Derive(boundKey, peerKey, salt)
}

// Answer makes the owner answer every further prompt with err, or allow it where err is nil.
func (p *PresenceBinding) Answer(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.answer = err
}

// Prompts lists the reason of every prompt the owner was shown.
func (p *PresenceBinding) Prompts() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.reasons...)
}

// Created counts the keys Create made.
func (p *PresenceBinding) Created() int { return p.hardware.Created() }

// Reset drops every held key, as a reset of the hardware or a move to another device does.
func (p *PresenceBinding) Reset() { p.hardware.Reset() }

// FailCreate makes every further Create fail with err.
func (p *PresenceBinding) FailCreate(err error) { p.hardware.FailCreate(err) }
