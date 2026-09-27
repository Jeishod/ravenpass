package preferences

import (
	"errors"
	"maps"
	"regexp"
)

// Shortcut bounds keep every recorded binding inside the preferences record's size limit.
const (
	maxShortcuts         = 16
	maxShortcutActionLen = 32
	maxShortcutHotkeyLen = 48
)

var (
	// ErrInvalidShortcut reports a malformed action or hotkey, or more than maxShortcuts.
	ErrInvalidShortcut = errors.New("invalid shortcut")

	shortcutAction = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Shortcuts reports the hotkeys the user changed, by action, stored as written and never interpreted.
func (s *Store) Shortcuts() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return maps.Clone(s.current.Shortcuts)
}

// SetShortcut records the hotkey for an action; an empty hotkey restores its default.
func (s *Store) SetShortcut(action, hotkey string) error {
	if !validShortcutAction(action) || (hotkey != "" && !validShortcutHotkey(hotkey)) {
		return ErrInvalidShortcut
	}
	return s.apply(func(next *record) error {
		shortcuts := maps.Clone(next.Shortcuts)
		if hotkey == "" {
			delete(shortcuts, action)
		} else {
			if shortcuts == nil {
				shortcuts = map[string]string{}
			}
			shortcuts[action] = hotkey
		}
		if len(shortcuts) > maxShortcuts {
			return ErrInvalidShortcut
		}
		if len(shortcuts) == 0 {
			shortcuts = nil
		}
		next.Shortcuts = shortcuts
		return nil
	})
}

func validShortcutAction(action string) bool {
	return len(action) <= maxShortcutActionLen && shortcutAction.MatchString(action)
}

func validShortcutHotkey(hotkey string) bool {
	if hotkey == "" || len(hotkey) > maxShortcutHotkeyLen {
		return false
	}
	for _, r := range hotkey {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// validShortcuts tells whether a stored record's shortcuts all keep within the bounds.
func validShortcuts(shortcuts map[string]string) bool {
	if len(shortcuts) > maxShortcuts {
		return false
	}
	for action, hotkey := range shortcuts {
		if !validShortcutAction(action) || !validShortcutHotkey(hotkey) {
			return false
		}
	}
	return true
}
