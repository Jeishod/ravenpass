//go:build darwin && cgo

package confirmationpanel

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
#include <stdbool.h>

// Global display points: origin at the primary display's top-left, y down, as Wails places screens.
static bool ravenpass_pointer(double *x, double *y) {
	// CGEventCreate is safe off the main thread.
	CGEventRef event = CGEventCreate(NULL);
	if (event == NULL) {
		return false;
	}
	CGPoint location = CGEventGetLocation(event);
	CFRelease(event);
	*x = location.x;
	*y = location.y;
	return true;
}
*/
import "C"

import (
	"math"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func pointer() (application.Point, bool) {
	var x, y C.double
	if !C.ravenpass_pointer(&x, &y) {
		return application.Point{}, false
	}
	return application.Point{X: int(math.Floor(float64(x))), Y: int(math.Floor(float64(y)))}, true
}
