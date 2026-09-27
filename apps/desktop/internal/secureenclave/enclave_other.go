//go:build !darwin || !arm64 || !cgo

package secureenclave

import "github.com/dortanes/ravenpass/packages/app/unlock"

func create(policy, []byte) ([]byte, []byte, [unlock.SecretSize]byte, error) {
	return nil, nil, [unlock.SecretSize]byte{}, ErrUnavailable
}

func derive(policy, []byte, []byte, []byte, string) ([unlock.SecretSize]byte, error) {
	return [unlock.SecretSize]byte{}, ErrUnavailable
}
