//go:build darwin && cgo

package ownerauth

/*
#cgo CFLAGS: -fblocks
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication
#include <stdlib.h>
#include "auth_darwin.h"
*/
import "C"

import (
	"context"
	"unsafe"
)

type systemPlatform struct{}

func (systemPlatform) Available() bool { return C.ravenpass_device_owner_available() == 1 }

func (systemPlatform) Authenticate(ctx context.Context, reason string) error {
	text := C.CString(reason)
	defer C.free(unsafe.Pointer(text))
	prompt := C.ravenpass_owner_prompt_start(text)
	if prompt == nil {
		return ErrUnavailable
	}
	defer C.ravenpass_owner_prompt_free(prompt)
	dismissed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		C.ravenpass_owner_prompt_cancel(prompt)
		close(dismissed)
	})
	result := C.ravenpass_owner_prompt_wait(prompt)
	// The prompt is freed only once a dismissal that started has finished with it.
	if !stop() {
		<-dismissed
		return ctx.Err()
	}
	switch result {
	case C.RAVENPASS_AUTH_SUCCESS:
		return nil
	case C.RAVENPASS_AUTH_CANCELED:
		return ErrCanceled
	case C.RAVENPASS_AUTH_FAILED:
		return ErrFailed
	default:
		return ErrUnavailable
	}
}
