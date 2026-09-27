package preferences

// BankDetails reports whether a card's bank name and colour are looked up on the bank's site; on by default.
func (s *Store) BankDetails() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return !s.current.BankDetailsOff
}

// SetBankDetails records whether a card's bank details are looked up.
func (s *Store) SetBankDetails(enabled bool) error {
	return s.update(func(next *record) { next.BankDetailsOff = !enabled })
}
