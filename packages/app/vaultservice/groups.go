package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// Groups reports the groups the open vault holds, in the order they were created.
func (s *Service) Groups() ([]vault.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, ErrNotReady
	}
	return s.session.Groups()
}

// DefaultGroup resolves the recorded group ID new passwords join, reporting false once the group is gone.
func (s *Service) DefaultGroup(recorded string) (vault.ID, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.defaultGroup(recorded)
}

// defaultGroup is DefaultGroup for a caller that holds s.mu.
func (s *Service) defaultGroup(recorded string) (vault.ID, bool, error) {
	if recorded == "" {
		return vault.ID{}, false, nil
	}
	if s.session == nil {
		return vault.ID{}, false, ErrNotReady
	}
	groups, err := s.session.Groups()
	if err != nil {
		return vault.ID{}, false, err
	}
	for _, group := range groups {
		if group.ID.String() == recorded {
			return group.ID, true, nil
		}
	}
	return vault.ID{}, false, nil
}

// CreateGroup adds a group named name and returns its ID.
func (s *Service) CreateGroup(name string) (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	pending, id, err := s.session.PrepareCreateGroup(name)
	if err != nil {
		return vault.ID{}, err
	}
	if err := s.commit(pending); err != nil {
		return vault.ID{}, err
	}
	return id, nil
}

// RenameGroup fails with vault.ErrNotFound for an unknown group and vault.ErrInvalidInput for a blank, overlong or taken name.
func (s *Service) RenameGroup(id vault.ID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareRenameGroup(id, name)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// DeleteGroup also drops the group from every item's membership; an unknown group fails with vault.ErrNotFound.
func (s *Service) DeleteGroup(id vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareDeleteGroup(id)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// SetItemGroups replaces the membership of an item of either kind.
func (s *Service) SetItemGroups(id vault.ID, groups []vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareSetGroups(id, groups)
	if err != nil {
		return err
	}
	return s.commit(pending)
}
