//go:build !darwin || !cgo

package backupexclusion

func exclude(string) error { return nil }
