package api

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// Appearance reports whether the interface is light, dark or follows the system, and the appearances offered.
type Appearance struct {
	Appearance string   `json:"appearance"`
	Offered    []string `json:"offered"`
}

// GetAppearance reports the appearance and the appearances offered.
func (s *Service) GetAppearance() (Appearance, error) {
	appearances := preferences.Appearances()
	offered := make([]string, len(appearances))
	for i, appearance := range appearances {
		offered[i] = string(appearance)
	}
	return Appearance{Appearance: string(s.preferences.Appearance()), Offered: offered}, nil
}

// SetAppearance records an offered appearance; the host's windows and the interface apply it.
func (s *Service) SetAppearance(appearance string) error {
	if err := s.preferences.SetAppearance(preferences.Appearance(appearance)); err != nil {
		if errors.Is(err, preferences.ErrUnsupportedAppearance) {
			return fail(failureAppearanceUnsupported)
		}
		return present(err)
	}
	return nil
}
