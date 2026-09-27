package preferences

// ScreenshotsAllowed reports whether the user allows screenshots of the main window.
func (s *Store) ScreenshotsAllowed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return s.current.ScreenshotsAllowed
}

// SetScreenshotsAllowed records whether the user allows screenshots of the main window.
func (s *Store) SetScreenshotsAllowed(allowed bool) error {
	return s.update(func(next *record) { next.ScreenshotsAllowed = allowed })
}
