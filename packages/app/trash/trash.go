// Package trash moves a file to the system trash, where the person can still restore it.
package trash

import "errors"

// Errors MoveToTrash fails with.
var (
	ErrUnavailable = errors.New("trash is unavailable on this platform")
	ErrRefused     = errors.New("file could not be moved to the trash")
)

// MoveToTrash moves the file at path to the trash of the volume it is on.
func MoveToTrash(path string) error {
	if path == "" {
		return ErrRefused
	}
	return moveToTrash(path)
}
