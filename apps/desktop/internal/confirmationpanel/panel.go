// Package confirmationpanel shows the confirmation queue's requests in a floating window kept between requests.
package confirmationpanel

import (
	"errors"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// width is the panel's width in points; the window is the card.
	width = 360
	// cornerRadius is the card's corner radius in points, the page's rounded-xl.
	cornerRadius = 16
	// initialHeight holds the window until the page reports its card's height.
	initialHeight = 240
	minHeight     = 80
	maxHeight     = 600
)

// Window is the panel's native window.
type Window interface {
	SetSize(width, height int)
	// CenterOn centres the window in screen's work area.
	CenterOn(screen *application.Screen)
	// Show orders the window front without making Ravenpass the active app.
	Show()
	// Focus makes the window key without making Ravenpass the active app.
	Focus()
	Hide()
}

// Display builds the panel's window and finds where to show it.
type Display interface {
	Open() Window
	// ScreenUnderPointer returns the screen that holds the pointer, or nil when it is not known.
	ScreenUnderPointer() *application.Screen
}

// Panel shows its window while a request waits and hides it once none does.
type Panel struct {
	display Display

	mu     sync.Mutex
	window Window
	height int
	screen *application.Screen
}

// New composes a Panel whose window display builds on first use.
func New(display Display) (*Panel, error) {
	if display == nil {
		return nil, errors.New("a display is required")
	}
	return &Panel{display: display, height: initialHeight}, nil
}

// Follow shows the panel on the screen under the pointer while a request waits and hides it otherwise.
func (p *Panel) Follow(waiting bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !waiting {
		if p.window != nil {
			p.window.Hide()
		}
		return
	}
	if p.window == nil {
		p.window = p.display.Open()
	}
	p.screen = p.display.ScreenUnderPointer()
	p.window.SetSize(width, p.height)
	p.window.CenterOn(p.screen)
	p.window.Show()
	p.window.Focus()
}

// Fit sets the panel's height to its page's height in points, clamped, and keeps it centred where it was shown.
func (p *Panel) Fit(height int) {
	height = min(max(height, minHeight), maxHeight)
	p.mu.Lock()
	defer p.mu.Unlock()
	if height == p.height {
		return
	}
	p.height = height
	if p.window == nil {
		return
	}
	p.window.SetSize(width, height)
	p.window.CenterOn(p.screen)
}
