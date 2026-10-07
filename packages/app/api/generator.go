package api

import (
	"errors"
	"time"

	"github.com/dortanes/ravenpass/packages/app/genhistory"
	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// GeneratedPassword is one password the generator handed out; Mode is "words" or "characters" and At is RFC 3339 in
// UTC.
type GeneratedPassword struct {
	Value string `json:"value"`
	Mode  string `json:"mode"`
	At    string `json:"at"`
}

// GeneratorHistorySetting is whether the generator's history is kept, for how many days, and the periods offered.
type GeneratorHistorySetting struct {
	Enabled bool  `json:"enabled"`
	Days    int   `json:"days"`
	Offered []int `json:"offered"`
}

// GeneratorHistory reads the passwords the generator handed out for the open vault, newest first.
func (s *Service) GeneratorHistory() ([]GeneratedPassword, error) {
	if s.generator == nil {
		return nil, fail(failureGeneral)
	}
	entries, err := s.generator.History()
	if err != nil {
		return nil, presentGenerator(err)
	}
	history := make([]GeneratedPassword, len(entries))
	for i, entry := range entries {
		history[i] = generated(entry)
	}
	return history, nil
}

// RecordGeneratedPassword adds a password the generator handed out in mode to the open vault's history, unless the
// history is turned off.
func (s *Service) RecordGeneratedPassword(value, mode string) error {
	if s.generator == nil {
		return fail(failureGeneral)
	}
	if !s.preferences.GeneratorHistory().Enabled {
		return nil
	}
	_, err := s.generator.Record(value, mode)
	return presentGenerator(err)
}

// ClearGeneratorHistory forgets every password the generator handed out for the open vault.
func (s *Service) ClearGeneratorHistory() error {
	if s.generator == nil {
		return fail(failureGeneral)
	}
	return presentGenerator(s.generator.Clear())
}

// CopyGeneratedPassword puts a generated password on the clipboard, cleared like any other copied secret.
func (s *Service) CopyGeneratedPassword(value string) error {
	if value == "" {
		return fail(failureFieldEmpty)
	}
	if !s.copyToClipboard(value, clearAfter) {
		return fail(failureCopyFailed)
	}
	return nil
}

// GetGeneratorHistorySetting reports whether the generator's history is kept and for how many days.
func (s *Service) GetGeneratorHistorySetting() (GeneratorHistorySetting, error) {
	history := s.preferences.GeneratorHistory()
	return GeneratorHistorySetting{
		Enabled: history.Enabled,
		Days:    history.Days,
		Offered: preferences.GeneratorHistoryPeriods(),
	}, nil
}

// CountGeneratorHistoryPast counts the passwords in the open vault's history that a period of days would remove.
func (s *Service) CountGeneratorHistoryPast(days int) (int, error) {
	if s.generator == nil {
		return 0, fail(failureGeneral)
	}
	count, err := s.generator.Past(days)
	if err != nil {
		return 0, presentGenerator(err)
	}
	return count, nil
}

// SetGeneratorHistorySetting records whether the generator's history is kept and for how many days, then removes from
// the open vault's history what it no longer keeps: everything when turned off, else what is older than days. A
// shorter period's removal that fails once the setting is saved is not reported: the next reading of the history
// retries it.
func (s *Service) SetGeneratorHistorySetting(enabled bool, days int) error {
	history := preferences.GeneratorHistory{Enabled: enabled, Days: days}
	if err := s.preferences.SetGeneratorHistory(history); err != nil {
		if errors.Is(err, preferences.ErrUnsupportedGeneratorHistoryDays) {
			return fail(failureInvalidItem)
		}
		return present(err)
	}
	if s.generator == nil {
		return nil
	}
	if !enabled {
		return presentGenerator(s.generator.Clear())
	}
	_ = s.generator.Prune()
	return nil
}

func generated(entry genhistory.Entry) GeneratedPassword {
	return GeneratedPassword{Value: entry.Value, Mode: entry.Mode, At: entry.At.UTC().Format(time.RFC3339)}
}

func presentGenerator(err error) error {
	if errors.Is(err, genhistory.ErrInvalidEntry) {
		return fail(failureInvalidItem)
	}
	return present(err)
}
