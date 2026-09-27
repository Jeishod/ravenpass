package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// LinkApp links credential id to Android app pkg for each signer digest it does not link yet.
func (s *Service) LinkApp(id vault.ID, pkg string, signers [][32]byte) error {
	if pkg == "" || len(signers) == 0 {
		return vault.ErrInvalidInput
	}
	changed, err := s.linkApp(id, pkg, signers)
	if err != nil || !changed {
		return err
	}
	s.changes.Record()
	return nil
}

// linkApp is LinkApp under one hold of the lock, reporting whether the credential changed.
func (s *Service) linkApp(id vault.ID, pkg string, signers [][32]byte) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return false, ErrNotReady
	}
	credential, err := s.session.ReadCredential(id)
	if err != nil {
		return false, err
	}
	apps := AppRequester(pkg, signers, nil).linked(credential.Apps)
	if len(apps) == len(credential.Apps) {
		return false, nil
	}
	return true, s.editCredential(id, vault.CredentialPatch{Apps: &apps})
}
