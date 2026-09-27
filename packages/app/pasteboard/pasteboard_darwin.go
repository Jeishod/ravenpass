//go:build darwin && cgo

package pasteboard

/*
#cgo LDFLAGS: -framework AppKit -framework Foundation
#include <stdlib.h>
#include "pasteboard_darwin.h"
*/
import "C"

import "unsafe"

func write(content []byte, uti string) (int64, error) {
	if len(content) == 0 {
		return 0, ErrUnavailable
	}
	kind := C.CString(uti)
	defer C.free(unsafe.Pointer(kind))
	count := C.ravenpass_pasteboard_write(unsafe.Pointer(&content[0]), C.long(len(content)), kind)
	if count < 0 {
		return 0, ErrUnavailable
	}
	return int64(count), nil
}

func writeText(text string) (int64, error) {
	count := C.ravenpass_pasteboard_write_text(unsafe.Pointer(unsafe.StringData(text)), C.long(len(text)))
	if count < 0 {
		return 0, ErrUnavailable
	}
	return int64(count), nil
}

func changeCount() int64 { return int64(C.ravenpass_pasteboard_change_count()) }

func clearContents() { C.ravenpass_pasteboard_clear() }
