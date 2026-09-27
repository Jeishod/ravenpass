//go:build !darwin || !cgo

package printing

// Print fails: the print panel exists on macOS only.
func (Mac) Print(string, []byte) error { return ErrUnavailable }
