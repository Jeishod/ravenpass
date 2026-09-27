package vaultservice

import (
	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/vault"
)

// stagedRekey is a new vault key and recovery phrase awaiting confirmation of the phrase.
type stagedRekey struct {
	rekey *vault.Rekey
	// pinEnvelope is the PIN's copy of the new key, nil where the vault has no PIN on this device.
	pinEnvelope []byte
}

// BeginRekey stages a new vault key and recovery phrase for the open vault and returns the phrase; nothing is written.
// Where the vault has a PIN on this device, pin must be it, counted and locking as VerifyPIN does, so the PIN keeps
// opening the vault; elsewhere pin is empty.
func (s *Service) BeginRekey(pin string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discardRekey()
	id, policy, err := s.openVaultPolicy()
	if err != nil {
		return "", err
	}
	if !policy.HasPIN() && pin != "" {
		return "", unlock.ErrNoPIN
	}
	if policy.HasPIN() && !vault.ValidPIN(pin) {
		return "", vault.ErrInvalidPIN
	}
	rekey, err := s.session.BeginRekey()
	if err != nil {
		return "", err
	}
	stage := &stagedRekey{rekey: rekey}
	if policy.HasPIN() {
		err := s.attemptPIN(id, policy, pin, func(key []byte) error {
			if err := s.session.VerifyDeviceKey(key, policy.PIN.Envelope); err != nil {
				return err
			}
			var err error
			stage.pinEnvelope, err = rekey.WrapDeviceKey(key)
			return err
		})
		if err != nil {
			rekey.Discard()
			s.lockOnPINRemoved(err)
			return "", err
		}
	}
	s.rekeying = stage
	return rekey.RecoveryPhrase(), nil
}

// VerifyRecoveryPhrase fails with vault.ErrInvalidPhrase or vault.ErrAuthentication unless phrase opens the open vault.
func (s *Service) VerifyRecoveryPhrase(phrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	return s.session.VerifyRecoveryPhrase(phrase)
}

// ConfirmRekey seals the open vault under the staged key once phrase matches the staged recovery phrase, and moves this
// device's ways in to the new key. The new file replaces the old one whole; other devices then need the new phrase.
func (s *Service) ConfirmRekey(phrase string) (vault.Head, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stage := s.rekeying
	if stage == nil {
		return vault.Head{}, ErrNoPendingSetup
	}
	if err := stage.rekey.VerifyPhrase(phrase); err != nil {
		return vault.Head{}, err
	}
	defer s.discardRekey()
	id, policy, err := s.openVaultPolicy()
	if err != nil {
		return vault.Head{}, err
	}
	next, err := s.rewrappedPolicy(id, policy, stage)
	if err != nil {
		return vault.Head{}, err
	}
	pending, err := s.session.PrepareRekey(stage.rekey)
	if err != nil {
		return vault.Head{}, err
	}
	if err := s.commitThen(pending, func() error { return s.savePolicy(id, next) }); err != nil {
		return vault.Head{}, err
	}
	return s.session.Head()
}

// DiscardRekey drops the staged vault key and recovery phrase, if any.
func (s *Service) DiscardRekey() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discardRekey()
}

func (s *Service) discardRekey() {
	if s.rekeying != nil {
		s.rekeying.rekey.Discard()
		s.rekeying = nil
	}
}

// rewrappedPolicy is policy with each way in holding the staged key: the PIN's envelope rewrapped when it was begun,
// device authentication bound to a new hardware key without asking the owner.
func (s *Service) rewrappedPolicy(id vault.ID, policy unlock.Policy, stage *stagedRekey) (unlock.Policy, error) {
	var next unlock.Policy
	if policy.HasPIN() && stage.pinEnvelope != nil {
		pin := *policy.PIN
		pin.Envelope = stage.pinEnvelope
		next.PIN = &pin
	}
	if policy.HasPlatform() {
		platform, err := s.platforms.Set(id, stage.rekey.WrapDeviceKey)
		if err != nil {
			return unlock.Policy{}, err
		}
		next.Platform = platform
	}
	return next, nil
}
