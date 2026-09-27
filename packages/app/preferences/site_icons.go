package preferences

// SiteIcons reports whether website icons load, which they do until the user turns them off.
func (s *Store) SiteIcons() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return !s.current.SiteIconsOff
}

// SetSiteIcons records whether website icons load.
func (s *Store) SetSiteIcons(enabled bool) error {
	return s.update(func(next *record) { next.SiteIconsOff = !enabled })
}
