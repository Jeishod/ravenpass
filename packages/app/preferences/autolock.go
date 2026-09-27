package preferences

import (
	"errors"
	"slices"
	"time"
)

// AutoLock locks an open vault once the device is idle, or the app hidden, for After.
type AutoLock struct {
	Enabled bool
	After   time.Duration
}

// autoLockChoices are a host's offered delays in seconds, shortest first, and its default.
type autoLockChoices struct {
	whenHidden bool
	offered    []int
	fallback   int
}

func (c autoLockChoices) offers(seconds int) bool { return slices.Contains(c.offered, seconds) }

var (
	// idleAutoLock counts from the device's last keyboard, mouse or trackpad input.
	idleAutoLock = autoLockChoices{offered: []int{60, 5 * 60, 15 * 60, 30 * 60, 60 * 60}, fallback: 15 * 60}
	// hiddenAutoLock counts from the moment the app is hidden.
	hiddenAutoLock = autoLockChoices{whenHidden: true, offered: []int{0, 60, 5 * 60, 15 * 60, 30 * 60, 60 * 60}, fallback: 0}
)

// ErrUnsupportedAutoLockDelay reports a delay the host does not offer.
var ErrUnsupportedAutoLockDelay = errors.New("unsupported auto-lock delay")

// Option adapts a Store to its host.
type Option func(*Store)

// LockWhenHidden counts the lock delay from when the app is hidden, defaulting to locking at once.
func LockWhenHidden() Option {
	return func(s *Store) { s.autoLock = hiddenAutoLock }
}

// LocksWhenHidden reports whether the store was made with LockWhenHidden.
func (s *Store) LocksWhenHidden() bool { return s.autoLock.whenHidden }

// AutoLockDelays lists the delays the user may choose, shortest first.
func (s *Store) AutoLockDelays() []time.Duration {
	delays := make([]time.Duration, len(s.autoLock.offered))
	for i, seconds := range s.autoLock.offered {
		delays[i] = time.Duration(seconds) * time.Second
	}
	return delays
}

// AutoLock reports the recorded choice, or the host's default where none was recorded.
func (s *Store) AutoLock() AutoLock {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	seconds := s.autoLock.fallback
	if s.current.AutoLockSeconds != nil {
		seconds = *s.current.AutoLockSeconds
	}
	return AutoLock{Enabled: !s.current.AutoLockOff, After: time.Duration(seconds) * time.Second}
}

// SetAutoLock records whether the vault locks on its own and the delay, which is kept while off.
func (s *Store) SetAutoLock(lock AutoLock) error {
	seconds := int(lock.After / time.Second)
	if lock.After%time.Second != 0 || !s.autoLock.offers(seconds) {
		return ErrUnsupportedAutoLockDelay
	}
	return s.update(func(next *record) {
		next.AutoLockOff = !lock.Enabled
		next.AutoLockSeconds = &seconds
	})
}

// keepOfferedAutoLock forgets a recorded delay this host does not offer, so its default applies.
func (s *Store) keepOfferedAutoLock(stored *record) {
	if stored.AutoLockSeconds != nil && !s.autoLock.offers(*stored.AutoLockSeconds) {
		stored.AutoLockSeconds = nil
	}
}
