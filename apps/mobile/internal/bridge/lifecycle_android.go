//go:build android

package bridge

import "sync/atomic"

var interfaceDestroyed atomic.Pointer[func()]

// OnInterfaceDestroyed runs destroyed on the main thread each time the interface's activity is destroyed.
func OnInterfaceDestroyed(destroyed func()) {
	interfaceDestroyed.Store(&destroyed)
}
