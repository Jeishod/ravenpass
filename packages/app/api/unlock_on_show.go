package api

import "sync"

// UnlockOnShow decides whether the locked screen prompts for device unlock as the app comes into view.
type UnlockOnShow struct {
	mu       sync.Mutex
	inView   bool
	heldBack bool
}

// Shown reports the app in view.
func (u *UnlockOnShow) Shown() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.inView = true
}

// Hidden reports the app out of view, so the locked screen asks again once the app returns.
func (u *UnlockOnShow) Hidden() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.inView = false
	u.heldBack = false
}

// locked holds the prompt back until the app next leaves view when the lock was made in view.
func (u *UnlockOnShow) locked() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.inView {
		u.heldBack = true
	}
}

func (u *UnlockOnShow) prompts() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return !u.heldBack
}

// PromptsUnlock reports whether the locked screen may prompt for device unlock now.
func (s *Service) PromptsUnlock() bool { return s.shows.prompts() }
