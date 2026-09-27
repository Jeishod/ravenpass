// Package pagezoom zooms with WKWebView pageZoom; Wails' magnification overflows the window and stops at 1.
package pagezoom

import "github.com/wailsapp/wails/v3/pkg/application"

// levels are the zoom steps Safari offers.
var levels = []float64{0.5, 0.75, 0.85, 1, 1.15, 1.25, 1.5, 1.75, 2, 2.5, 3}

// tolerance absorbs the rounding a web view applies to a zoom factor it stores.
const tolerance = 0.001

// nextLevel returns the first level above current, or the largest level.
func nextLevel(current float64) float64 {
	for _, level := range levels {
		if level > current+tolerance {
			return level
		}
	}
	return levels[len(levels)-1]
}

// previousLevel returns the last level below current, or the smallest level.
func previousLevel(current float64) float64 {
	for i := len(levels) - 1; i >= 0; i-- {
		if levels[i] < current-tolerance {
			return levels[i]
		}
	}
	return levels[0]
}

// Of returns the page zoom of window.
func Of(window application.Window) float64 {
	var level float64
	application.InvokeSync(func() { level = zoomOf(window) })
	return level
}

// Set applies a page zoom to window.
func Set(window application.Window, level float64) {
	application.InvokeSync(func() { setZoom(window, level) })
}

// Bind points the zoom items of menu at page zoom for the window current returns.
func Bind(menu *application.Menu, current func() application.Window) {
	actions := map[application.Role]func(float64) float64{
		application.ZoomIn:    nextLevel,
		application.ZoomOut:   previousLevel,
		application.ResetZoom: func(float64) float64 { return 1 },
	}
	for role, step := range actions {
		item := menu.FindByRole(role)
		if item == nil {
			continue
		}
		item.OnClick(func(*application.Context) {
			window := current()
			if window == nil {
				return
			}
			application.InvokeSync(func() { setZoom(window, step(zoomOf(window))) })
		})
	}
}
