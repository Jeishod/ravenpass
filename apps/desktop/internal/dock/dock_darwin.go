//go:build darwin && cgo

package dock

/*
#cgo LDFLAGS: -framework AppKit
#include "dock_darwin.h"
*/
import "C"

func show() { C.ravenpass_dock_show() }

func hide() { C.ravenpass_dock_hide() }
