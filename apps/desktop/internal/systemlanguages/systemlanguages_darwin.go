//go:build darwin && cgo

package systemlanguages

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "systemlanguages_darwin.h"
*/
import "C"

import (
	"strings"
	"unsafe"
)

func preferred() []string {
	tags := C.ravenpass_preferred_languages()
	if tags == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(tags))
	return strings.Split(C.GoString(tags), "\n")
}
