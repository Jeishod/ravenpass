package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// AddItems adds items of several kinds and the groups they join in one save.
func (s *Service) AddItems(items []vault.NewItem) (vault.BatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.BatchResult{}, ErrNotReady
	}
	pending, result, err := s.session.PrepareAddItems(items)
	if err != nil {
		return vault.BatchResult{}, err
	}
	if err := s.commit(pending); err != nil {
		return vault.BatchResult{}, err
	}
	return result, nil
}
