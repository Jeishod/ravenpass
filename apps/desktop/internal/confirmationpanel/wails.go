package confirmationpanel

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// pageURL is the page the panel shows.
const pageURL = "/confirm.html"

type wailsDisplay struct {
	app func() *application.App
}

// NewDisplay returns the Display of the Wails app that app returns once it runs.
func NewDisplay(app func() *application.App) Display {
	return wailsDisplay{app: app}
}

// Open builds the panel's window, which refuses every close to keep its web view for the next request.
func (d wailsDisplay) Open() Window {
	window := d.app().Window.NewWithOptions(panelOptions())
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
	})
	return webview{window: window}
}

func (d wailsDisplay) ScreenUnderPointer() *application.Screen {
	at, known := pointer()
	return screenAt(d.app().Screen.GetAll(), at, known)
}

type webview struct {
	window *application.WebviewWindow
}

func (w webview) SetSize(width, height int) { w.window.SetSize(width, height) }

func (w webview) CenterOn(screen *application.Screen) {
	if screen == nil {
		w.window.Center()
		return
	}
	w.window.SetScreen(screen)
}

func (w webview) Show() { w.window.Show() }

func (w webview) Focus() { w.window.Focus() }

func (w webview) Hide() { w.window.Hide() }

func baseOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name:             "confirmation",
		URL:              pageURL,
		Width:            width,
		Height:           initialHeight,
		Frameless:        true,
		DisableResize:    true,
		Hidden:           true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	}
}

// screenAt falls back to the primary screen, then the first screen, and is nil only without screens.
func screenAt(screens []*application.Screen, point application.Point, known bool) *application.Screen {
	var primary *application.Screen
	for _, screen := range screens {
		if known && contains(screen.Bounds, point) {
			return screen
		}
		if screen.IsPrimary {
			primary = screen
		}
	}
	if primary == nil && len(screens) > 0 {
		return screens[0]
	}
	return primary
}

func contains(bounds application.Rect, point application.Point) bool {
	return point.X >= bounds.X && point.X < bounds.X+bounds.Width &&
		point.Y >= bounds.Y && point.Y < bounds.Y+bounds.Height
}
