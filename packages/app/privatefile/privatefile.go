// Package privatefile reads and atomically replaces files only the user may read.
package privatefile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// BackupExclusion keeps a file out of the system's backups.
type BackupExclusion interface {
	Exclude(path string) error
}

// Option adjusts how Write places a file.
type Option func(*options)

type options struct {
	backups BackupExclusion
}

// ExcludedFromBackups marks the file excluded from backups before it takes its name.
func ExcludedFromBackups(exclusion BackupExclusion) Option {
	return func(o *options) { o.backups = exclusion }
}

// Write replaces the file at path with data, creating its 0700 directory when missing.
func Write(path string, data []byte, opts ...Option) error {
	var settings options
	for _, apply := range opts {
		apply(&settings)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	pending, err := Prepare(path, data)
	if err != nil {
		return err
	}
	defer pending.Discard()
	if settings.backups != nil {
		// A rename keeps the mark and a replacing file drops it, so every write marks again.
		if err := settings.backups.Exclude(pending.Path()); err != nil {
			slog.Warn("exclude file from backups", "err", err)
		}
	}
	return pending.Place()
}

// Pending is data held in a 0600 temporary file beside its destination until Place renames it.
type Pending struct {
	path        string
	destination string
	placed      bool
}

// Prepare writes and flushes data to a new temporary file in the destination's existing directory.
func Prepare(destination string, data []byte) (*Pending, error) {
	file, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+"-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary file: %w", err)
	}
	pending := &Pending{path: file.Name(), destination: destination}
	if err := fill(file, data); err != nil {
		pending.Discard()
		return nil, err
	}
	return pending, nil
}

func fill(file *os.File, data []byte) error {
	err := file.Chmod(0600)
	if err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

// Path is the temporary file's path.
func (p *Pending) Path() string {
	return p.path
}

// Placed reports whether the temporary file took the destination's name.
func (p *Pending) Placed() bool {
	return p.placed
}

// Place renames the temporary file onto the destination and flushes the directory entry.
func (p *Pending) Place() error {
	if err := os.Rename(p.path, p.destination); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	p.placed = true
	return syncDirectory(filepath.Dir(p.destination))
}

// Discard removes the temporary file unless it was placed.
func (p *Pending) Discard() {
	if p.placed {
		return
	}
	if err := os.Remove(p.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("remove temporary file", "err", err)
	}
}

// syncDirectory flushes a directory's entries to storage.
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

// Read returns a regular, non-empty file of at most limit bytes; any other file is fs.ErrInvalid.
func Read(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, fs.ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > limit {
		clear(data)
		return nil, fs.ErrInvalid
	}
	return data, nil
}
