// Package ownerauth asks device authentication to verify the owner; it proves presence and releases no key.
package ownerauth

import (
	"context"
	"errors"
)

// Errors an owner authentication fails with.
var (
	ErrCanceled    = errors.New("the owner canceled the authentication")
	ErrFailed      = errors.New("the owner could not be authenticated")
	ErrUnavailable = errors.New("this device cannot authenticate its owner")
	ErrNoReason    = errors.New("an authentication prompt needs a reason")
)

// Platform is the system's owner authentication.
type Platform interface {
	// Available reports without prompting whether the owner can be authenticated at all.
	Available() bool
	// Authenticate shows the system prompt with reason until the owner answers or ctx ends and dismisses it.
	Authenticate(ctx context.Context, reason string) error
}

// Authenticator verifies this device's owner.
type Authenticator struct {
	platform Platform
}

// New returns the Authenticator of the built-in platform, which is unavailable off macOS.
func New() *Authenticator { return NewFor(systemPlatform{}) }

// NewFor returns the Authenticator that verifies the owner through platform.
func NewFor(platform Platform) *Authenticator { return &Authenticator{platform: platform} }

// DeviceOwnerAvailable reports, without prompting, whether this device can authenticate its owner.
func (a *Authenticator) DeviceOwnerAvailable() bool { return a.platform.Available() }

// AuthenticateOwner asks the device to authenticate its owner for reason until answered or ctx ends.
func (a *Authenticator) AuthenticateOwner(ctx context.Context, reason string) error {
	if reason == "" {
		return ErrNoReason
	}
	return a.platform.Authenticate(ctx, reason)
}
