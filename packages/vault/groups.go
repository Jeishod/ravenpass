package vault

import (
	"bytes"
	"slices"
	"strings"
	"unicode/utf8"
)

// AcceptGroupName returns a group name as the vault stores it: trimmed, and otherwise as typed.
func AcceptGroupName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || !utf8.ValidString(trimmed) {
		return "", ErrInvalidInput
	}
	if utf8.RuneCountInString(trimmed) > MaxGroupNameLength {
		return "", ErrInvalidInput
	}
	return trimmed, nil
}

// GroupKey is the form in which two group names are compared; one vault holds each key once.
func GroupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// acceptMembership returns ids sorted and deduplicated for a deterministic encoding, refusing a group the vault does not hold.
func acceptMembership(ids []ID, groups []Group) ([]ID, error) {
	if len(ids) > MaxCredentialGroups {
		return nil, ErrResourceLimit
	}
	known := make(map[ID]struct{}, len(groups))
	for _, group := range groups {
		known[group.ID] = struct{}{}
	}
	accepted := make([]ID, 0, len(ids))
	for _, id := range ids {
		if _, exists := known[id]; !exists {
			return nil, ErrNotFound
		}
		if slices.Contains(accepted, id) {
			continue
		}
		accepted = append(accepted, id)
	}
	slices.SortFunc(accepted, func(left, right ID) int {
		return bytes.Compare(left[:], right[:])
	})
	if len(accepted) == 0 {
		return nil, nil
	}
	return accepted, nil
}

func (s *Session) findGroup(id ID) int {
	for i, group := range s.groups {
		if group.ID == id {
			return i
		}
	}
	return -1
}

func (s *Session) nameTaken(name string, except ID) bool {
	folded := GroupKey(name)
	for _, group := range s.groups {
		if group.ID != except && GroupKey(group.Name) == folded {
			return true
		}
	}
	return false
}

// Groups reports the groups the vault holds, in the order they were created.
func (s *Session) Groups() ([]Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, ErrLocked
	}
	return append([]Group(nil), s.groups...), nil
}

// PrepareCreateGroup prepares a new group named name.
func (s *Session) PrepareCreateGroup(name string) (*Pending, ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyToWrite(); err != nil {
		return nil, ID{}, err
	}
	accepted, err := AcceptGroupName(name)
	if err != nil {
		return nil, ID{}, err
	}
	if len(s.groups) >= MaxGroups {
		return nil, ID{}, ErrResourceLimit
	}
	if s.nameTaken(accepted, ID{}) {
		return nil, ID{}, ErrInvalidInput
	}
	id, err := uniqueID(func(candidate ID) bool { return s.findGroup(candidate) >= 0 })
	if err != nil {
		return nil, ID{}, err
	}
	groups := append(append([]Group(nil), s.groups...), Group{ID: id, Name: accepted})
	pending, err := s.prepareWithGroups(append([]entryMeta(nil), s.entries...), append([]sealedBox(nil), s.records...), groups)
	return pending, id, err
}

// PrepareRenameGroup prepares the group with id renamed to name.
func (s *Session) PrepareRenameGroup(id ID, name string) (*Pending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyToWrite(); err != nil {
		return nil, err
	}
	index := s.findGroup(id)
	if index < 0 {
		return nil, ErrNotFound
	}
	accepted, err := AcceptGroupName(name)
	if err != nil {
		return nil, err
	}
	if s.nameTaken(accepted, id) {
		return nil, ErrInvalidInput
	}
	groups := append([]Group(nil), s.groups...)
	groups[index].Name = accepted
	return s.prepareWithGroups(append([]entryMeta(nil), s.entries...), append([]sealedBox(nil), s.records...), groups)
}

// PrepareDeleteGroup prepares the group's removal from the vault and every membership without resealing any record.
func (s *Session) PrepareDeleteGroup(id ID) (*Pending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyToWrite(); err != nil {
		return nil, err
	}
	index := s.findGroup(id)
	if index < 0 {
		return nil, ErrNotFound
	}
	groups := append([]Group(nil), s.groups[:index]...)
	groups = append(groups, s.groups[index+1:]...)
	entries := append([]entryMeta(nil), s.entries...)
	for i := range entries {
		if !slices.Contains(entries[i].groups, id) {
			continue
		}
		remaining := make([]ID, 0, len(entries[i].groups)-1)
		for _, member := range entries[i].groups {
			if member != id {
				remaining = append(remaining, member)
			}
		}
		if len(remaining) == 0 {
			remaining = nil
		}
		entries[i].groups = remaining
	}
	return s.prepareWithGroups(entries, append([]sealedBox(nil), s.records...), groups)
}

// PrepareSetGroups prepares the item with id belonging to exactly groups.
func (s *Session) PrepareSetGroups(id ID, groups []ID) (*Pending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyToWrite(); err != nil {
		return nil, err
	}
	index := s.findItem(id)
	if index < 0 {
		return nil, ErrNotFound
	}
	membership, err := acceptMembership(groups, s.groups)
	if err != nil {
		return nil, err
	}
	entries := append([]entryMeta(nil), s.entries...)
	entries[index].groups = membership
	return s.prepareWithGroups(entries, append([]sealedBox(nil), s.records...), append([]Group(nil), s.groups...))
}
