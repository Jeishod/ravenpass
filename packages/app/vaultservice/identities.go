package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// ReadSelectedIdentity returns the identity ticket selects.
func (s *Service) ReadSelectedIdentity(ticket vault.Selection) (vault.Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.Identity{}, ErrNotReady
	}
	return s.session.ReadSelectedIdentity(ticket)
}

// CreateIdentity saves a new identity in groups and returns its ID.
func (s *Service) CreateIdentity(input vault.IdentityInput, groups []vault.ID) (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	pending, id, err := s.session.PrepareCreateIdentity(input, groups)
	if err != nil {
		return vault.ID{}, err
	}
	if err := s.commit(pending); err != nil {
		return vault.ID{}, err
	}
	return id, nil
}

// EditIdentity replaces an identity's content and membership as a whole.
func (s *Service) EditIdentity(id vault.ID, input vault.IdentityInput, groups []vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareEditIdentity(id, input, groups)
	if err != nil {
		return err
	}
	return s.commit(pending)
}
