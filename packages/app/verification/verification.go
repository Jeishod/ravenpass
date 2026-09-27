// Package verification asks the person to confirm one action on the device; nothing is remembered.
package verification

import (
	"context"
	"errors"
	"time"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
)

// Timeout is how long a verification waits for an answer before it ends as declined.
const Timeout = 2 * time.Minute

// Errors a verification ends with.
var (
	ErrDeclined     = errors.New("the verification was declined")
	ErrUnverifiable = errors.New("the vault offers no way to verify the person on this device")
)

// Method is how a person is asked to verify.
type Method uint8

const (
	// MethodDevice is the system prompt for device authentication.
	MethodDevice Method = iota + 1
	// MethodPIN is the vault's PIN in the confirmation panel.
	MethodPIN
)

// Methods reports the ways the open vault opens on the device.
type Methods interface {
	UnlockMethods() (vaultservice.Methods, error)
}

// OwnerPrompt asks device authentication to verify the owner for reason until answered or ctx ends.
type OwnerPrompt interface {
	AuthenticateOwner(ctx context.Context, reason string) error
}

// PINPrompt asks for the vault's PIN for reason, returning confirmation.ErrDeclined on refusal.
type PINPrompt interface {
	Ask(ctx context.Context, reason confirmation.Reason) error
}

// Verifier asks the person to verify each reason.
type Verifier struct {
	methods Methods
	owner   OwnerPrompt
	reason  func(confirmation.Reason) string
	pins    PINPrompt
	timeout time.Duration
}

// New composes a Verifier. reason words the system prompt for a reason in the recorded language.
func New(methods Methods, owner OwnerPrompt, reason func(confirmation.Reason) string, pins PINPrompt) (*Verifier, error) {
	if methods == nil || owner == nil || reason == nil || pins == nil {
		return nil, errors.New("unlock methods, owner prompt, reason and PIN prompt are required")
	}
	return &Verifier{methods: methods, owner: owner, reason: reason, pins: pins, timeout: Timeout}, nil
}

// Verify asks by device authentication where the vault opens with it, else by PIN; asked learns the method first.
func (v *Verifier) Verify(ctx context.Context, reason confirmation.Reason, asked func(Method)) error {
	methods, err := v.methods.UnlockMethods()
	if err != nil {
		return err
	}
	waiting, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	switch {
	case methods.BiometryEnabled && methods.BiometryAvailable:
		asked(MethodDevice)
		err = v.owner.AuthenticateOwner(waiting, v.reason(reason))
	case methods.PINSet:
		asked(MethodPIN)
		err = v.pins.Ask(waiting, reason)
	default:
		return ErrUnverifiable
	}
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case err == nil:
		return nil
	case errors.Is(err, ownerauth.ErrUnavailable):
		return ErrUnverifiable
	case waiting.Err() != nil, errors.Is(err, confirmation.ErrDeclined),
		errors.Is(err, ownerauth.ErrCanceled), errors.Is(err, ownerauth.ErrFailed):
		return ErrDeclined
	default:
		return err
	}
}

// VerifyDevice asks device authentication alone to verify reason, whatever the vault opens with.
func (v *Verifier) VerifyDevice(ctx context.Context, reason confirmation.Reason) error {
	waiting, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	err := v.owner.AuthenticateOwner(waiting, v.reason(reason))
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case err == nil:
		return nil
	case errors.Is(err, ownerauth.ErrUnavailable):
		return ErrUnverifiable
	case waiting.Err() != nil, errors.Is(err, ownerauth.ErrCanceled):
		return ErrDeclined
	default:
		return err
	}
}
