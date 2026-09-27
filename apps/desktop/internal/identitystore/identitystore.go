// Package identitystore keeps the system credential identity store in step with the last open vault.
package identitystore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/vault"
)

// System is the platform's credential identity store.
type System interface {
	// Enabled reports whether Ravenpass is an enabled AutoFill provider; the system clears a disabled one's store.
	Enabled() bool
	// Replace makes identities the store's whole list. A failed write leaves the list as it was.
	Replace(identities []autofill.CredentialIdentity) error
	// RemoveAll empties the store. A failed removal leaves the list as it was.
	RemoveAll() error
}

// Vault reports changes to the vault state and which vault is current.
type Vault interface {
	AwaitVaultState(ctx context.Context, seen uint64) (uint64, error)
	CurrentVault() (vault.ID, bool)
}

// Identities lists the open vault's suggestible accounts and fails with autofill.ErrLocked while it is locked.
type Identities interface {
	CredentialIdentities() ([]autofill.CredentialIdentity, error)
}

// held is what the store holds as far as this run knows.
type held uint8

const (
	// heldUnknown is a store this run has not written or cleared.
	heldUnknown held = iota
	heldNothing
	// heldList is owner's list, whose digest is unknown when an earlier run wrote it.
	heldList
)

// List keeps the system store in step with the vault.
type List struct {
	vault      Vault
	identities Identities
	on         func() bool
	system     System
	changed    chan struct{}

	// held, owner and digest are read and written by Run's goroutine alone.
	held   held
	owner  vault.ID
	digest [sha256.Size]byte
}

// New composes a List. on reports whether the owner has the identity list on.
func New(vault Vault, identities Identities, on func() bool, system System) (*List, error) {
	if vault == nil || identities == nil || on == nil || system == nil {
		return nil, errors.New("the vault, its identities, the owner's choice and the system store are required")
	}
	return &List{vault: vault, identities: identities, on: on, system: system, changed: make(chan struct{}, 1)}, nil
}

// Changed asks Run to bring the store in step again.
func (l *List) Changed() {
	select {
	case l.changed <- struct{}{}:
	default:
	}
}

// Run brings the store in step now and after each vault state change or Changed call, until ctx ends.
func (l *List) Run(ctx context.Context) {
	go l.follow(ctx)
	for {
		l.sync()
		select {
		case <-ctx.Done():
			return
		case <-l.changed:
		}
	}
}

// follow calls Changed after each change of the vault's state until ctx ends.
func (l *List) follow(ctx context.Context) {
	var seen uint64
	for {
		count, err := l.vault.AwaitVaultState(ctx, seen)
		if err != nil {
			return
		}
		seen = count
		l.Changed()
	}
}

// sync reads the current vault before its list; a switch in between is corrected by the switch's own change.
func (l *List) sync() {
	if !l.system.Enabled() {
		l.held = heldNothing
		return
	}
	if !l.on() {
		l.clear()
		return
	}
	current, found := l.vault.CurrentVault()
	identities, err := l.identities.CredentialIdentities()
	switch {
	case err == nil && found:
		l.replace(current, identities)
	case !errors.Is(err, autofill.ErrLocked):
		// An unreadable index, or a vault opened after current was read; the next state change resyncs.
	case !found:
		l.clear()
	case l.held == heldUnknown:
		// The list a run before this one wrote is taken to be the current vault's.
		l.held, l.owner = heldList, current
	case l.held == heldList && l.owner != current:
		l.clear()
	}
}

// replace writes identities as owner's list unless the store already holds that list.
func (l *List) replace(owner vault.ID, identities []autofill.CredentialIdentity) {
	listed, err := json.Marshal(identities)
	if err != nil {
		return
	}
	digest := sha256.Sum256(listed)
	if l.held == heldList && l.owner == owner && l.digest == digest {
		return
	}
	if l.system.Replace(identities) != nil {
		return
	}
	l.held, l.owner, l.digest = heldList, owner, digest
}

// clear empties the store unless it holds nothing already.
func (l *List) clear() {
	if l.held == heldNothing || l.system.RemoveAll() != nil {
		return
	}
	l.held, l.owner, l.digest = heldNothing, vault.ID{}, [sha256.Size]byte{}
}
