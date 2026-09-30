package api

import (
	"context"
	"errors"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/app/verification"
	"github.com/dortanes/ravenpass/packages/vault"
)

// OwnerVerifier asks the person on this device to prove they own the vault before what reason names.
type OwnerVerifier interface {
	// Verify fails with verification.ErrDeclined on refusal and ErrUnverifiable when the vault offers no way to ask.
	Verify(ctx context.Context, reason confirmation.Reason, asked func(verification.Method)) error
	// VerifyDevice uses device authentication alone; it adds ownerauth.ErrFailed for an unrecognised person.
	VerifyDevice(ctx context.Context, reason confirmation.Reason) error
}

// UnlockMethods reports the ways into the bound vault this device has and the PIN length bounds.
type UnlockMethods struct {
	BiometryAvailable bool `json:"biometryAvailable"`
	BiometryEnabled   bool `json:"biometryEnabled"`
	PINSet            bool `json:"pinSet"`
	PINAttemptsLeft   int  `json:"pinAttemptsLeft"`
	PINMinLength      int  `json:"pinMinLength"`
	PINMaxLength      int  `json:"pinMaxLength"`
}

// GetUnlockMethods reports the ways into the bound vault.
func (s *Service) GetUnlockMethods() (UnlockMethods, error) {
	methods, err := s.vault.UnlockMethods()
	if err != nil {
		return UnlockMethods{}, present(err)
	}
	return UnlockMethods{
		BiometryAvailable: methods.BiometryAvailable,
		BiometryEnabled:   methods.BiometryEnabled,
		PINSet:            methods.PINSet,
		PINAttemptsLeft:   methods.PINAttemptsLeft,
		PINMinLength:      vault.MinPINLength,
		PINMaxLength:      vault.MaxPINLength,
	}, nil
}

// UnlockWithPIN opens the vault with its PIN.
func (s *Service) UnlockWithPIN(pin string) error {
	_, err := s.vault.UnlockWithPIN(pin)
	return s.opened(err)
}

// SetPIN sets or replaces the open vault's PIN once verifyOwner accepts the owner; current is the PIN or recovery key
// it may check.
func (s *Service) SetPIN(ctx context.Context, pin, current string) error {
	if !vault.ValidPIN(pin) {
		return present(vault.ErrInvalidPIN)
	}
	opening, err := s.verifyOwner(ctx, current, s.verifyPIN)
	if err != nil {
		return err
	}
	return present(s.vault.SetPIN(opening, pin))
}

// verifyPIN checks pin against the open vault as verifyOwner's PIN check; the change it authorizes checks the session.
func (s *Service) verifyPIN(_ vaultservice.Opening, pin string) error {
	return s.vault.VerifyPIN(pin)
}

// RemovePIN removes the open vault's PIN once verifyOwner accepts the owner, as SetPIN does.
func (s *Service) RemovePIN(ctx context.Context, current string) error {
	opening, err := s.verifyOwner(ctx, current, s.verifyPIN)
	if err != nil {
		return err
	}
	return present(s.vault.RemovePIN(opening))
}

// SetBiometryUnlock turns device authentication on or off once verifyOwner accepts the owner, as SetPIN does; asking
// for what is already set changes nothing and asks no one.
func (s *Service) SetBiometryUnlock(ctx context.Context, enabled bool, current string) error {
	methods, err := s.vault.UnlockMethods()
	if err != nil {
		return present(err)
	}
	if methods.BiometryEnabled == enabled {
		return nil
	}
	opening, err := s.verifyOwner(ctx, current, s.verifyPIN)
	if err != nil {
		return err
	}
	return present(s.vault.SetBiometryUnlock(opening, enabled))
}

// verifyOwner names the open vault session, then asks device authentication where the vault opens with it, else checks
// current by checkPIN, which counts a wrong PIN as the locked screen does, else checks current as the vault's recovery
// key. The change it authorizes is bound to the session named, so one that closed meanwhile changes nothing.
func (s *Service) verifyOwner(ctx context.Context, current string, checkPIN func(opening vaultservice.Opening, pin string) error) (vaultservice.Opening, error) {
	opening, err := s.vault.CurrentOpening()
	if err != nil {
		return 0, present(err)
	}
	methods, err := s.vault.UnlockMethods()
	if err != nil {
		return 0, present(err)
	}
	switch {
	case methods.BiometryEnabled && methods.BiometryAvailable:
		verified := s.owner.VerifyDevice(ctx, confirmation.ChangingUnlock())
		// The device reports its authentication unavailable only once the prompt is asked for.
		if errors.Is(verified, verification.ErrUnverifiable) || errors.Is(verified, ownerauth.ErrFailed) {
			return 0, fail(failureOwnerUnverified)
		}
		err = present(verified)
	case methods.PINSet:
		err = present(checkPIN(opening, current))
	default:
		err = presentOwnerKey(s.vault.VerifyRecoveryPhrase(current))
	}
	return opening, err
}

// presentOwnerKey words a recovery key that did not open the open vault as a key that does not match.
func presentOwnerKey(err error) error {
	if errors.Is(err, vault.ErrAuthentication) {
		return fail(failureRecoveryKeyMismatch)
	}
	return present(err)
}
