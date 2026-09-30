//go:build !darwin || !cgo

package appearance

import "github.com/dortanes/ravenpass/packages/app/preferences"

func apply(preferences.Appearance) {}
