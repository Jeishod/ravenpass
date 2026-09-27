package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// ReadSelectedSeed returns the seed ticket selects.
func (s *Service) ReadSelectedSeed(ticket vault.Selection) (vault.Seed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.Seed{}, ErrNotReady
	}
	return s.session.ReadSelectedSeed(ticket)
}

// CreateSeed saves a new seed in groups and returns its ID.
func (s *Service) CreateSeed(input vault.SeedInput, groups []vault.ID) (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	pending, id, err := s.session.PrepareCreateSeed(input, groups)
	if err != nil {
		return vault.ID{}, err
	}
	if err := s.commit(pending); err != nil {
		return vault.ID{}, err
	}
	return id, nil
}

// EditSeed replaces a seed's content and membership as a whole.
func (s *Service) EditSeed(id vault.ID, input vault.SeedInput, groups []vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareEditSeed(id, input, groups)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// SpendBackupCode marks one unused code of a backup codes seed used.
func (s *Service) SpendBackupCode(id vault.ID, index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareUseBackupCode(id, index)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// RecordSeedCheck sets the day, as YYYY-MM-DD, a phrase seed's copy was last confirmed.
func (s *Service) RecordSeedCheck(id vault.ID, on string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareRecordSeedCheck(id, on)
	if err != nil {
		return err
	}
	return s.commit(pending)
}
