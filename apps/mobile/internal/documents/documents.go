// Package documents keeps vaults, saved files and backups in documents granted by the system file picker.
package documents

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

// Provider is the device's access to the documents the owner picks.
type Provider interface {
	// Pick shows the system file picker that creates or opens a document and keeps access to it.
	Pick(create bool, name string) (address string, label storage.Label, picked bool, err error)
	// Create shows the system file picker that creates a document; access lasts until the device restarts.
	Create(name, mediaType string) (address string, created bool, err error)
	// PickFolder shows the system folder picker and keeps access to the folder and the documents created in it.
	PickFolder() (address string, label storage.Label, picked bool, err error)
	// CreateIn creates an empty document in folder; the provider may give it a name other than name.
	CreateIn(folder, name, mediaType string) (address string, err error)
	// Read returns the document's content, failing with storage.ErrTooLarge beyond limit bytes and with
	// storage.ErrNotFound only where the provider reports the document no longer exists.
	Read(address string, limit int64) ([]byte, error)
	// Write replaces the document's content with data.
	Write(address string, data []byte) error
	// Delete removes the document and gives up access to it.
	Delete(address string) error
}

var (
	_ storage.Backend     = (*Backend)(nil)
	_ storage.Store       = (*store)(nil)
	_ storage.Placeholder = (*store)(nil)
)

// Backend keeps vaults in the documents its Provider reaches.
type Backend struct {
	provider Provider
	// mu serializes document reads and writes across every store the Backend opens.
	mu sync.Mutex
}

// NewBackend returns a Backend that reaches documents through provider.
func NewBackend(provider Provider) *Backend {
	return &Backend{provider: provider}
}

func (*Backend) Kind() storage.Kind { return storage.Document }

// Check accepts a content URI with an authority, the form of address the system file picker grants.
func (*Backend) Check(path string) error {
	address, err := url.Parse(path)
	if err != nil || address.Scheme != "content" || address.Host == "" {
		return fmt.Errorf("%w: not a document address", storage.ErrInvalidPath)
	}
	return nil
}

func (backend *Backend) Open(target storage.Target, maxBytes int64) (storage.Store, error) {
	if err := backend.Check(target.Path); err != nil {
		return nil, err
	}
	return &store{backend: backend, address: target.Path, vault: target.Vault, maxBytes: maxBytes}, nil
}

// errKnownVault refuses to discard a document a vault was opened at; a provider can read it empty while it syncs.
var errKnownVault = errors.New("document belongs to a known vault")

// store keeps a vault in one document, written in place: a write that fails partway can truncate it.
type store struct {
	backend *Backend
	address string
	// vault is the identity the selection records for the document, empty until a vault opens there.
	vault    string
	maxBytes int64
}

func (s *store) LoadCiphertext() ([]byte, error) {
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	return s.load()
}

func (s *store) CommitCiphertext(expected *[sha256.Size]byte, candidate []byte, finalize func() error) error {
	if len(candidate) == 0 {
		return storage.ErrEmptyCiphertext
	}
	if int64(len(candidate)) > s.maxBytes {
		return storage.ErrTooLarge
	}
	if finalize == nil {
		return storage.ErrFinalizerRequired
	}
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	current, err := s.head(expected)
	if err != nil {
		return err
	}
	if err := s.backend.provider.Write(s.address, candidate); err != nil {
		return s.failedWrite(current, err)
	}
	written, err := s.read()
	if err != nil {
		return fmt.Errorf("%w: %v", storage.ErrDurabilityUncertain, err)
	}
	if !bytes.Equal(written, candidate) {
		return fmt.Errorf("%w: the document reads back other content", storage.ErrDurabilityUncertain)
	}
	return storage.Finish(finalize)
}

func (s *store) ReconcileCiphertext(expected [sha256.Size]byte, finalize func() error) error {
	if finalize == nil {
		return storage.ErrFinalizerRequired
	}
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	if _, err := s.head(&expected); err != nil {
		return err
	}
	return storage.Finish(finalize)
}

// Restricted is false: the provider, not Ravenpass, decides who reads a document.
func (*store) Restricted() bool { return false }

func (s *store) Remove() error {
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	return s.remove()
}

// DiscardEmpty deletes the document while it is as empty as the picker created it and no vault was opened there.
func (s *store) DiscardEmpty() error {
	if s.vault != "" {
		return errKnownVault
	}
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	content, err := s.read()
	if err != nil {
		return err
	}
	if len(content) > 0 {
		return nil
	}
	return s.remove()
}

func (s *store) head(expected *[sha256.Size]byte) ([]byte, error) {
	current, err := s.load()
	if err := storage.CheckHead(expected, current, err); err != nil {
		return nil, err
	}
	return current, nil
}

// load reports an empty or gone document as storage.ErrNotFound; an unreadable one must stay an error or a new vault
// overwrites it.
func (s *store) load() ([]byte, error) {
	content, err := s.read()
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, storage.ErrNotFound
	}
	return content, nil
}

func (s *store) read() ([]byte, error) {
	content, err := s.backend.provider.Read(s.address, s.maxBytes)
	if err != nil {
		return nil, fmt.Errorf("read vault document: %w", err)
	}
	return content, nil
}

// failedWrite treats a document that no longer holds previous as a possibly truncated vault.
func (s *store) failedWrite(previous []byte, writeErr error) error {
	after, err := s.read()
	if err == nil && bytes.Equal(after, previous) {
		return fmt.Errorf("write vault document: %w", writeErr)
	}
	return fmt.Errorf("%w: %v", storage.ErrDurabilityUncertain, writeErr)
}

func (s *store) remove() error {
	if err := s.backend.provider.Delete(s.address); err != nil {
		return fmt.Errorf("remove vault document: %w", err)
	}
	return nil
}
