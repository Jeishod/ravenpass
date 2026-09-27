//go:build darwin && cgo

package trash

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "trash_darwin.h"
*/
import "C"

import "unsafe"

func moveToTrash(path string) error {
	location := C.CString(path)
	defer C.free(unsafe.Pointer(location))
	if C.ravenpass_trash_move(location) == 0 {
		return ErrRefused
	}
	return nil
}
