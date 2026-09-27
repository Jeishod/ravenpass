package preferences

// IdentityList reports whether the system keeps the vault's sites and user names for its AutoFill; off by default.
func (s *Store) IdentityList() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return s.current.IdentityListOn
}

// SetIdentityList records whether the system keeps the vault's accounts for its AutoFill.
func (s *Store) SetIdentityList(enabled bool) error {
	return s.update(func(next *record) { next.IdentityListOn = enabled })
}
