package api

import (
	"context"
	"errors"

	"github.com/dortanes/ravenpass/packages/app/vaultservice"
)

// BeginRecoveryPhraseChange returns a new recovery phrase for the open vault once verifyOwner accepts the owner; nothing
// is written until ConfirmRecoveryPhraseChange. Where the vault has a PIN on this device, pin is that PIN whichever way
// the owner is verified, since the PIN's copy of the new key is wrapped from it; elsewhere pin is empty. Where the
// device has no way in it can use, current must be the vault's current recovery phrase, which opened it, so whoever
// finds the vault open cannot take it over.
func (s *Service) BeginRecoveryPhraseChange(ctx context.Context, pin, current string) (string, error) {
	var phrase string
	begun := false
	begin := func(opening vaultservice.Opening, pin string) error {
		begun = true
		var err error
		phrase, err = s.vault.BeginRekey(opening, pin)
		return err
	}
	proof := pin
	if proof == "" {
		proof = current
	}
	// BeginRekey counts the PIN as VerifyPIN does, so where the PIN verifies the owner one Argon2id derivation serves both.
	opening, err := s.verifyOwner(ctx, proof, begin)
	if err != nil {
		return "", err
	}
	if !begun {
		if err := begin(opening, pin); err != nil {
			return "", present(err)
		}
	}
	return phrase, nil
}

// ConfirmRecoveryPhraseChange seals the open vault for the new recovery phrase once phrase matches it; automatic backups
// then back it up under the new key at once. Other devices then open the vault with the new phrase; copies saved before
// keep opening with the old one.
func (s *Service) ConfirmRecoveryPhraseChange(phrase string) error {
	if err := s.vault.VerifyStagedPhrase(phrase); err != nil {
		return presentStagedPhraseError(err)
	}
	_, err := s.vault.ConfirmRekey(phrase)
	// A file written under the new key needs a backup under it, the next time the vault is open if not now.
	if err == nil || errors.Is(err, vaultservice.ErrKeyChangeUnfinished) {
		if id, known := s.vault.CurrentVault(); known && s.backups != nil {
			s.backups.KeyChanged(id)
		}
	}
	return present(err)
}

// CancelRecoveryPhraseChange drops the new recovery phrase, leaving the vault as it was.
func (s *Service) CancelRecoveryPhraseChange() {
	s.vault.DiscardRekey()
}
