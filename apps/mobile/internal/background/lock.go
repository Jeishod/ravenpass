// Package background locks the vault once the app has stayed hidden for the owner's automatic lock delay.
package background

import (
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// releaseGrace is the shortest countdown after a released hold: MainActivity's start event can reach Go after it.
const releaseGrace = 2 * time.Second

// autofillSession is how long an autofill session keeps the vault open after its last request.
const autofillSession = 3 * time.Minute

// Lock counts down while the app is hidden and locks when the delay passes.
type Lock struct {
	settings func() preferences.AutoLock
	lock     func()
	after    func(time.Duration, func()) (stop func() bool)
	now      func() time.Time

	mu      sync.Mutex
	hidden  bool
	holds   int
	pending func() bool
	// sessionEnds is the zero time when no autofill session runs.
	sessionEnds time.Time
}

// New returns a Lock hidden until Shown: Android may start the process for a system screen with no app window.
func New(settings func() preferences.AutoLock, lock func()) *Lock {
	return &Lock{settings: settings, lock: lock, hidden: true, now: time.Now, after: func(delay time.Duration, run func()) func() bool {
		return time.AfterFunc(delay, run).Stop
	}}
}

// Hidden starts the countdown unless automatic locking is off or a hold is in place.
func (l *Lock) Hidden() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hidden = true
	l.start(0)
}

// Shown stops the countdown and ends any autofill session.
func (l *Lock) Shown() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hidden = false
	l.sessionEnds = time.Time{}
	l.stop()
}

// Hold keeps the vault open while the app is behind a system screen it opened, such as the file picker.
func (l *Lock) Hold() (release func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.holds++
	l.stop()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.holds--
			l.start(releaseGrace)
		})
	}
}

// AutofillUnlocked starts an autofill session for an unlock made through autofill.
func (l *Lock) AutofillUnlocked() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.extendSession()
}

// AutofillRequested moves the end of a running autofill session to autofillSession from now.
func (l *Lock) AutofillRequested() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.now().Before(l.sessionEnds) {
		l.extendSession()
	}
}

func (l *Lock) extendSession() {
	l.sessionEnds = l.now().Add(autofillSession)
	l.start(0)
}

func (l *Lock) start(least time.Duration) {
	l.stop()
	if !l.hidden || l.holds > 0 {
		return
	}
	if setting := l.settings(); setting.Enabled {
		session := max(l.sessionEnds.Sub(l.now()), 0)
		l.pending = l.after(max(setting.After, least, session), l.lock)
	}
}

func (l *Lock) stop() {
	if l.pending != nil {
		l.pending()
		l.pending = nil
	}
}
