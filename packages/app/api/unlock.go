package api

import (
	"context"
	"errors"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
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

// SetPIN sets or replaces the open vault's PIN once verifyOwner accepts the owner; current is the PIN it may check.
func (s *Service) SetPIN(ctx context.Context, pin, current string) error {
	if !vault.ValidPIN(pin) {
		return present(vault.ErrInvalidPIN)
	}
	if err := s.verifyOwner(ctx, current, s.vault.VerifyPIN, nil); err != nil {
		return err
	}
	return present(s.vault.SetPIN(pin))
}

// RemovePIN removes the open vault's PIN once verifyOwner accepts the owner, as SetPIN does.
func (s *Service) RemovePIN(ctx context.Context, current string) error {
	if err := s.verifyOwner(ctx, current, s.vault.VerifyPIN, nil); err != nil {
		return err
	}
	return present(s.vault.RemovePIN())
}

// SetBiometryUnlock turns device authentication on or off once verifyOwner accepts the owner, as SetPIN does; asking
// for what is already set changes nothing and asks no one.
func (s *Service) SetBiometryUnlock(ctx context.Context, enabled bool, current string) error {
	methods, err := s.vault.UnlockMethods()
	if err != nil {
		return present(err)
	}
	if methods.BiometryEnabled != enabled {
		if err := s.verifyOwner(ctx, current, s.vault.VerifyPIN, nil); err != nil {
			return err
		}
	}
	return present(s.vault.SetBiometryUnlock(enabled))
}

// verifyOwner asks device authentication where the vault opens with it, else checks current by checkPIN, which counts a
// wrong PIN as the locked screen does. A vault with neither, which only the recovery phrase opened, is checked by
// checkNoWayIn, or changes unasked where that is nil.
func (s *Service) verifyOwner(ctx context.Context, current string, checkPIN func(pin string) error, checkNoWayIn func() error) error {
	methods, err := s.vault.UnlockMethods()
	if err != nil {
		return present(err)
	}
	switch {
	case methods.BiometryEnabled && methods.BiometryAvailable:
		err := s.owner.VerifyDevice(ctx, confirmation.ChangingUnlock())
		// The device reports its authentication unavailable only once the prompt is asked for.
		if errors.Is(err, verification.ErrUnverifiable) || errors.Is(err, ownerauth.ErrFailed) {
			return fail(failureOwnerUnverified)
		}
		return present(err)
	case methods.PINSet:
		return present(checkPIN(current))
	case checkNoWayIn != nil:
		return present(checkNoWayIn())
	default:
		return nil
	}
}
