//go:build android

package bridge

import (
	"context"
	"time"

	"github.com/dortanes/ravenpass/packages/app/ownerauth"
)

// cancelRetry spaces cancels: one that reaches Java before the prompt registers dismisses nothing.
const cancelRetry = 50 * time.Millisecond

// prompting admits one prompt at a time, so a cancel reaches the prompt of the call that sent it.
var prompting = make(chan struct{}, 1)

var _ ownerauth.Platform = Owner{}

// Owner verifies the device's owner with BiometricPrompt: a strong biometric or the screen lock.
type Owner struct{}

// Available reports without prompting whether the device can verify its owner.
func (Owner) Available() bool { return ownerAvailable() }

// Authenticate shows the prompt until the owner answers or ctx ends, which dismisses it.
func (Owner) Authenticate(ctx context.Context, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := titled(reason); err != nil {
		return err
	}
	select {
	case prompting <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-prompting }()
	answered := make(chan status, 1)
	go func() { answered <- authenticate(reason) }()
	select {
	case s := <-answered:
		return ownerError(s)
	case <-ctx.Done():
	}
	retry := time.NewTicker(cancelRetry)
	defer retry.Stop()
	for {
		cancelAuthentication()
		select {
		case <-answered:
			return ctx.Err()
		case <-retry.C:
		}
	}
}
