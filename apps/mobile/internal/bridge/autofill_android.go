//go:build android

package bridge

import (
	"sync/atomic"

	"github.com/dortanes/ravenpass/packages/app/api"
)

var autofillServer atomic.Pointer[func(request []byte) []byte]

// ServeAutofill answers the UTF-8 JSON requests of com.dortanes.ravenpass.autofill.Core with serve.
func ServeAutofill(serve func(request []byte) []byte) {
	autofillServer.Store(&serve)
}

var _ api.SystemAutofill = SystemAutofill{}

// SystemAutofill is the device's choice of autofill service and, from Android 14, of passkey providers.
type SystemAutofill struct{}

// Status reports whether Ravenpass is the autofill service and one of the passkey providers.
func (SystemAutofill) Status() api.SystemAutofillStatus {
	return systemAutofillStatus(autofillSelected(), passkeyProvider())
}

// Choose shows the system screen that selects the autofill service and returns once the owner leaves it.
func (SystemAutofill) Choose() error { return leftError(selectAutofill()) }
