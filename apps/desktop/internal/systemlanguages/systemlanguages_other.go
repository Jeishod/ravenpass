//go:build !darwin || !cgo

package systemlanguages

func preferred() []string { return nil }
