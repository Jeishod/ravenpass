package preferences

// DockHiddenWithWindow reports whether Ravenpass leaves the Dock while its window is closed; off by default.
func (s *Store) DockHiddenWithWindow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return s.current.HideDockWithWindow
}

// SetDockHiddenWithWindow records whether Ravenpass leaves the Dock while its window is closed.
func (s *Store) SetDockHiddenWithWindow(hidden bool) error {
	return s.update(func(next *record) { next.HideDockWithWindow = hidden })
}
