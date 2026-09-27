//go:build darwin && cgo

package pagezoom

/*
#cgo LDFLAGS: -framework AppKit -framework WebKit
#include "pagezoom_darwin.h"
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

func zoomOf(window application.Window) float64 {
	return float64(C.ravenpass_page_zoom(window.NativeWindow()))
}

func setZoom(window application.Window, level float64) {
	C.ravenpass_set_page_zoom(window.NativeWindow(), C.double(level))
}
