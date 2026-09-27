//go:build !darwin || !cgo

package trash

func moveToTrash(string) error { return ErrUnavailable }
