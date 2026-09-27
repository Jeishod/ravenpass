// Package autolock locks the vault after the chosen delay without keyboard, mouse or trackpad input.
package autolock

import (
	"context"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/spiretechnology/go-idletime"
)

// checkInterval bounds how late a lock can come after the chosen delay.
const checkInterval = 10 * time.Second

// Watcher locks once per idle period; input starts a new period.
type Watcher struct {
	idle     func() (time.Duration, error)
	settings func() preferences.AutoLock
	lock     func()
	fired    bool
}

// New watches the system idle time and reads settings on every check.
func New(settings func() preferences.AutoLock, lock func()) *Watcher {
	return &Watcher{idle: idletime.GetIdleTime, settings: settings, lock: lock}
}

// Run checks the idle time until ctx ends.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check()
		}
	}
}

func (w *Watcher) check() {
	idle, err := w.idle()
	if err != nil {
		return
	}
	setting := w.settings()
	if !setting.Enabled || idle < setting.After {
		w.fired = false
		return
	}
	if w.fired {
		return
	}
	w.fired = true
	w.lock()
}
