// Package genhistory keeps the passwords the generator handed out, for each vault, in a file on this device sealed as
// the open vault's device data, until they are older than the period the owner keeps them.
package genhistory

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dortanes/ravenpass/packages/app/privatefile"
	"github.com/dortanes/ravenpass/packages/vault"
)

// Modes of a generated password, as the generator names them.
const (
	ModeWords      = "words"
	ModeCharacters = "characters"
)

const (
	sealName = "generator-history"
	// MaxEntries bounds the history; the oldest entries go first past it.
	MaxEntries = 2000
	// MaxValueBytes is the longest password the history records.
	MaxValueBytes = 1024
	// maxFileSize is a sealed history's ciphertext limit plus its envelope.
	maxFileSize = 32<<20 + 1<<10
	day         = 24 * time.Hour
)

// ErrInvalidEntry reports a password or mode the history does not record.
var ErrInvalidEntry = errors.New("generated password is invalid")

// Vault is the open vault as the history needs it; sealing for a vault no longer open fails.
type Vault interface {
	OpenVaultID() (vault.ID, error)
	SealDeviceData(id vault.ID, name string, plaintext []byte) ([]byte, error)
	OpenDeviceData(id vault.ID, name string, sealed []byte) ([]byte, error)
}

// Entry is one password the generator handed out.
type Entry struct {
	Value string    `json:"value"`
	Mode  string    `json:"mode"`
	At    time.Time `json:"at"`
}

func (e Entry) valid() bool {
	return (e.Mode == ModeWords || e.Mode == ModeCharacters) && e.Value != "" && len(e.Value) <= MaxValueBytes &&
		utf8.ValidString(e.Value)
}

// stored is the sealed file's plaintext; History is oldest first.
type stored struct {
	History []Entry `json:"history"`
}

// Store keeps each vault's generator history in directory.
type Store struct {
	directory string
	vault     Vault
	keptDays  func() int
	now       func() time.Time
	mu        sync.Mutex
}

// New returns a store keeping histories in directory; keptDays reports how many days an entry is kept.
func New(directory string, vault Vault, keptDays func() int) (*Store, error) {
	if directory == "" || vault == nil || keptDays == nil {
		return nil, errors.New("history directory, vault and kept days are required")
	}
	return &Store{directory: directory, vault: vault, keptDays: keptDays, now: time.Now}, nil
}

// History reads the open vault's history, newest first.
func (s *Store) History() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, history, err := s.read()
	if err != nil {
		return nil, err
	}
	newest := make([]Entry, len(history))
	for i, entry := range history {
		newest[len(newest)-1-i] = entry
	}
	return newest, nil
}

// Record adds a password the generator handed out in mode.
func (s *Store) Record(value, mode string) (Entry, error) {
	entry := Entry{Value: value, Mode: mode}
	if !entry.valid() {
		return Entry{}, ErrInvalidEntry
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, history, err := s.read()
	if err != nil {
		return Entry{}, err
	}
	entry.At = s.now().UTC()
	history = append(history, entry)
	if excess := len(history) - MaxEntries; excess > 0 {
		history = history[excess:]
	}
	if err := s.write(id, history); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Clear forgets the open vault's history by removing its file.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.vault.OpenVaultID()
	if err != nil {
		return err
	}
	if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Past counts the entries a period of days would remove.
func (s *Store) Past(days int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, history, err := s.read()
	if err != nil {
		return 0, err
	}
	cutoff := s.now().Add(-time.Duration(days) * day)
	count := 0
	for _, entry := range history {
		if !entry.At.After(cutoff) {
			count++
		}
	}
	return count, nil
}

// Prune removes the entries kept longer than the period now set.
func (s *Store) Prune() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, err := s.read()
	return err
}

// read opens the open vault's history, oldest first, and writes it back without the entries past the period; a vault
// without a file has none.
func (s *Store) read() (vault.ID, []Entry, error) {
	id, err := s.vault.OpenVaultID()
	if err != nil {
		return vault.ID{}, nil, err
	}
	plaintext, err := s.open(id)
	if errors.Is(err, fs.ErrNotExist) {
		return id, nil, nil
	}
	if err != nil {
		return vault.ID{}, nil, err
	}
	defer clear(plaintext)
	var data stored
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return vault.ID{}, nil, vault.ErrMalformed
	}
	cutoff := s.now().Add(-time.Duration(s.keptDays()) * day)
	kept := data.History[:0]
	for _, entry := range data.History {
		if entry.valid() && entry.At.After(cutoff) {
			kept = append(kept, entry)
		}
	}
	if len(kept) != len(data.History) {
		if err := s.write(id, kept); err != nil {
			return vault.ID{}, nil, err
		}
	}
	return id, kept, nil
}

// open reads and opens the vault id's file. A key change may seal it again between the read and the opening, so a
// file that fails to open is read once more; one that still fails is set aside rather than lost.
func (s *Store) open(id vault.ID) ([]byte, error) {
	var last []byte
	for {
		sealed, err := privatefile.Read(s.path(id), maxFileSize)
		if err != nil {
			return nil, err
		}
		plaintext, err := s.vault.OpenDeviceData(id, sealName, sealed)
		if !unreadable(err) {
			return plaintext, err
		}
		if last != nil && bytes.Equal(last, sealed) {
			if err := s.setAside(id); err != nil {
				return nil, err
			}
			return nil, fs.ErrNotExist
		}
		last = sealed
	}
}

func (s *Store) write(id vault.ID, history []Entry) error {
	plaintext, err := json.Marshal(stored{History: history})
	if err != nil {
		return err
	}
	sealed, err := s.vault.SealDeviceData(id, sealName, plaintext)
	clear(plaintext)
	if err != nil {
		return err
	}
	return privatefile.Write(s.path(id), sealed)
}

// ResealDeviceData seals the vault id's history again for a key change. It runs while the vault service is busy, so it
// reads the file without the vault and leaves s.mu to a Record waiting on the service, which writes after it.
func (s *Store) ResealDeviceData(id vault.ID, open, seal func(name string, data []byte) ([]byte, error)) (func() error, error) {
	path := s.path(id)
	sealed, err := privatefile.Read(path, maxFileSize)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plaintext, err := open(sealName, sealed)
	if err != nil {
		return nil, err
	}
	resealed, err := seal(sealName, plaintext)
	clear(plaintext)
	if err != nil {
		return nil, err
	}
	return func() error { return privatefile.Write(path, resealed) }, nil
}

// setAside renames the vault id's unreadable history out of the way, keeping it for whatever key may still open it.
func (s *Store) setAside(id vault.ID) error {
	path := s.path(id)
	aside := fmt.Sprintf("%s.unreadable-%d", path, s.now().UTC().Unix())
	slog.Warn("set aside a generator history the open vault cannot read", "path", aside)
	return os.Rename(path, aside)
}

// unreadable reports a history file whose own content failed to open.
func unreadable(err error) bool {
	return errors.Is(err, vault.ErrAuthentication) || errors.Is(err, vault.ErrMalformed) ||
		errors.Is(err, vault.ErrUnsupported)
}

func (s *Store) path(id vault.ID) string {
	return filepath.Join(s.directory, id.String())
}
