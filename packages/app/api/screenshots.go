package api

// ScreenCapture keeps the host's windows out of screenshots, recordings and the recent apps list.
type ScreenCapture interface {
	// AllowScreenshots lets the owner allow screenshots of the main window.
	AllowScreenshots(allowed bool)
}

type noScreenCapture struct{}

func (noScreenCapture) AllowScreenshots(bool) {}

// ScreenshotsAllowed reports whether the owner allows screenshots of the main window.
func (s *Service) ScreenshotsAllowed() (bool, error) {
	return s.preferences.ScreenshotsAllowed(), nil
}

// AllowScreenshots records and applies whether the main window may be captured; other windows never may.
func (s *Service) AllowScreenshots(allowed bool) error {
	if err := s.preferences.SetScreenshotsAllowed(allowed); err != nil {
		return present(err)
	}
	s.screens.AllowScreenshots(allowed)
	return nil
}
