//go:build darwin && cgo

package appearance

/*
#cgo LDFLAGS: -framework AppKit
#include "appearance_darwin.h"
*/
import "C"

import "github.com/dortanes/ravenpass/packages/app/preferences"

func apply(chosen preferences.Appearance) {
	switch chosen {
	case preferences.AppearanceLight:
		C.ravenpass_appearance_set(C.ravenpass_appearance_light)
	case preferences.AppearanceDark:
		C.ravenpass_appearance_set(C.ravenpass_appearance_dark)
	default:
		C.ravenpass_appearance_set(C.ravenpass_appearance_system)
	}
}
