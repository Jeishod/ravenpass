package api

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// SignInStyle reports how a linked extension offers sign-in on websites and the styles offered.
type SignInStyle struct {
	Style   string   `json:"style"`
	Offered []string `json:"offered"`
}

// GetSignInStyle reports the sign-in style and the styles offered.
func (s *Service) GetSignInStyle() (SignInStyle, error) {
	styles := preferences.SignInStyles()
	offered := make([]string, len(styles))
	for i, style := range styles {
		offered[i] = string(style)
	}
	return SignInStyle{Style: string(s.preferences.SignInStyle()), Offered: offered}, nil
}

// SetSignInStyle records the style, applied from the next extension request.
func (s *Service) SetSignInStyle(style string) error {
	if err := s.preferences.SetSignInStyle(preferences.SignInStyle(style)); err != nil {
		if errors.Is(err, preferences.ErrUnsupportedSignInStyle) {
			return fail(failureSignInStyleUnsupported)
		}
		return present(err)
	}
	return nil
}
