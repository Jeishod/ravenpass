package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// ReadSelectedCard returns the card ticket selects.
func (s *Service) ReadSelectedCard(ticket vault.Selection) (vault.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.Card{}, ErrNotReady
	}
	return s.session.ReadSelectedCard(ticket)
}

// CreateCard saves a new card in groups and returns its ID.
func (s *Service) CreateCard(input vault.CardInput, groups []vault.ID) (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	pending, id, err := s.session.PrepareCreateCard(input, groups)
	if err != nil {
		return vault.ID{}, err
	}
	if err := s.commit(pending); err != nil {
		return vault.ID{}, err
	}
	return id, nil
}

// EditCard replaces a card's content and membership as a whole.
func (s *Service) EditCard(id vault.ID, input vault.CardInput, groups []vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareEditCard(id, input, groups)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// IdentityAddresses lists what a card's billing address can link to.
func (s *Service) IdentityAddresses() ([]vault.IdentityAddresses, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, ErrNotReady
	}
	return s.session.IdentityAddresses()
}
