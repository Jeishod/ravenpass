package autofill

import "sync"

// screens holds the automatic lock while any autofill screen is shown; screens can overlap.
type screens struct {
	hold func() (release func())

	mu      sync.Mutex
	shown   int
	release func()
}

func (s *screens) show() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shown == 0 {
		s.release = s.hold()
	}
	s.shown++
}

func (s *screens) hide() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shown == 0 {
		return
	}
	s.shown--
	if s.shown == 0 {
		s.release()
		s.release = nil
	}
}
