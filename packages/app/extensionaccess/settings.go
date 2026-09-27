package extensionaccess

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// Preferences is the desktop app's recorded choices as a linked extension follows them.
type Preferences interface {
	Language() (preferences.Language, bool)
	SignInStyle() preferences.SignInStyle
}

// Settings is the link server's Settings.
type Settings struct {
	preferences Preferences
}

// NewSettings composes Settings over the recorded preferences.
func NewSettings(recorded Preferences) (*Settings, error) {
	if recorded == nil {
		return nil, errors.New("preferences are required")
	}
	return &Settings{preferences: recorded}, nil
}

// Language is the language in use.
func (s *Settings) Language() string {
	language, _ := s.preferences.Language()
	return string(language)
}

// SignInStyle is the recorded sign-in style.
func (s *Settings) SignInStyle() string {
	return string(s.preferences.SignInStyle())
}
