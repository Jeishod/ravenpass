// Package genhistory generates passwords and passphrases and keeps every one made, with the options last used, in a
// file on this device sealed as the open vault's device data.
package genhistory

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/privatefile"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	sealPurpose = "generator-history"
	// MaxEntries bounds the history; the oldest entries go first past it.
	MaxEntries = 2000
	// maxFileSize is a sealed history's ciphertext limit plus its envelope.
	maxFileSize = 32<<20 + 1<<10
)

// Vault is the open vault as the history needs it; sealing for a vault no longer open fails.
type Vault interface {
	OpenVaultID() (vault.ID, error)
	SealDeviceData(id vault.ID, name string, plaintext []byte) ([]byte, error)
	OpenDeviceData(id vault.ID, name string, sealed []byte) ([]byte, error)
}

// Entry is one generated value.
type Entry struct {
	Value string    `json:"value"`
	Kind  string    `json:"kind"`
	At    time.Time `json:"at"`
}

// State is the open vault's history, newest first, and the options last used.
type State struct {
	Options Options
	History []Entry
}

// stored is the sealed file's plaintext; History is oldest first.
type stored struct {
	Options Options `json:"options"`
	History []Entry `json:"history"`
}

// Store keeps each vault's generator history in directory.
type Store struct {
	directory string
	vault     Vault
	wordlist  func() []string
	random    io.Reader
	now       func() time.Time
	mu        sync.Mutex
}

// New returns a store keeping histories in directory; wordlist supplies passphrase words.
func New(directory string, vault Vault, wordlist func() []string) (*Store, error) {
	if directory == "" || vault == nil || wordlist == nil {
		return nil, errors.New("history directory, vault and wordlist are required")
	}
	return &Store{directory: directory, vault: vault, wordlist: wordlist, random: rand.Reader, now: time.Now}, nil
}

// State reads the open vault's history and options.
func (s *Store) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, data, err := s.read()
	if err != nil {
		return State{}, err
	}
	history := make([]Entry, len(data.History))
	for i, entry := range data.History {
		history[len(history)-1-i] = entry
	}
	return State{Options: data.Options, History: history}, nil
}

// Generate makes a value with options, records it and keeps options as the last used.
func (s *Store) Generate(options Options) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := options.Validate(); err != nil {
		return Entry{}, err
	}
	id, data, err := s.read()
	if err != nil {
		return Entry{}, err
	}
	return s.record(id, data, options)
}

// record makes a value with options, appends it to data and writes data; the caller holds s.mu.
func (s *Store) record(id vault.ID, data stored, options Options) (Entry, error) {
	var wordlist []string
	if options.Kind == KindPassphrase {
		wordlist = s.wordlist()
	}
	value, err := Generate(options, wordlist, s.random)
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{Value: value, Kind: options.Kind, At: s.now().UTC()}
	data.Options = options
	data.History = append(data.History, entry)
	if excess := len(data.History) - MaxEntries; excess > 0 {
		data.History = data.History[excess:]
	}
	if err := s.write(id, data); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Clear forgets the open vault's history and keeps its options.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, data, err := s.read()
	if err != nil {
		return err
	}
	data.History = nil
	return s.write(id, data)
}

// read opens the open vault's file; a vault without one starts with the default options and no history.
func (s *Store) read() (vault.ID, stored, error) {
	id, err := s.vault.OpenVaultID()
	if err != nil {
		return vault.ID{}, stored{}, err
	}
	sealed, err := privatefile.Read(s.path(id), maxFileSize)
	if errors.Is(err, fs.ErrNotExist) {
		return id, stored{Options: DefaultOptions()}, nil
	}
	if err != nil {
		return vault.ID{}, stored{}, err
	}
	plaintext, err := s.vault.OpenDeviceData(id, sealPurpose, sealed)
	if unreadable(err) {
		// A key change that could not seal it again leaves a file no key opens; keep it aside rather than lose it.
		if err := s.setAside(id); err != nil {
			return vault.ID{}, stored{}, err
		}
		return id, stored{Options: DefaultOptions()}, nil
	}
	if err != nil {
		return vault.ID{}, stored{}, err
	}
	defer clear(plaintext)
	var data stored
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return vault.ID{}, stored{}, vault.ErrMalformed
	}
	if data.Options.Validate() != nil {
		data.Options = DefaultOptions()
	}
	return id, data, nil
}

func (s *Store) write(id vault.ID, data stored) error {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return err
	}
	sealed, err := s.vault.SealDeviceData(id, sealPurpose, plaintext)
	clear(plaintext)
	if err != nil {
		return err
	}
	return privatefile.Write(s.path(id), sealed)
}

// ResealDeviceData seals the vault id's history again for a key change; it runs while the vault service is busy, so
// it reads the file without the vault and leaves s.mu alone. A Generate waiting on the service writes after it, from
// the history it read, sealed with the new key.
func (s *Store) ResealDeviceData(id vault.ID, open, seal func(name string, data []byte) ([]byte, error)) (func() error, error) {
	path := s.path(id)
	sealed, err := privatefile.Read(path, maxFileSize)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plaintext, err := open(sealPurpose, sealed)
	if err != nil {
		return nil, err
	}
	resealed, err := seal(sealPurpose, plaintext)
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
