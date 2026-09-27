//go:build !darwin || !cgo

package ownerauth

import "context"

type systemPlatform struct{}

func (systemPlatform) Available() bool { return false }

func (systemPlatform) Authenticate(context.Context, string) error { return ErrUnavailable }
