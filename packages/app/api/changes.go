package api

import (
	"context"
)

// AwaitVaultChange returns the count of changes made outside the interface once it passes seen.
func (s *Service) AwaitVaultChange(ctx context.Context, seen uint64) (uint64, error) {
	count, err := s.vault.AwaitVaultChange(ctx, seen)
	if err != nil {
		return 0, present(err)
	}
	return count, nil
}

// AdoptChangedVault opens a vault whose unlock failed as diverged, discarding this device's latest changes.
func (s *Service) AdoptChangedVault() error {
	_, err := s.vault.AdoptDiverged()
	return s.opened(err)
}
