package preferences

// ConfirmExtensionFills reports whether each password or code a linked extension fills waits for the owner to
// confirm it, which it does not until the owner turns it on.
func (s *Store) ConfirmExtensionFills() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return s.current.ConfirmExtensionFills
}

// SetConfirmExtensionFills records whether fills a linked extension asks for wait for the owner to confirm them.
func (s *Store) SetConfirmExtensionFills(confirm bool) error {
	return s.update(func(next *record) { next.ConfirmExtensionFills = confirm })
}
