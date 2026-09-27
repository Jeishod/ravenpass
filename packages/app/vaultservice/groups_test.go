package vaultservice

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func createTestGroup(t *testing.T, service *Service, name string) vault.ID {
	t.Helper()
	id, err := service.CreateGroup(name)
	if err != nil {
		t.Fatalf("create group %q: %v", name, err)
	}
	return id
}

func TestTheDefaultGroupIsOneTheVaultHolds(t *testing.T) {
	service, _ := readyVault(t)
	work := createTestGroup(t, service, "Work")
	if group, held, err := service.DefaultGroup(work.String()); err != nil || !held || group != work {
		t.Fatalf("a held group = %v, %v, error = %v", group, held, err)
	}
	for _, recorded := range []string{"", vault.ID{0xee}.String(), "work"} {
		if group, held, err := service.DefaultGroup(recorded); err != nil || held || group != (vault.ID{}) {
			t.Fatalf("recorded %q = %v, %v, error = %v", recorded, group, held, err)
		}
	}
	if err := service.DeleteGroup(work); err != nil {
		t.Fatal(err)
	}
	if _, held, err := service.DefaultGroup(work.String()); err != nil || held {
		t.Fatalf("a deleted group reads as held = %v, error = %v", held, err)
	}
	service.Lock()
	if _, held, err := service.DefaultGroup(""); err != nil || held {
		t.Fatalf("no recorded group while locked = %v, error = %v", held, err)
	}
	if _, _, err := service.DefaultGroup(work.String()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("a recorded group while locked: got %v, want ErrNotReady", err)
	}
}

func groupNames(groups []vault.Group) []string {
	names := make([]string, len(groups))
	for i, group := range groups {
		names[i] = group.Name
	}
	return names
}

func entryGroups(t *testing.T, service *Service, id vault.ID) []vault.ID {
	t.Helper()
	entries, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry.Groups
		}
	}
	t.Fatalf("credential %s is not listed", id)
	return nil
}

func readTestCredential(t *testing.T, service *Service, id vault.ID) vault.Credential {
	t.Helper()
	selection, err := service.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.ReadSelected(selection)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func TestGroupsRoundTripInCreationOrder(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	createTestVault(t, service)

	work := createTestGroup(t, service, "Work")
	personal := createTestGroup(t, service, "Personal")
	archive := createTestGroup(t, service, "Archive")

	groups, err := service.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if got := groupNames(groups); len(got) != 3 || got[0] != "Work" || got[1] != "Personal" || got[2] != "Archive" {
		t.Fatalf("groups after creation = %v", got)
	}

	if err := service.RenameGroup(personal, "Home"); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteGroup(work); err != nil {
		t.Fatal(err)
	}

	groups, err = service.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if got := groupNames(groups); len(got) != 2 || got[0] != "Home" || got[1] != "Archive" {
		t.Fatalf("groups after a rename and a delete = %v", got)
	}
	if groups[0].ID != personal || groups[1].ID != archive {
		t.Fatalf("a group changed identity: %+v", groups)
	}

	if err := service.RenameGroup(work, "Work"); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("renaming a deleted group = %v", err)
	}
	if err := service.DeleteGroup(work); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("deleting a deleted group = %v", err)
	}
}

func TestDeletingAGroupLeavesItsCredentialsIntact(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	createTestVault(t, service)

	work := createTestGroup(t, service, "Work")
	archive := createTestGroup(t, service, "Archive")
	input := vault.CredentialInput{
		Label: "Mail", Websites: []string{"https://example.com", "https://admin.example.com"}, Login: "alice",
		Email: "alice@example.com", Password: "a secret\nwith spaces", Notes: "recovery codes",
	}
	id, err := service.CreateCredential(input, []vault.ID{work, archive})
	if err != nil {
		t.Fatal(err)
	}
	if got := entryGroups(t, service, id); len(got) != 2 {
		t.Fatalf("membership at creation = %v", got)
	}

	if err := service.DeleteGroup(work); err != nil {
		t.Fatal(err)
	}
	remaining := entryGroups(t, service, id)
	if len(remaining) != 1 || remaining[0] != archive {
		t.Fatalf("membership after the delete = %v", remaining)
	}
	if credential := readTestCredential(t, service, id); !reflect.DeepEqual(credential.CredentialInput, input) {
		t.Fatalf("credential after the delete = %+v", credential)
	}

	if err := service.DeleteGroup(archive); err != nil {
		t.Fatal(err)
	}
	if got := entryGroups(t, service, id); len(got) != 0 {
		t.Fatalf("membership after the last group was deleted = %v", got)
	}
	if credential := readTestCredential(t, service, id); !reflect.DeepEqual(credential.CredentialInput, input) {
		t.Fatalf("credential without groups = %+v", credential)
	}
}

func TestSetItemGroupsRefusesUnknownIdentifiers(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)

	work := createTestGroup(t, service, "Work")
	id, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, []vault.ID{work})
	if err != nil {
		t.Fatal(err)
	}
	head, err := service.State()
	if err != nil {
		t.Fatal(err)
	}
	saved := bytes.Clone(files.data)

	unknown := vault.ID{9}
	cases := []struct {
		name       string
		credential vault.ID
		groups     []vault.ID
	}{
		{name: "unknown credential", credential: unknown, groups: []vault.ID{work}},
		{name: "unknown group", credential: id, groups: []vault.ID{unknown}},
		{name: "one unknown group among known ones", credential: id, groups: []vault.ID{work, unknown}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := service.SetItemGroups(testCase.credential, testCase.groups); !errors.Is(err, vault.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
			if !bytes.Equal(files.data, saved) {
				t.Fatal("the refused change was written to the vault file")
			}
			current, err := service.State()
			if err != nil {
				t.Fatal(err)
			}
			if current.Head != head.Head {
				t.Fatalf("head moved to %+v", current.Head)
			}
			if got := entryGroups(t, service, id); len(got) != 1 || got[0] != work {
				t.Fatalf("membership = %v", got)
			}
		})
	}
}

func TestGroupMethodsNeedAnOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())

	cases := []struct {
		name string
		call func() error
	}{
		{name: "Groups", call: func() error { _, err := service.Groups(); return err }},
		{name: "CreateGroup", call: func() error { _, err := service.CreateGroup("Work"); return err }},
		{name: "RenameGroup", call: func() error { return service.RenameGroup(vault.ID{1}, "Work") }},
		{name: "DeleteGroup", call: func() error { return service.DeleteGroup(vault.ID{1}) }},
		{name: "SetItemGroups", call: func() error { return service.SetItemGroups(vault.ID{1}, nil) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); !errors.Is(err, ErrNotReady) {
				t.Fatalf("error before a vault is open = %v, want ErrNotReady", err)
			}
		})
	}
}
