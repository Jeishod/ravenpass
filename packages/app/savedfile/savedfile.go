// Package savedfile reads and writes the files the owner keeps outside Ravenpass, verified before they take their name.
package savedfile

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/dortanes/ravenpass/packages/app/privatefile"
)

// ErrInvalidSize reports a file that is empty, larger than allowed, or not a regular file.
var ErrInvalidSize = errors.New("file has an invalid size")

// ErrUnverified reports a saved file that does not read back as it was written.
var ErrUnverified = errors.New("file verification failed")

// Read returns the content of a regular, non-empty file of at most limit bytes.
func Read(path string, limit int64) ([]byte, error) {
	data, err := privatefile.Read(path, limit)
	if errors.Is(err, fs.ErrInvalid) {
		return nil, ErrInvalidSize
	}
	return data, err
}

// Replace writes data to path, replacing a file the owner already agreed to replace.
func Replace(path string, data []byte, limit int64) error {
	return write(path, data, limit, false)
}

// Create writes data to a new file at path, else fs.ErrExist; the check is not atomic, exFAT has no hard links.
func Create(path string, data []byte, limit int64) error {
	return write(path, data, limit, true)
}

func write(path string, data []byte, limit int64, exclusive bool) error {
	if len(data) == 0 || int64(len(data)) > limit {
		return ErrInvalidSize
	}
	pending, err := privatefile.Prepare(path, data)
	if err != nil {
		return err
	}
	defer pending.Discard()
	readback, err := Read(pending.Path(), limit)
	defer clear(readback)
	if err != nil || !bytes.Equal(readback, data) {
		return ErrUnverified
	}
	if exclusive {
		if _, err := os.Lstat(path); err == nil {
			return &fs.PathError{Op: "create", Path: path, Err: fs.ErrExist}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	err = pending.Place()
	if err != nil && exclusive && pending.Placed() {
		if removeErr := os.Remove(path); removeErr != nil {
			slog.Warn("remove unsynced file", "err", removeErr)
		}
	}
	return err
}
