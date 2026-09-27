package preferences

import (
	"errors"
	"slices"
	"time"
)

// ClipboardClearing clears a copied value After the delay, when Enabled and the clipboard still holds it.
type ClipboardClearing struct {
	Enabled bool
	After   time.Duration
}

const defaultClearSeconds = 60

var clearDelaySeconds = []int{15, 30, 60, 120, 300}

// ErrUnsupportedClearDelay reports a clearing delay this build does not offer.
var ErrUnsupportedClearDelay = errors.New("unsupported clipboard clearing delay")

// ClearDelays lists the delays the user may choose, shortest first.
func ClearDelays() []time.Duration {
	delays := make([]time.Duration, len(clearDelaySeconds))
	for i, seconds := range clearDelaySeconds {
		delays[i] = time.Duration(seconds) * time.Second
	}
	return delays
}

// DefaultClipboardClearing is what a device that never made the choice uses.
func DefaultClipboardClearing() ClipboardClearing {
	return ClipboardClearing{Enabled: true, After: defaultClearSeconds * time.Second}
}

func offeredClearDelay(seconds int) bool {
	return slices.Contains(clearDelaySeconds, seconds)
}

// ClipboardClearing reports the recorded choice, or the default where none was recorded.
func (s *Store) ClipboardClearing() ClipboardClearing {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	clearing := DefaultClipboardClearing()
	clearing.Enabled = !s.current.ClipboardKept
	if s.current.ClipboardClearSeconds != 0 {
		clearing.After = time.Duration(s.current.ClipboardClearSeconds) * time.Second
	}
	return clearing
}

// SetClipboardClearing records whether copies clear and the delay, which is kept while off.
func (s *Store) SetClipboardClearing(clearing ClipboardClearing) error {
	if clearing.After%time.Second != 0 || !offeredClearDelay(int(clearing.After/time.Second)) {
		return ErrUnsupportedClearDelay
	}
	return s.update(func(next *record) {
		next.ClipboardKept = !clearing.Enabled
		next.ClipboardClearSeconds = int(clearing.After / time.Second)
	})
}
