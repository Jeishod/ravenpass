//go:build darwin && cgo

package printing

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework WebKit
#include "printing_darwin.h"
*/
import "C"

import "unsafe"

// Print shows the print panel for page and returns once the owner prints or cancels. It must not run on the main
// thread, which shows the panel.
func (Mac) Print(job string, page []byte) error {
	if len(page) == 0 {
		return ErrUnavailable
	}
	if C.ravenpass_print_page((*C.char)(unsafe.Pointer(unsafe.StringData(job))), C.size_t(len(job)),
		(*C.char)(unsafe.Pointer(unsafe.SliceData(page))), C.size_t(len(page))) != 0 {
		return ErrUnavailable
	}
	return nil
}
