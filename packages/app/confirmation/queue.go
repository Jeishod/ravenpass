// Package confirmation queues PIN and unlock requests that wait for the person's answer, oldest first.
package confirmation

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"

	"github.com/dortanes/ravenpass/packages/app/unlock"
)

var (
	// ErrEnded answers a request that no longer waits.
	ErrEnded = errors.New("the confirmation request has ended")
	// ErrDeclined ends a verify request the person refused, or whose PIN wrong attempts removed.
	ErrDeclined = errors.New("the confirmation request was declined")
)

// Kind is what a request asks of the person.
type Kind string

const (
	// KindVerify asks for the vault's PIN before what a Reason names.
	KindVerify Kind = "verify"
	// KindUnlock asks to unlock the vault, for a Requester.
	KindUnlock Kind = "unlock"
)

// Requester is what asks to unlock the vault.
type Requester string

const (
	// RequesterExtension is a linked browser extension.
	RequesterExtension Requester = "extension"
	// RequesterAutofill is the system AutoFill.
	RequesterAutofill Requester = "autofill"
)

// Request is one request waiting for the person. Reason is set for KindVerify only, Requester for KindUnlock only.
type Request struct {
	ID        string
	Kind      Kind
	Reason    Reason
	Requester Requester
}

// PINChecker checks the open vault's PIN, counting a wrong one as unlocking does.
type PINChecker interface {
	VerifyPIN(pin string) error
}

// Queue holds the waiting requests, oldest first.
type Queue struct {
	pins PINChecker

	mu      sync.Mutex
	waiting []*request
	// changed is closed and replaced whenever a request joins or leaves the queue.
	changed chan struct{}
	counter uint64
	// onDevice is what a new unlock request tries before it shows; nil shows it at once.
	onDevice func() bool

	// watchMu keeps watchers hearing one change at a time, in order.
	watchMu   sync.Mutex
	watchers  []func(waiting bool)
	announced bool
}

// request is one waiting request; answer receives its outcome once, as it leaves the queue.
type request struct {
	Request
	answer chan error
	// held keeps an unlock request from Next and the watchers while onDevice runs.
	held bool
}

// New composes a Queue whose verify requests are answered with pins.
func New(pins PINChecker) (*Queue, error) {
	if pins == nil {
		return nil, errors.New("PIN checks are required")
	}
	return &Queue{pins: pins, changed: make(chan struct{})}, nil
}

// Ask queues a verify request for reason and returns its outcome; ctx ending withdraws the request.
func (q *Queue) Ask(ctx context.Context, reason Reason) error {
	q.mu.Lock()
	asked := q.enqueue(Request{Kind: KindVerify, Reason: reason})
	q.mu.Unlock()
	q.announce()
	select {
	case err := <-asked.answer:
		return err
	case <-ctx.Done():
		// An answer given just before ctx ended stands.
		q.settle(asked, ctx.Err())
		return <-asked.answer
	}
}

// UnlockOnDevice makes each new unlock request try first, held from view, and show only once try returns false; a try
// that opens the vault ends the request through VaultOpened.
func (q *Queue) UnlockOnDevice(try func() bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onDevice = try
}

// PostUnlock queues an unlock request for requester, or joins the one already waiting with its requester, and returns its ID.
func (q *Queue) PostUnlock(requester Requester) string {
	q.mu.Lock()
	if index := slices.IndexFunc(q.waiting, func(waiting *request) bool { return waiting.Kind == KindUnlock }); index >= 0 {
		id := q.waiting[index].ID
		q.mu.Unlock()
		return id
	}
	posted := q.enqueue(Request{Kind: KindUnlock, Requester: requester})
	try := q.onDevice
	posted.held = try != nil
	q.mu.Unlock()
	if try != nil {
		go q.showUnless(posted, try)
	}
	q.announce()
	return posted.ID
}

// showUnless shows held once try fails, unless it ended meanwhile.
func (q *Queue) showUnless(held *request, try func() bool) {
	if try() {
		return
	}
	q.mu.Lock()
	waits := slices.Contains(q.waiting, held)
	if waits {
		held.held = false
		q.signal()
	}
	q.mu.Unlock()
	if waits {
		q.announce()
	}
}

// enqueue adds asked to the queue under a new ID. The caller holds q.mu.
func (q *Queue) enqueue(asked Request) *request {
	q.counter++
	asked.ID = strconv.FormatUint(q.counter, 10)
	added := &request{Request: asked, answer: make(chan error, 1)}
	q.waiting = append(q.waiting, added)
	q.signal()
	return added
}

// signal wakes every Next. The caller holds q.mu.
func (q *Queue) signal() {
	close(q.changed)
	q.changed = make(chan struct{})
}

// Next returns the oldest shown request once its ID differs from shown; an empty ID means none is shown.
func (q *Queue) Next(ctx context.Context, shown string) (Request, error) {
	for {
		q.mu.Lock()
		var oldest Request
		if index := slices.IndexFunc(q.waiting, isShown); index >= 0 {
			oldest = q.waiting[index].Request
		}
		changed := q.changed
		q.mu.Unlock()
		if oldest.ID != shown {
			return oldest, nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return Request{}, ctx.Err()
		}
	}
}

// AwaitEnd returns once the request id no longer waits, or with ctx's error once ctx ends.
func (q *Queue) AwaitEnd(ctx context.Context, id string) error {
	for {
		q.mu.Lock()
		waits := slices.ContainsFunc(q.waiting, func(waiting *request) bool { return waiting.ID == id })
		changed := q.changed
		q.mu.Unlock()
		if !waits {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Waiting returns the request id while it waits, and fails with ErrEnded once it does not.
func (q *Queue) Waiting(id string) (Request, error) {
	found, err := q.find(id)
	if err != nil {
		return Request{}, err
	}
	return found.Request, nil
}

// ConfirmPIN checks pin for the verify request id; the attempt that removes the PIN declines it.
func (q *Queue) ConfirmPIN(id, pin string) error {
	answered, err := q.find(id)
	if err != nil {
		return err
	}
	if answered.Kind != KindVerify {
		return ErrEnded
	}
	switch err := q.pins.VerifyPIN(pin); {
	case err == nil:
		if !q.settle(answered, nil) {
			return ErrEnded
		}
		return nil
	case errors.Is(err, unlock.ErrPINRemoved):
		q.settle(answered, ErrDeclined)
		return err
	default:
		return err
	}
}

// Decline ends the request id as declined. A request that no longer waits fails with ErrEnded.
func (q *Queue) Decline(id string) error {
	declined, err := q.find(id)
	if err != nil {
		return err
	}
	if !q.settle(declined, ErrDeclined) {
		return ErrEnded
	}
	return nil
}

// VaultOpened ends the waiting unlock request, however the vault opened.
func (q *Queue) VaultOpened() {
	q.mu.Lock()
	var opened []*request
	q.waiting = slices.DeleteFunc(q.waiting, func(waiting *request) bool {
		if waiting.Kind != KindUnlock {
			return false
		}
		opened = append(opened, waiting)
		return true
	})
	if len(opened) > 0 {
		q.signal()
	}
	q.mu.Unlock()
	for _, answered := range opened {
		answered.answer <- nil
	}
	q.announce()
}

// EndAll ends every waiting request with cause.
func (q *Queue) EndAll(cause error) {
	q.mu.Lock()
	ended := q.waiting
	q.waiting = nil
	if len(ended) > 0 {
		q.signal()
	}
	q.mu.Unlock()
	for _, waiting := range ended {
		waiting.answer <- cause
	}
	q.announce()
}

// Watch calls watcher with whether a request is shown, at once and after each change, never under q.mu.
func (q *Queue) Watch(watcher func(waiting bool)) {
	q.watchMu.Lock()
	defer q.watchMu.Unlock()
	q.watchers = append(q.watchers, watcher)
	watcher(q.announced)
}

// announce tells the watchers whether a request is shown, when that changed since they last heard.
func (q *Queue) announce() {
	q.watchMu.Lock()
	defer q.watchMu.Unlock()
	q.mu.Lock()
	waiting := slices.ContainsFunc(q.waiting, isShown)
	q.mu.Unlock()
	if waiting == q.announced {
		return
	}
	q.announced = waiting
	for _, watcher := range q.watchers {
		watcher(waiting)
	}
}

// isShown reports whether waiting is shown. The caller holds q.mu.
func isShown(waiting *request) bool { return !waiting.held }

func (q *Queue) find(id string) (*request, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	index := slices.IndexFunc(q.waiting, func(waiting *request) bool { return waiting.ID == id })
	if index < 0 {
		return nil, ErrEnded
	}
	return q.waiting[index], nil
}

// settle takes answered out of the queue with its outcome, and reports whether it still waited.
func (q *Queue) settle(answered *request, outcome error) bool {
	q.mu.Lock()
	index := slices.Index(q.waiting, answered)
	if index < 0 {
		q.mu.Unlock()
		return false
	}
	q.waiting = slices.Delete(q.waiting, index, index+1)
	q.signal()
	answered.answer <- outcome
	q.mu.Unlock()
	q.announce()
	return true
}
