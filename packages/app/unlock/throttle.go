package unlock

import "time"

// Throttle refuses a vault's PIN attempts until their earned delay passes; the caller serializes its use.
type Throttle struct {
	now       func() time.Time
	notBefore map[string]time.Time
}

// NewThrottle returns a throttle that reads the time from now.
func NewThrottle(now func() time.Time) *Throttle {
	return &Throttle{now: now, notBefore: make(map[string]time.Time)}
}

// Allow fails with ErrTooSoon while the vault's delay runs.
func (t *Throttle) Allow(vaultID string) error {
	if t.now().Before(t.notBefore[vaultID]) {
		return ErrTooSoon
	}
	return nil
}

// Record starts the delay the vault's count of wrong PINs in a row earns, and clears it at zero.
func (t *Throttle) Record(vaultID string, failures int) {
	delay := AttemptDelay(failures)
	if delay == 0 {
		delete(t.notBefore, vaultID)
		return
	}
	t.notBefore[vaultID] = t.now().Add(delay)
}
