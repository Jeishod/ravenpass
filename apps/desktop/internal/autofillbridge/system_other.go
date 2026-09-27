//go:build !darwin || !cgo

package autofillbridge

import (
	"errors"
	"net"
)

// Mac admits nothing where macOS code signatures and App Groups do not exist.
type Mac struct{}

func (Mac) Signed() bool { return false }

func (Mac) Admits(net.Conn) bool { return false }

func (Mac) Container() (string, error) {
	return "", errors.New("App Groups exist on macOS only")
}

func (Mac) AnnounceListening() {}
