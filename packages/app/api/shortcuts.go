package api

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// GetShortcuts reports changed hotkeys by action; an action missing here keeps the interface's default.
func (s *Service) GetShortcuts() (map[string]string, error) {
	shortcuts := s.preferences.Shortcuts()
	if shortcuts == nil {
		shortcuts = map[string]string{}
	}
	return shortcuts, nil
}

// SetShortcut records the hotkey for an action; an empty hotkey restores the default.
func (s *Service) SetShortcut(action, hotkey string) error {
	if err := s.preferences.SetShortcut(action, hotkey); err != nil {
		if errors.Is(err, preferences.ErrInvalidShortcut) {
			return fail(failureShortcutInvalid)
		}
		return present(err)
	}
	return nil
}
