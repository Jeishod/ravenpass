package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// ReadSelectedNote returns the note ticket selects.
func (s *Service) ReadSelectedNote(ticket vault.Selection) (vault.Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.Note{}, ErrNotReady
	}
	return s.session.ReadSelectedNote(ticket)
}

// CreateNote saves a new note in groups and returns its ID.
func (s *Service) CreateNote(input vault.NoteInput, groups []vault.ID) (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	pending, id, err := s.session.PrepareCreateNote(input, groups)
	if err != nil {
		return vault.ID{}, err
	}
	if err := s.commit(pending); err != nil {
		return vault.ID{}, err
	}
	return id, nil
}

// EditNote replaces a note's content and membership as a whole.
func (s *Service) EditNote(id vault.ID, input vault.NoteInput, groups []vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareEditNote(id, input, groups)
	if err != nil {
		return err
	}
	return s.commit(pending)
}
