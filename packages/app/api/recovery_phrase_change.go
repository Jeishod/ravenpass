package api

import "context"

// BeginRecoveryPhraseChange returns a new recovery phrase for the open vault once verifyOwner accepts the owner; nothing
// is written until ConfirmRecoveryPhraseChange. Where the vault has a PIN on this device, pin is that PIN whichever way
// the owner is verified, since the PIN's copy of the new key is wrapped from it; elsewhere pin is empty. Where the
// device has no way in, current must be the vault's current recovery phrase, which opened it, so whoever finds the vault
// open cannot take it over.
func (s *Service) BeginRecoveryPhraseChange(ctx context.Context, pin, current string) (string, error) {
	var phrase string
	begun := false
	begin := func(pin string) error {
		begun = true
		var err error
		phrase, err = s.vault.BeginRekey(pin)
		return err
	}
	// BeginRekey counts the PIN as VerifyPIN does, so where the PIN verifies the owner one Argon2id derivation serves both.
	checkPhrase := func() error { return s.vault.VerifyRecoveryPhrase(current) }
	if err := s.verifyOwner(ctx, pin, begin, checkPhrase); err != nil {
		return "", err
	}
	if !begun {
		if err := begin(pin); err != nil {
			return "", present(err)
		}
	}
	return phrase, nil
}

// ConfirmRecoveryPhraseChange seals the open vault for the new recovery phrase once phrase matches it. Other devices
// then open the vault with the new phrase; copies saved before keep opening with the old one.
func (s *Service) ConfirmRecoveryPhraseChange(phrase string) error {
	if err := s.vault.VerifyStagedPhrase(phrase); err != nil {
		return presentStagedPhraseError(err)
	}
	if _, err := s.vault.ConfirmRekey(phrase); err != nil {
		return present(err)
	}
	if s.backups != nil {
		s.backups.Poke()
	}
	return nil
}

// CancelRecoveryPhraseChange drops the new recovery phrase, leaving the vault as it was.
func (s *Service) CancelRecoveryPhraseChange() {
	s.vault.DiscardRekey()
}
