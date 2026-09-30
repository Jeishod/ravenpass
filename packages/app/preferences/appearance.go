package preferences

import (
	"errors"
	"slices"
)

// Appearance is whether the interface is light, dark or follows the system.
type Appearance string

const (
	// AppearanceSystem follows the system's light or dark mode.
	AppearanceSystem Appearance = "system"
	// AppearanceLight is light whatever the system's mode.
	AppearanceLight Appearance = "light"
	// AppearanceDark is dark whatever the system's mode.
	AppearanceDark Appearance = "dark"
)

// ErrUnsupportedAppearance reports an appearance this build does not offer.
var ErrUnsupportedAppearance = errors.New("unsupported appearance")

// Appearances lists the appearances this build offers, in the order the interface shows them.
func Appearances() []Appearance {
	return []Appearance{AppearanceSystem, AppearanceLight, AppearanceDark}
}

// Appearance reports the recorded appearance, AppearanceSystem until the user chooses another.
func (s *Store) Appearance() Appearance {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if !slices.Contains(Appearances(), s.current.Appearance) {
		return AppearanceSystem
	}
	return s.current.Appearance
}

// SetAppearance records an offered appearance.
func (s *Store) SetAppearance(appearance Appearance) error {
	if !slices.Contains(Appearances(), appearance) {
		return ErrUnsupportedAppearance
	}
	if err := s.update(func(next *record) { next.Appearance = appearance }); err != nil {
		return err
	}
	s.notify(&s.appearanceListeners)
	return nil
}

// OnAppearanceChange calls listener after each appearance the user records.
func (s *Store) OnAppearanceChange(listener func()) {
	s.listen(&s.appearanceListeners, listener)
}
