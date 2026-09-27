package vaultservice

import "context"

// AwaitVaultChange blocks until the count of changes the interface did not make passes seen.
func (s *Service) AwaitVaultChange(ctx context.Context, seen uint64) (uint64, error) {
	return s.changes.Await(ctx, seen)
}

// RecordOutsideChange counts a change to the open vault the interface did not make.
func (s *Service) RecordOutsideChange() {
	s.changes.Record()
}
