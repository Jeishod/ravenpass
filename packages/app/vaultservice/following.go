package vaultservice

import (
	"crypto/sha256"
	"errors"

	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

// stagedDivergence is an opened vault whose file diverged from witness, held until adopted.
type stagedDivergence struct {
	divergence *vault.Divergence
	name       string
	witness    *vault.Witness
}

func (s *Service) discardDivergence() {
	if s.diverged != nil {
		s.diverged.divergence.Discard()
		s.diverged = nil
	}
}

// AdoptDiverged opens the held diverged vault and acknowledges its file's version, dropping local changes.
func (s *Service) AdoptDiverged() (vault.Head, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stage := s.diverged
	if stage == nil {
		return vault.Head{}, ErrNoPendingSetup
	}
	s.diverged = nil
	if s.session != nil || s.creating != nil || s.recovering != nil {
		stage.divergence.Discard()
		return vault.Head{}, ErrSetupInProgress
	}
	opened, err := stage.divergence.Adopt(s.advanceWitness(stage.name, stage.witness))
	if err != nil {
		return vault.Head{}, err
	}
	return s.adoptSession(opened)
}

// FileChange is what Follow found in the open vault's file.
type FileChange uint8

// Results of Follow.
const (
	// FileUnchanged is the open vault's version, an unreadable file, or no open vault.
	FileUnchanged FileChange = iota
	// FileAdopted is a successor another device saved, which the open vault now shows.
	FileAdopted
	// FileRefused is any other file, or no file; the vault locked.
	FileRefused
)

// Follow adopts a successor file without unlocking and locks when the file is gone; an unreadable or still-changing
// file waits for the next check. The file is read without holding the service, since a document provider can take
// seconds; a read that a save, another open or a move overtook waits for the next check.
func (s *Service) Follow() FileChange {
	opened, head, ok := s.openHead()
	if !ok {
		return FileUnchanged
	}
	location := s.files.Status().Current
	container, err := s.files.LoadCiphertext()
	defer clear(container)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != opened || !s.files.Status().Current.Same(location) {
		return FileUnchanged
	}
	if current, headErr := opened.Head(); headErr == nil && current != head {
		return FileUnchanged
	}
	change := s.followLoaded(head, container, err)
	if change == FileRefused {
		s.lockSession()
	}
	return change
}

// openHead is the open session and its head, false while none is open or its head cannot be read.
func (s *Service) openHead() (*vault.Session, vault.Head, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, vault.Head{}, false
	}
	head, err := s.session.Head()
	return s.session, head, err == nil
}

// followFile is Follow without the lock and with the file read under s.mu: a refused file leaves the vault open for
// the caller to lock. The caller holds s.mu.
func (s *Service) followFile() FileChange {
	if s.session == nil {
		return FileUnchanged
	}
	head, err := s.session.Head()
	if err != nil {
		return FileRefused
	}
	container, err := s.files.LoadCiphertext()
	defer clear(container)
	return s.followLoaded(head, container, err)
}

// followLoaded places the file read as container, or failing with err, against the open vault at head. The caller
// holds s.mu.
func (s *Service) followLoaded(head vault.Head, container []byte, err error) FileChange {
	if errors.Is(err, storage.ErrNotFound) && !errors.Is(err, storage.ErrEmptyFile) {
		return FileRefused
	}
	if err != nil || sha256.Sum256(container) == head.Hash {
		return FileUnchanged
	}
	acknowledged := vault.WitnessFor(head)
	decision, err := s.session.Follow(container, s.advanceWitness(head.VaultID.String(), &acknowledged))
	switch {
	// A file another device is still writing reads as malformed or fails to authenticate until it is whole.
	case errors.Is(err, storage.ErrStaleHead), errors.Is(err, vault.ErrMalformed), errors.Is(err, vault.ErrAuthentication):
		return FileUnchanged
	case err != nil:
		return FileRefused
	case decision == vault.WitnessAdvance:
		s.changes.Record()
		s.states.Record()
		return FileAdopted
	default:
		return FileUnchanged
	}
}
