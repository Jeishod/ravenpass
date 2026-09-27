package api

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/vault"
)

// presentGroup names failures of a group and presents the rest as present does.
func presentGroup(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, vault.ErrInvalidInput):
		return fail(failureGroupNameInvalid)
	case errors.Is(err, vault.ErrNotFound):
		return fail(failureGroupUnknown)
	default:
		return present(err)
	}
}

// Group is a name the person gave a set of credentials. It carries no credential data.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListGroups reports the open vault's groups.
func (s *Service) ListGroups() ([]Group, error) {
	groups, err := s.vault.Groups()
	if err != nil {
		return nil, present(err)
	}
	result := make([]Group, len(groups))
	for i, group := range groups {
		result[i] = Group{ID: group.ID.String(), Name: group.Name}
	}
	return result, nil
}

// CreateGroup creates a group named name and returns its id.
func (s *Service) CreateGroup(name string) (string, error) {
	id, err := s.vault.CreateGroup(name)
	if err != nil {
		return "", presentGroup(err)
	}
	return id.String(), nil
}

// RenameGroup reports a malformed or unknown id as an unknown group and a blank, overlong or taken name as invalid.
func (s *Service) RenameGroup(id string, name string) error {
	parsed, err := vault.ParseID(id)
	if err != nil {
		return fail(failureGroupUnknown)
	}
	return presentGroup(s.vault.RenameGroup(parsed, name))
}

// DeleteGroup reports a malformed or unknown id as an unknown group.
func (s *Service) DeleteGroup(id string) error {
	parsed, err := vault.ParseID(id)
	if err != nil {
		return fail(failureGroupUnknown)
	}
	return presentGroup(s.vault.DeleteGroup(parsed))
}

// SetItemGroups replaces the membership of an item of either kind.
func (s *Service) SetItemGroups(id string, groups []string) error {
	parsed, err := vault.ParseID(id)
	if err != nil {
		return fail(failureItemUnreadable)
	}
	membership, err := s.knownGroups(groups)
	if err != nil {
		return err
	}
	return presentGroup(s.vault.SetItemGroups(parsed, membership))
}

// DefaultGroup reports the group new passwords join, empty for none or one the vault no longer holds.
func (s *Service) DefaultGroup() (string, error) {
	group, held, err := s.vault.DefaultGroup(s.preferences.DefaultGroup())
	if err != nil {
		return "", present(err)
	}
	if !held {
		return "", nil
	}
	return group.String(), nil
}

// SetDefaultGroup records the group new passwords join; an empty id clears it.
func (s *Service) SetDefaultGroup(id string) error {
	if id == "" {
		return present(s.preferences.SetDefaultGroup(""))
	}
	parsed, err := vault.ParseID(id)
	if err != nil {
		return fail(failureGroupUnknown)
	}
	groups, err := s.vault.Groups()
	if err != nil {
		return present(err)
	}
	for _, group := range groups {
		if group.ID == parsed {
			return present(s.preferences.SetDefaultGroup(id))
		}
	}
	return fail(failureGroupUnknown)
}

// knownGroups refuses unknown group ids before a write, whose core error could not tell them from field errors.
func (s *Service) knownGroups(groups []string) ([]vault.ID, error) {
	membership, err := parseGroupIDs(groups)
	if err != nil || len(membership) == 0 {
		return membership, err
	}
	current, err := s.vault.Groups()
	if err != nil {
		return nil, present(err)
	}
	for _, id := range membership {
		held := false
		for _, group := range current {
			if group.ID == id {
				held = true
				break
			}
		}
		if !held {
			return nil, fail(failureGroupUnknown)
		}
	}
	return membership, nil
}

func parseGroupIDs(groups []string) ([]vault.ID, error) {
	parsed, valid := parseIDs(groups)
	if !valid {
		return nil, fail(failureGroupUnknown)
	}
	return parsed, nil
}

// parseIDs reads identifiers the interface passed, nil for none.
func parseIDs(values []string) ([]vault.ID, bool) {
	if len(values) == 0 {
		return nil, true
	}
	parsed := make([]vault.ID, len(values))
	for i, value := range values {
		id, err := vault.ParseID(value)
		if err != nil {
			return nil, false
		}
		parsed[i] = id
	}
	return parsed, true
}

// idStrings reports identifiers as the interface reads them, an empty array for none.
func idStrings(ids []vault.ID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}
