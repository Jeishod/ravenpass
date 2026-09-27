package vaultservice

import (
	"context"

	"github.com/dortanes/ravenpass/packages/vault"
)

// AwaitVaultState blocks until the count of saves, opens, locks and location changes passes seen.
func (s *Service) AwaitVaultState(ctx context.Context, seen uint64) (uint64, error) {
	return s.states.Await(ctx, seen)
}

// CurrentVault is the open vault's ID, else the unverified ID the current location's file names.
func (s *Service) CurrentVault() (vault.ID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		head, err := s.session.Head()
		return head.VaultID, err == nil
	}
	container, err := s.files.LoadCiphertext()
	if err != nil {
		return vault.ID{}, false
	}
	defer clear(container)
	id, err := vault.InspectUntrustedVaultID(container)
	return id, err == nil
}
