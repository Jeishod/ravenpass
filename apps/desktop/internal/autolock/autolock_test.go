package autolock

import (
	"errors"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

type scripted struct {
	idle    time.Duration
	err     error
	setting preferences.AutoLock
	locks   int
}

func (s *scripted) watcher() *Watcher {
	return &Watcher{
		idle:     func() (time.Duration, error) { return s.idle, s.err },
		settings: func() preferences.AutoLock { return s.setting },
		lock:     func() { s.locks++ },
	}
}

func TestLocksOnceTheIdleTimeReachesTheDelay(t *testing.T) {
	s := &scripted{setting: preferences.AutoLock{Enabled: true, After: 5 * time.Minute}}
	w := s.watcher()
	s.idle = 5*time.Minute - time.Second
	w.check()
	if s.locks != 0 {
		t.Fatalf("locked before the delay: %d", s.locks)
	}
	s.idle = 5 * time.Minute
	w.check()
	if s.locks != 1 {
		t.Fatalf("locks at the delay = %d", s.locks)
	}
}

func TestLocksOncePerIdlePeriod(t *testing.T) {
	s := &scripted{setting: preferences.AutoLock{Enabled: true, After: time.Minute}}
	w := s.watcher()
	for _, idle := range []time.Duration{time.Minute, 2 * time.Minute, 10 * time.Minute} {
		s.idle = idle
		w.check()
	}
	if s.locks != 1 {
		t.Fatalf("locks during one idle period = %d", s.locks)
	}
	s.idle = time.Second
	w.check()
	s.idle = time.Minute
	w.check()
	if s.locks != 2 {
		t.Fatalf("locks after input and a second idle period = %d", s.locks)
	}
}

func TestNeverLocksWhileOff(t *testing.T) {
	s := &scripted{setting: preferences.AutoLock{Enabled: false, After: time.Minute}, idle: time.Hour}
	s.watcher().check()
	if s.locks != 0 {
		t.Fatalf("locked while auto-lock is off: %d", s.locks)
	}
}

func TestUnreadableIdleTimeNeverLocks(t *testing.T) {
	s := &scripted{
		setting: preferences.AutoLock{Enabled: true, After: time.Minute},
		idle:    time.Hour,
		err:     errors.New("unsupported platform"),
	}
	s.watcher().check()
	if s.locks != 0 {
		t.Fatalf("locked without an idle time: %d", s.locks)
	}
}
