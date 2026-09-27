// Package captures holds sign-ins waiting to be saved, in memory only, until saved, discarded or expired.
package captures

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"slices"
	"sync"
	"time"
)

// Lifetime is how long a capture is held from the moment it arrives.
const Lifetime = 3 * time.Minute

const (
	// maxHeld bounds the captures held for one sender; a newer one pushes out the oldest.
	maxHeld = 4
	// idBytes is the length of a held capture's random identifier: 128 bits.
	idBytes = 16
)

var (
	// ErrNotHeld reports a capture that is gone, is being saved, or was not offered the target.
	ErrNotHeld = errors.New("capture is not held")
	// ErrClosed reports a holder that keeps no more captures.
	ErrClosed = errors.New("captures are no longer held")
)

// entry is a capture kept for one sender; saving is set while a save of it runs.
type entry[T any] struct {
	id      string
	capture T
	offered []string
	saving  bool
	expiry  *time.Timer
}

// Held keeps each sender's captures of type T in memory, oldest first; a sender reaches only its own, and forgetting one cannot wipe its strings.
type Held[T any] struct {
	mu       sync.Mutex
	lifetime time.Duration
	held     map[string][]*entry[T]
	closed   bool
}

// New holds captures for lifetime each, at most four for one sender.
func New[T any](lifetime time.Duration) *Held[T] {
	return &Held[T]{lifetime: lifetime, held: make(map[string][]*entry[T])}
}

// Keep holds capture for sender under a new random identifier, pushing out the oldest beyond four.
func (h *Held[T]) Keep(sender string, capture T, offered []string) (string, error) {
	random := make([]byte, idBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(random)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return "", ErrClosed
	}
	held := h.held[sender]
	for len(held) >= maxHeld {
		held[0].expiry.Stop()
		held = held[1:]
	}
	kept := &entry[T]{id: id, capture: capture, offered: offered}
	kept.expiry = time.AfterFunc(h.lifetime, func() { h.Forget(sender, id) })
	h.held[sender] = append(slices.Clone(held), kept)
	return id, nil
}

// Find returns the capture sender holds under id.
func (h *Held[T]) Find(sender, id string) (T, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := h.lookup(sender, id)
	if kept == nil {
		var none T
		return none, false
	}
	return kept.capture, true
}

// Count is how many captures sender holds.
func (h *Held[T]) Count(sender string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.held[sender])
}

// Offer records the targets the capture id was offered in its latest answer.
func (h *Held[T]) Offer(sender, id string, offered []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if kept := h.lookup(sender, id); kept != nil {
		kept.offered = offered
	}
}

// Claim marks the capture saving to target until Release; an empty target, a new credential, is always offered.
func (h *Held[T]) Claim(sender, id, target string) (T, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := h.lookup(sender, id)
	if kept == nil || kept.saving || target != "" && !slices.Contains(kept.offered, target) {
		var none T
		return none, ErrNotHeld
	}
	kept.saving = true
	return kept.capture, nil
}

// Release ends a claim, forgetting the capture when saved.
func (h *Held[T]) Release(sender, id string, saved bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if saved {
		h.remove(sender, id)
	} else if kept := h.lookup(sender, id); kept != nil {
		kept.saving = false
	}
}

// Forget drops the capture sender holds under id, if it holds one.
func (h *Held[T]) Forget(sender, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.remove(sender, id)
}

// ForgetSender drops every capture sender holds.
func (h *Held[T]) ForgetSender(sender string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, kept := range h.held[sender] {
		kept.expiry.Stop()
	}
	delete(h.held, sender)
}

// Close drops every capture and keeps none from then on.
func (h *Held[T]) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, held := range h.held {
		for _, kept := range held {
			kept.expiry.Stop()
		}
	}
	clear(h.held)
	h.closed = true
}

// lookup is the capture sender holds under id, nil for none. The caller holds h.mu.
func (h *Held[T]) lookup(sender, id string) *entry[T] {
	index := slices.IndexFunc(h.held[sender], func(kept *entry[T]) bool { return kept.id == id })
	if index < 0 {
		return nil
	}
	return h.held[sender][index]
}

// remove drops the capture sender holds under id. The caller holds h.mu.
func (h *Held[T]) remove(sender, id string) {
	held := h.held[sender]
	index := slices.IndexFunc(held, func(kept *entry[T]) bool { return kept.id == id })
	if index < 0 {
		return
	}
	held[index].expiry.Stop()
	if len(held) == 1 {
		delete(h.held, sender)
		return
	}
	h.held[sender] = slices.Delete(slices.Clone(held), index, index+1)
}
