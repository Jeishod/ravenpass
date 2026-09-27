// Package mainwindow keeps Ravenpass's one window, rebuilt on show where it was and at its zoom.
package mainwindow

import (
	"sync"

	"github.com/dortanes/ravenpass/apps/desktop/internal/pagezoom"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Window is the main window; Hide destroys its web view with everything the page showed.
type Window struct {
	app     *application.App
	options application.WebviewWindowOptions
	closed  func()

	mu      sync.Mutex
	current *application.WebviewWindow
	bounds  application.Rect
	zoom    float64
}

// New prepares the window without building it; closed runs after every close, including Hide's.
func New(app *application.App, options application.WebviewWindowOptions, closed func()) *Window {
	options.Hidden = true
	return &Window{app: app, options: options, closed: closed, zoom: 1}
}

// Show brings the window forward, building it first when it is closed.
func (w *Window) Show() {
	window := w.open()
	window.Show()
	window.Focus()
}

// Hide closes the window and releases its web view.
func (w *Window) Hide() {
	if window := w.opened(); window != nil && w.detach(window) {
		window.Close()
	}
}

// InFront reports whether the window is open, visible and focused.
func (w *Window) InFront() bool {
	window := w.opened()
	return window != nil && window.IsVisible() && window.IsFocused()
}

// Reload loads the page again, if the window is open.
func (w *Window) Reload() {
	if window := w.opened(); window != nil {
		window.Reload()
	}
}

func (w *Window) opened() *application.WebviewWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.current
}

func (w *Window) open() *application.WebviewWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		return w.current
	}
	window := w.app.Window.NewWithOptions(w.options)
	window.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
		w.detach(window)
		w.closed()
	})
	if w.bounds != (application.Rect{}) {
		window.SetBounds(w.bounds)
	}
	pagezoom.Set(window, w.zoom)
	w.current = window
	return window
}

// detach keeps window's place and zoom for the next build and reports false when it was already detached.
func (w *Window) detach(window *application.WebviewWindow) bool {
	bounds, zoom := window.Bounds(), pagezoom.Of(window)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != window {
		return false
	}
	w.current, w.bounds, w.zoom = nil, bounds, zoom
	return true
}
