package api

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"time"
)

// memoryPasteboard holds one copy and counts changes as the system pasteboard does.
type memoryPasteboard struct {
	text      string
	content   []byte
	mediaType string
	count     int64
	failing   bool
}

func (m *memoryPasteboard) WriteText(text string) (int64, error) {
	if m.failing {
		return 0, errors.New("pasteboard refused the write")
	}
	m.text, m.content, m.mediaType = text, nil, ""
	m.count++
	return m.count, nil
}

func (m *memoryPasteboard) Write(content []byte, mediaType string) (int64, error) {
	if m.failing {
		return 0, errors.New("pasteboard refused the write")
	}
	m.text, m.content, m.mediaType = "", bytes.Clone(content), mediaType
	m.count++
	return m.count, nil
}

func (m *memoryPasteboard) ChangeCount() int64 { return m.count }

func (m *memoryPasteboard) Clear() {
	m.text, m.content, m.mediaType = "", nil, ""
	m.count++
}

// replace stands for another app writing text to the pasteboard.
func (m *memoryPasteboard) replace(text string) {
	m.text, m.content, m.mediaType = text, nil, ""
	m.count++
}

// attachPasteboard gives service a fresh memory pasteboard and returns it.
func attachPasteboard(service *Service) *memoryPasteboard {
	board := &memoryPasteboard{}
	service.pasteboard = board
	return board
}

func TestCopyClearsAfterTheChosenDelay(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	board := attachPasteboard(service)
	if err := service.SetClipboardClearing(true, 15); err != nil {
		t.Fatal(err)
	}
	var discard func()
	var delay time.Duration
	if !service.copyToClipboard("secret", func(after time.Duration, clear func()) {
		delay, discard = after, clear
	}) {
		t.Fatal("copy failed")
	}
	if delay != 15*time.Second {
		t.Fatalf("clear delay = %v, want 15s", delay)
	}
	discard()
	if board.text != "" {
		t.Fatal("the copy stayed on the clipboard after the chosen delay")
	}
}

func TestCopyStaysWhenClearingIsOff(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	board := attachPasteboard(service)
	if err := service.SetClipboardClearing(false, 30); err != nil {
		t.Fatal(err)
	}
	scheduled := false
	if !service.copyToClipboard("secret", func(time.Duration, func()) { scheduled = true }) {
		t.Fatal("copy failed")
	}
	if scheduled {
		t.Fatal("a clear was scheduled while clearing is off")
	}
	if board.text != "secret" {
		t.Fatalf("clipboard = %q", board.text)
	}
	service.clearClipboard()
	if board.text != "" {
		t.Fatal("locking left the copy on the clipboard while clearing is off")
	}
}

func TestClearingKeepsTheSameTextCopiedElsewhere(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	board := attachPasteboard(service)
	var discard func()
	if !service.copyToClipboard("secret", func(_ time.Duration, clear func()) { discard = clear }) {
		t.Fatal("copy failed")
	}
	board.replace("secret")
	discard()
	if board.text != "secret" {
		t.Fatal("the timed clear removed the same text another app wrote afterwards")
	}

	if !service.copyToClipboard("secret", func(time.Duration, func()) {}) {
		t.Fatal("copy failed")
	}
	board.replace("secret")
	service.clearClipboard()
	if board.text != "secret" {
		t.Fatal("locking removed the same text another app wrote afterwards")
	}
}

func TestRefusedTextCopyHoldsNothing(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	board := attachPasteboard(service)
	board.failing = true
	scheduled := false
	if service.copyToClipboard("secret", func(time.Duration, func()) { scheduled = true }) {
		t.Fatal("a refused write reported a copy")
	}
	if scheduled || service.clipboard.held != nil {
		t.Fatal("a refused write left a clear waiting")
	}
	before := board.count
	service.clearClipboard()
	if board.count != before {
		t.Fatal("locking cleared a pasteboard this app never wrote to")
	}
}

func TestClearWhenClosedDiscardsOnceTheChannelCloses(t *testing.T) {
	ended := make(chan struct{})
	discarded := make(chan struct{})
	clearWhenClosed(ended, func() { close(discarded) })
	select {
	case <-discarded:
		t.Fatal("discarded before the channel closed")
	case <-time.After(20 * time.Millisecond):
	}
	close(ended)
	select {
	case <-discarded:
	case <-time.After(hangLimit):
		t.Fatal("not discarded after the channel closed")
	}
}

func TestClipboardClearingSettings(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	current, err := service.GetClipboardClearing()
	if err != nil {
		t.Fatal(err)
	}
	if !current.Enabled || current.Seconds != 60 || !slices.Contains(current.Offered, 60) {
		t.Fatalf("default clearing = %+v", current)
	}
	if err := service.SetClipboardClearing(false, 120); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetClipboardClearing(); current.Enabled || current.Seconds != 120 {
		t.Fatalf("clearing after turning it off = %+v", current)
	}
	for _, seconds := range []int{0, -1, 45, 3600} {
		err := service.SetClipboardClearing(true, seconds)
		if err == nil || err.Error() != failurePrefix+string(failureClearDelayUnsupported) {
			t.Fatalf("delay %d: error = %v", seconds, err)
		}
	}
	if current, _ = service.GetClipboardClearing(); current.Enabled || current.Seconds != 120 {
		t.Fatalf("a refused delay changed the setting to %+v", current)
	}
}
