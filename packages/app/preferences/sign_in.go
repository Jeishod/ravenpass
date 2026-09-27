package preferences

import (
	"errors"
	"slices"
)

// SignInStyle is how a linked extension offers sign-in on websites.
type SignInStyle string

const (
	// SignInCard shows a card in the page's corner as soon as the page asks to sign in.
	SignInCard SignInStyle = "card"
	// SignInField shows a menu under the sign-in field that takes focus.
	SignInField SignInStyle = "field"
)

// ErrUnsupportedSignInStyle reports a style this build does not offer.
var ErrUnsupportedSignInStyle = errors.New("unsupported sign-in style")

// SignInStyles lists the styles this build offers, in the order the interface shows them.
func SignInStyles() []SignInStyle { return []SignInStyle{SignInCard, SignInField} }

func offeredSignInStyle(style SignInStyle) bool {
	return slices.Contains(SignInStyles(), style)
}

// SignInStyle reports the recorded style, SignInCard until the user chooses another.
func (s *Store) SignInStyle() SignInStyle {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if s.current.SignInStyle == "" {
		return SignInCard
	}
	return s.current.SignInStyle
}

// SetSignInStyle records an offered style.
func (s *Store) SetSignInStyle(style SignInStyle) error {
	if !offeredSignInStyle(style) {
		return ErrUnsupportedSignInStyle
	}
	return s.update(func(next *record) { next.SignInStyle = style })
}
