// Package filewatch notices when the open vault's file changes on disk.
package filewatch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"time"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

// A kqueue watch reported only the first of two replacements by rename (measured on macOS 27, 2026-09-24).
const checkInterval = 2 * time.Second

// Vault is the vault whose file the watcher follows.
type Vault interface {
	Unlocked() bool
	Storage() storage.Status
}

// Watcher follows the open vault's file when its identity, size or modification time changes.
type Watcher struct {
	vault  Vault
	follow func()
	seen   fs.FileInfo
}

// New watches an open vault's local file and calls follow once on open, for each change, and when the file goes.
func New(vault Vault, follow func()) *Watcher {
	return &Watcher{vault: vault, follow: follow}
}

// Run checks the file until ctx ends.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check()
		}
	}
}

func (w *Watcher) check() {
	current := w.vault.Storage().Current
	if !w.vault.Unlocked() || current.Kind != storage.LocalFile {
		w.seen = nil
		return
	}
	info, err := os.Stat(current.Path)
	if err != nil {
		gone := w.seen != nil && errors.Is(err, fs.ErrNotExist)
		w.seen = nil
		if gone {
			w.follow()
		}
		return
	}
	if w.seen != nil && os.SameFile(w.seen, info) && w.seen.Size() == info.Size() && w.seen.ModTime().Equal(info.ModTime()) {
		return
	}
	w.seen = info
	w.follow()
}
