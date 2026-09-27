package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// VerifyStagedPhrase checks phrase against the recovery phrase of the vault being created or of the staged new phrase.
func (s *Service) VerifyStagedPhrase(phrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.verifyStagedPhrase(phrase)
}

// WithVerifiedStagedPhrase runs action under s.mu once phrase verifies as VerifyStagedPhrase checks it.
func (s *Service) WithVerifiedStagedPhrase(phrase string, action func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.verifyStagedPhrase(phrase); err != nil {
		return err
	}
	return action()
}

func (s *Service) verifyStagedPhrase(phrase string) error {
	if s.rekeying != nil {
		return s.rekeying.rekey.VerifyPhrase(phrase)
	}
	return s.verifyCreationPhrase(phrase)
}

func (s *Service) verifyCreationPhrase(phrase string) error {
	if s.creating == nil {
		return ErrNoPendingSetup
	}
	verified, err := vault.OpenWithRecovery(s.creating.container, phrase)
	if err != nil {
		return err
	}
	verified.Lock()
	return nil
}
