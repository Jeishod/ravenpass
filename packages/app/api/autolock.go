package api

import (
	"errors"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// AutoLock is the automatic lock setting and the delays offered.
type AutoLock struct {
	Enabled bool `json:"enabled"`
	// Seconds counts from the last input, or from hiding the app under LockWhenHidden, where zero locks at once.
	Seconds int   `json:"seconds"`
	Offered []int `json:"offered"`
}

// GetAutoLock reports the automatic lock setting.
func (s *Service) GetAutoLock() (AutoLock, error) {
	lock := s.preferences.AutoLock()
	delays := s.preferences.AutoLockDelays()
	offered := make([]int, len(delays))
	for i, delay := range delays {
		offered[i] = int(delay / time.Second)
	}
	return AutoLock{Enabled: lock.Enabled, Seconds: int(lock.After / time.Second), Offered: offered}, nil
}

// SetAutoLock records the automatic lock setting; seconds must be an offered delay.
func (s *Service) SetAutoLock(enabled bool, seconds int) error {
	lock := preferences.AutoLock{Enabled: enabled, After: time.Duration(seconds) * time.Second}
	if err := s.preferences.SetAutoLock(lock); err != nil {
		if errors.Is(err, preferences.ErrUnsupportedAutoLockDelay) {
			return fail(failureAutoLockDelayUnsupported)
		}
		return present(err)
	}
	return nil
}
