package api

import (
	"context"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// LanguageSettings reports the languages Ravenpass offers and the one in use.
type LanguageSettings struct {
	Languages []string `json:"languages"`
	// Language is, until one is chosen, the device's language when offered, else English.
	Language string `json:"language"`
	Chosen   bool   `json:"chosen"`
}

// GetLanguage reports the languages offered and the one in use.
func (s *Service) GetLanguage() (LanguageSettings, error) {
	language, chosen := s.preferences.Language()
	offered := preferences.Languages()
	languages := make([]string, len(offered))
	for i, tag := range offered {
		languages[i] = string(tag)
	}
	return LanguageSettings{Languages: languages, Language: string(language), Chosen: chosen}, nil
}

// SetLanguage records one of the offered languages.
func (s *Service) SetLanguage(language string) error {
	if err := s.preferences.SetLanguage(preferences.Language(language)); err != nil {
		return fail(failureLanguageUnsupported)
	}
	return nil
}

// AwaitLanguageChange returns the count of language choices since start once it passes seen.
func (s *Service) AwaitLanguageChange(ctx context.Context, seen uint64) (uint64, error) {
	count, err := s.preferences.AwaitLanguageChange(ctx, seen)
	if err != nil {
		return 0, present(err)
	}
	return count, nil
}
