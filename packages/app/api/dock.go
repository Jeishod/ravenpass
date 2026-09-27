package api

// DockIcon reports whether Ravenpass leaves the Dock while its window is closed.
type DockIcon struct {
	HideWithWindow bool `json:"hideWithWindow"`
}

// GetDockIcon reports whether the Dock icon hides with the window.
func (s *Service) GetDockIcon() (DockIcon, error) {
	return DockIcon{HideWithWindow: s.preferences.DockHiddenWithWindow()}, nil
}

// SetDockIcon records whether the Dock icon hides with the window, from the next close on.
func (s *Service) SetDockIcon(hideWithWindow bool) error {
	return present(s.preferences.SetDockHiddenWithWindow(hideWithWindow))
}
