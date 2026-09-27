package background

import (
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

type manualClock struct {
	time    time.Time
	delay   time.Duration
	run     func()
	stopped bool
	started int
}

func (c *manualClock) now() time.Time { return c.time }

func (c *manualClock) advance(by time.Duration) { c.time = c.time.Add(by) }

func (c *manualClock) after(delay time.Duration, run func()) func() bool {
	c.delay, c.run, c.stopped = delay, run, false
	c.started++
	return func() bool {
		c.stopped = true
		return true
	}
}

func (c *manualClock) fire() {
	if !c.stopped && c.run != nil {
		c.run()
	}
}

func newLock(setting preferences.AutoLock, locks *int) (*Lock, *manualClock) {
	clock := &manualClock{time: time.Date(2026, time.September, 25, 9, 0, 0, 0, time.UTC)}
	lock := New(func() preferences.AutoLock { return setting }, func() { *locks++ })
	lock.after = clock.after
	lock.now = clock.now
	return lock, clock
}

// signInThroughAutofill is an unlock on an autofill screen of an app whose own window is hidden.
func signInThroughAutofill(lock *Lock) {
	release := lock.Hold()
	lock.AutofillUnlocked()
	release()
}

func TestAHiddenAppLocksAfterTheDelay(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 5 * time.Minute}, &locks)
	lock.Hidden()
	if clock.delay != 5*time.Minute {
		t.Fatalf("the countdown lasts %v", clock.delay)
	}
	clock.fire()
	if locks != 1 {
		t.Fatalf("locked %d times", locks)
	}
}

func TestAnAppShownAgainStaysOpen(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: time.Minute}, &locks)
	lock.Hidden()
	lock.Shown()
	clock.fire()
	if locks != 0 {
		t.Fatalf("an app shown before the delay locked %d times", locks)
	}
}

func TestAHoldKeepsTheVaultOpenWhileTheAppIsHidden(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: time.Minute}, &locks)
	release := lock.Hold()
	lock.Hidden()
	if clock.run != nil {
		t.Fatal("a countdown started during a hold")
	}
	lock.Shown()
	release()
	if clock.run != nil {
		t.Fatal("a countdown started for an app shown again")
	}
}

func TestReleasingAHoldWhileHiddenStartsTheCountdown(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: time.Minute}, &locks)
	lock.Hidden()
	release := lock.Hold()
	clock.fire()
	if locks != 0 {
		t.Fatal("a hold did not stop a running countdown")
	}
	release()
	release()
	clock.fire()
	if locks != 1 {
		t.Fatalf("an app still hidden after a hold locked %d times", locks)
	}
}

func TestAProcessStartedWithoutTheWindowCountsDownOnceItsLastHoldEnds(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: time.Minute}, &locks)
	release := lock.Hold()
	if clock.run != nil {
		t.Fatal("a countdown started during a hold")
	}
	release()
	clock.fire()
	if locks != 1 {
		t.Fatalf("a vault opened with no window shown locked %d times", locks)
	}
}

func TestAnAppSetToLockAtOnceLocksAsItHides(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	lock.Shown()
	lock.Hidden()
	if clock.run == nil || clock.delay != 0 {
		t.Fatalf("hiding started a countdown of %v, want none", clock.delay)
	}
	clock.fire()
	if locks != 1 {
		t.Fatalf("an app set to lock at once locked %d times", locks)
	}
}

func TestAReleasedHoldWaitsForTheReturningAppToReportItself(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	lock.Shown()
	release := lock.Hold()
	lock.Hidden()
	release()
	if clock.delay != releaseGrace {
		t.Fatalf("a released hold counts down %v, want %v", clock.delay, releaseGrace)
	}
	lock.Shown()
	clock.fire()
	if locks != 0 {
		t.Fatalf("an app back from its picker locked %d times", locks)
	}
}

func TestAnAppStillHiddenAfterAHoldLocksOnceTheGracePasses(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	release := lock.Hold()
	release()
	if clock.delay != releaseGrace {
		t.Fatalf("a released hold counts down %v, want %v", clock.delay, releaseGrace)
	}
	clock.fire()
	if locks != 1 {
		t.Fatalf("an app left hidden after an autofill screen locked %d times", locks)
	}
}

func TestAReleasedHoldKeepsALongerDelay(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 5 * time.Minute}, &locks)
	release := lock.Hold()
	release()
	if clock.delay != 5*time.Minute {
		t.Fatalf("a released hold counts down %v, want the owner's delay", clock.delay)
	}
}

func TestLockingAtOnceRunsWithoutTheLockHeld(t *testing.T) {
	locked := make(chan struct{})
	var lock *Lock
	lock = New(func() preferences.AutoLock { return preferences.AutoLock{Enabled: true, After: 0} }, func() {
		lock.Shown()
		close(locked)
	})
	lock.Shown()
	lock.Hidden()
	select {
	case <-locked:
	case <-time.After(5 * time.Second):
		t.Fatal("hiding an app set to lock at once did not lock it")
	}
}

func TestAutomaticLockingOffNeverCountsDown(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: false, After: time.Minute}, &locks)
	lock.Hidden()
	if clock.run != nil {
		t.Fatal("a countdown started with automatic locking off")
	}
	signInThroughAutofill(lock)
	if clock.run != nil {
		t.Fatal("an autofill session started a countdown with automatic locking off")
	}
}

func TestAnAutofillUnlockKeepsTheVaultOpenForTheSession(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	signInThroughAutofill(lock)
	if clock.delay != autofillSession {
		t.Fatalf("an autofill unlock counts down %v, want %v", clock.delay, autofillSession)
	}
	clock.advance(autofillSession)
	clock.fire()
	if locks != 1 {
		t.Fatalf("an autofill session with no further request locked %d times", locks)
	}
}

func TestEachAutofillRequestMovesTheSessionsEnd(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	signInThroughAutofill(lock)
	clock.advance(2 * time.Minute)
	lock.AutofillRequested()
	if clock.delay != autofillSession || clock.stopped {
		t.Fatalf("a request two minutes in counts down %v, stopped %v, want %v", clock.delay, clock.stopped, autofillSession)
	}
	clock.advance(2 * time.Minute)
	release := lock.Hold()
	lock.AutofillRequested()
	release()
	if clock.delay != autofillSession {
		t.Fatalf("a screen closing after a request counts down %v, want %v", clock.delay, autofillSession)
	}
	if locks != 0 {
		t.Fatalf("a sign-in still asking autofill locked %d times", locks)
	}
}

func TestAnOwnersDelayLongerThanTheSessionKeepsTheVaultOpenLonger(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 5 * time.Minute}, &locks)
	signInThroughAutofill(lock)
	if clock.delay != 5*time.Minute {
		t.Fatalf("an autofill unlock counts down %v, want the owner's delay", clock.delay)
	}
	clock.advance(time.Minute)
	lock.AutofillRequested()
	if clock.delay != 5*time.Minute {
		t.Fatalf("a request counts down %v, want the owner's delay from it", clock.delay)
	}
}

func TestAutofillReachingAVaultOpenedInTheAppKeepsTheOwnersDelay(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: time.Minute}, &locks)
	lock.Shown()
	lock.Hidden()
	clock.advance(30 * time.Second)
	started := clock.started
	lock.AutofillRequested()
	if clock.started != started || clock.stopped {
		t.Fatal("a request with no autofill unlock restarted the owner's countdown")
	}
	clock.fire()
	if locks != 1 {
		t.Fatalf("the owner's delay locked %d times", locks)
	}
}

func TestAnEndedSessionIsNotExtended(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	signInThroughAutofill(lock)
	clock.advance(autofillSession)
	clock.fire()
	started := clock.started
	lock.AutofillRequested()
	if clock.started != started {
		t.Fatal("a request after the session ended started a countdown")
	}
	release := lock.Hold()
	release()
	if clock.delay != releaseGrace {
		t.Fatalf("a screen closing after the session ended counts down %v, want %v", clock.delay, releaseGrace)
	}
}

func TestShowingTheAppEndsTheSession(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	signInThroughAutofill(lock)
	lock.Shown()
	lock.Hidden()
	if clock.delay != 0 {
		t.Fatalf("leaving the app after a session counts down %v, want the owner's delay", clock.delay)
	}
	lock.AutofillRequested()
	if clock.delay != 0 {
		t.Fatalf("a request after the app was shown counts down %v, want the owner's delay", clock.delay)
	}
}

func TestAnAutofillUnlockWhileTheAppIsInFrontCountsDownOnceItLeaves(t *testing.T) {
	var locks int
	lock, clock := newLock(preferences.AutoLock{Enabled: true, After: 0}, &locks)
	lock.Shown()
	lock.AutofillUnlocked()
	if clock.run != nil {
		t.Fatal("a countdown started while the app is in front")
	}
	lock.Hidden()
	if clock.delay != autofillSession {
		t.Fatalf("leaving the app during a session counts down %v, want %v", clock.delay, autofillSession)
	}
}
