//go:build !darwin || !cgo

package identitystore

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/app/autofill"
)

var errNoStore = errors.New("a credential identity store exists on macOS only")

func (Mac) Enabled() bool { return false }

func (Mac) Replace([]autofill.CredentialIdentity) error { return errNoStore }

func (Mac) RemoveAll() error { return errNoStore }
