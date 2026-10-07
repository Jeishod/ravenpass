package preferences

import (
	"errors"
	"slices"
)

// GeneratorHistory keeps each password the generator hands out for Days, when Enabled.
type GeneratorHistory struct {
	Enabled bool
	Days    int
}

const defaultGeneratorHistoryDays = 30

var generatorHistoryDays = []int{7, 30, 90, 365}

// ErrUnsupportedGeneratorHistoryDays reports a period of the generator's history this build does not offer.
var ErrUnsupportedGeneratorHistoryDays = errors.New("unsupported generator history period")

// GeneratorHistoryPeriods lists the periods, in days, the generator's history may keep a password, shortest first.
func GeneratorHistoryPeriods() []int {
	return slices.Clone(generatorHistoryDays)
}

func offeredGeneratorHistoryDays(days int) bool {
	return slices.Contains(generatorHistoryDays, days)
}

// GeneratorHistory reports the recorded choice, or a history kept for the default period where none was recorded.
func (s *Store) GeneratorHistory() GeneratorHistory {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	history := GeneratorHistory{Enabled: !s.current.GeneratorHistoryOff, Days: defaultGeneratorHistoryDays}
	if s.current.GeneratorHistoryDays != 0 {
		history.Days = s.current.GeneratorHistoryDays
	}
	return history
}

// SetGeneratorHistory records whether the generator's history is kept and its period, which is kept while off.
func (s *Store) SetGeneratorHistory(history GeneratorHistory) error {
	if !offeredGeneratorHistoryDays(history.Days) {
		return ErrUnsupportedGeneratorHistoryDays
	}
	return s.update(func(next *record) {
		next.GeneratorHistoryOff = !history.Enabled
		next.GeneratorHistoryDays = history.Days
	})
}
