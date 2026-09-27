package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// OpenVaultID reports the identifier of the open vault.
func (s *Service) OpenVaultID() (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	head, err := s.session.Head()
	return head.VaultID, err
}

// SealDeviceCache seals device cache data with the open vault, or fails with ErrNotReady unless it is id.
func (s *Service) SealDeviceCache(id vault.ID, name string, plaintext []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(id); err != nil {
		return nil, err
	}
	return s.session.SealDeviceCache(name, plaintext)
}

// OpenDeviceCache opens device cache data with the open vault, or fails with ErrNotReady unless it is id.
func (s *Service) OpenDeviceCache(id vault.ID, name string, sealed []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(id); err != nil {
		return nil, err
	}
	return s.session.OpenDeviceCache(name, sealed)
}

func (s *Service) requireOpen(id vault.ID) error {
	if s.session == nil {
		return ErrNotReady
	}
	head, err := s.session.Head()
	if err != nil {
		return err
	}
	if head.VaultID != id {
		return ErrNotReady
	}
	return nil
}
