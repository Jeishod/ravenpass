package api

import (
	"errors"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// ClipboardClearing is the clipboard clearing setting and the delays offered.
type ClipboardClearing struct {
	Enabled bool  `json:"enabled"`
	Seconds int   `json:"seconds"`
	Offered []int `json:"offered"`
}

// GetClipboardClearing reports the clipboard clearing setting.
func (s *Service) GetClipboardClearing() (ClipboardClearing, error) {
	clearing := s.preferences.ClipboardClearing()
	delays := preferences.ClearDelays()
	offered := make([]int, len(delays))
	for i, delay := range delays {
		offered[i] = int(delay / time.Second)
	}
	return ClipboardClearing{Enabled: clearing.Enabled, Seconds: int(clearing.After / time.Second), Offered: offered}, nil
}

// SetClipboardClearing records the setting for later copies; a copy already made keeps its delay.
func (s *Service) SetClipboardClearing(enabled bool, seconds int) error {
	clearing := preferences.ClipboardClearing{Enabled: enabled, After: time.Duration(seconds) * time.Second}
	if err := s.preferences.SetClipboardClearing(clearing); err != nil {
		if errors.Is(err, preferences.ErrUnsupportedClearDelay) {
			return fail(failureClearDelayUnsupported)
		}
		return present(err)
	}
	return nil
}

// Pasteboard receives every copy; a write replaces its content, hidden from history, and returns the change count.
type Pasteboard interface {
	WriteText(text string) (int64, error)
	Write(content []byte, mediaType string) (int64, error)
	ChangeCount() int64
	Clear()
}

// heldCopy is the copy this app last placed, present while the change count is unchanged.
type heldCopy struct {
	pasteboard  Pasteboard
	changeCount int64
}

func (c heldCopy) present() bool { return c.pasteboard.ChangeCount() == c.changeCount }

// clipboardState tracks this app's copy so a clear removes only that copy; text and scans share one generation.
type clipboardState struct {
	mu         sync.Mutex
	generation uint64
	held       *heldCopy
}

// removal calls discard once a held copy is due to leave the clipboard.
type removal func(discard func())

// afterClearingDelay removes a copy after the user's clearing delay, never while clearing is off.
func (s *Service) afterClearingDelay(schedule func(time.Duration, func())) removal {
	return func(discard func()) {
		clearing := s.preferences.ClipboardClearing()
		if clearing.Enabled {
			schedule(clearing.After, discard)
		}
	}
}

// whenClosed removes a copy once ended closes, whatever the clearing setting.
func whenClosed(ended <-chan struct{}, watch func(<-chan struct{}, func())) removal {
	return func(discard func()) { watch(ended, discard) }
}

func clearWhenClosed(ended <-chan struct{}, discard func()) {
	go func() {
		<-ended
		discard()
	}()
}

func (s *Service) copyToClipboard(value string, schedule func(time.Duration, func())) bool {
	return s.copyText(value, s.afterClearingDelay(schedule))
}

func (s *Service) copyText(value string, remove removal) bool {
	return s.place(func() (int64, error) { return s.pasteboard.WriteText(value) }, remove) == nil
}

func (s *Service) copyScanToPasteboard(content []byte, mediaType string, schedule func(time.Duration, func())) error {
	return s.place(func() (int64, error) { return s.pasteboard.Write(content, mediaType) }, s.afterClearingDelay(schedule))
}

// place runs write under the clipboard lock and hands remove a discard that a later copy or clear makes a no-op.
func (s *Service) place(write func() (int64, error), remove removal) error {
	s.clipboard.mu.Lock()
	defer s.clipboard.mu.Unlock()
	count, err := write()
	if err != nil {
		return err
	}
	s.clipboard.generation++
	s.clipboard.held = &heldCopy{pasteboard: s.pasteboard, changeCount: count}
	generation := s.clipboard.generation
	remove(func() {
		s.clipboard.mu.Lock()
		defer s.clipboard.mu.Unlock()
		s.clipboard.discard(generation)
	})
	return nil
}

func (s *Service) clearClipboard() {
	s.clipboard.mu.Lock()
	defer s.clipboard.mu.Unlock()
	s.clipboard.discard(s.clipboard.generation)
	s.clipboard.generation++
}

func (c *clipboardState) discard(generation uint64) {
	if c.held == nil || c.generation != generation {
		return
	}
	if c.held.present() {
		c.held.pasteboard.Clear()
	}
	c.held = nil
}
