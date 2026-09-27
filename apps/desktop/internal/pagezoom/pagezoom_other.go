//go:build !darwin || !cgo

package pagezoom

import "github.com/wailsapp/wails/v3/pkg/application"

func zoomOf(window application.Window) float64 { return window.GetZoom() }

func setZoom(window application.Window, level float64) { window.SetZoom(level) }
