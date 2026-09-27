package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// ScansOf reports the scans an identity holds.
func (s *Service) ScansOf(owner vault.ID) ([]vault.ScanSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, ErrNotReady
	}
	return s.session.ScansOf(owner)
}

// ReadScan returns the scan id with its content.
func (s *Service) ReadScan(id vault.ID) (vault.Scan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.Scan{}, ErrNotReady
	}
	return s.session.ReadScan(id)
}
