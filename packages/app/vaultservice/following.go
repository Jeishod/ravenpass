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
// file waits for the next check.
func (s *Service) Follow() FileChange {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return FileUnchanged
	}
	head, err := s.session.Head()
	if err != nil {
		s.lockSession()
		return FileRefused
	}
	container, err := s.files.LoadCiphertext()
	if errors.Is(err, storage.ErrNotFound) {
		s.lockSession()
		return FileRefused
	}
	if err != nil || sha256.Sum256(container) == head.Hash {
		return FileUnchanged
	}
	acknowledged := vault.WitnessFor(head)
	decision, err := s.session.Follow(container, s.advanceWitness(head.VaultID.String(), &acknowledged))
	switch {
	case errors.Is(err, storage.ErrStaleHead):
		return FileUnchanged
	case err != nil:
		s.lockSession()
		return FileRefused
	case decision == vault.WitnessAdvance:
		s.changes.Record()
		s.states.Record()
		return FileAdopted
	default:
		return FileUnchanged
	}
}
