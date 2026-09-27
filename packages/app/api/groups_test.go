package api

import (
	"slices"
	"strings"
	"testing"
)

// unknownGroupID parses but names no group.
const unknownGroupID = "0123456789abcdef0123456789abcdef"

func createTestGroup(t *testing.T, service *Service, name string) string {
	t.Helper()
	id, err := service.CreateGroup(name)
	if err != nil {
		t.Fatalf("create group %q: %v", name, err)
	}
	return id
}

func summaryGroups(t *testing.T, service *Service, id string) []string {
	t.Helper()
	summaries, err := service.ListCredentials()
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range summaries {
		if summary.ID == id {
			return summary.Groups
		}
	}
	t.Fatalf("credential %s is not listed", id)
	return nil
}

func assertSameGroups(t *testing.T, got []string, want ...string) {
	t.Helper()
	sorted := slices.Clone(got)
	slices.Sort(sorted)
	expected := slices.Clone(want)
	slices.Sort(expected)
	if !slices.Equal(sorted, expected) {
		t.Fatalf("membership = %v, want %v", got, want)
	}
}

func TestListGroupsReportsWhatWasCreated(t *testing.T) {
	service := newReadyService(t)
	groups, err := service.ListGroups()
	if err != nil || len(groups) != 0 {
		t.Fatalf("groups of a new vault = %+v, error = %v", groups, err)
	}

	work := createTestGroup(t, service, "Work")
	personal := createTestGroup(t, service, "Personal")
	if err := service.RenameGroup(personal, "Home"); err != nil {
		t.Fatal(err)
	}

	groups, err = service.ListGroups()
	if err != nil || len(groups) != 2 {
		t.Fatalf("groups = %+v, error = %v", groups, err)
	}
	if groups[0].ID != work || groups[0].Name != "Work" {
		t.Fatalf("first group = %+v", groups[0])
	}
	if groups[1].ID != personal || groups[1].Name != "Home" {
		t.Fatalf("second group = %+v", groups[1])
	}

	if err := service.DeleteGroup(work); err != nil {
		t.Fatal(err)
	}
	groups, err = service.ListGroups()
	if err != nil || len(groups) != 1 || groups[0].ID != personal {
		t.Fatalf("groups after a delete = %+v, error = %v", groups, err)
	}
}

func TestCredentialMembershipIsReportedByListAndRead(t *testing.T) {
	service := newReadyService(t)
	work := createTestGroup(t, service, "Work")
	archive := createTestGroup(t, service, "Archive")

	id, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret"}, []string{work, archive})
	if err != nil {
		t.Fatal(err)
	}
	assertSameGroups(t, summaryGroups(t, service, id), work, archive)

	credential, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	assertSameGroups(t, credential.Groups, work, archive)

	if err := service.SetItemGroups(id, []string{archive}); err != nil {
		t.Fatal(err)
	}
	assertSameGroups(t, summaryGroups(t, service, id), archive)

	if err := service.SetItemGroups(id, nil); err != nil {
		t.Fatal(err)
	}
	if got := summaryGroups(t, service, id); len(got) != 0 {
		t.Fatalf("membership after clearing it = %v", got)
	}
	credential, err = service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(credential.Groups) != 0 {
		t.Fatalf("read membership after clearing it = %v", credential.Groups)
	}
}

func TestCreateAndUpdateCredentialSetMembership(t *testing.T) {
	service := newReadyService(t)
	work := createTestGroup(t, service, "Work")
	personal := createTestGroup(t, service, "Personal")
	archive := createTestGroup(t, service, "Archive")

	input := CredentialInput{Label: "Mail", Login: "alice", Password: "secret"}
	id, err := service.CreateCredential(input, []string{work, personal})
	if err != nil {
		t.Fatal(err)
	}
	assertSameGroups(t, summaryGroups(t, service, id), work, personal)

	input.Login = "alice@example.com"
	if err := service.UpdateCredential(id, input, []string{archive}, nil); err != nil {
		t.Fatal(err)
	}
	assertSameGroups(t, summaryGroups(t, service, id), archive)

	credential, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Login != "alice@example.com" {
		t.Fatalf("credential after the update = %+v", credential)
	}
	assertSameGroups(t, credential.Groups, archive)

	if err := service.UpdateCredential(id, input, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := summaryGroups(t, service, id); len(got) != 0 {
		t.Fatalf("membership after an update without groups = %v", got)
	}
}

func TestRefusedGroupNamesAndUnknownGroupsReportTheirOwnCodes(t *testing.T) {
	service := newReadyService(t)
	taken := createTestGroup(t, service, "Work")
	credential, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	nameCases := []struct {
		name string
		call func() error
	}{
		{name: "empty name", call: func() error { _, err := service.CreateGroup(""); return err }},
		{name: "blank name", call: func() error { _, err := service.CreateGroup("   "); return err }},
		{
			name: "name longer than the limit",
			call: func() error { _, err := service.CreateGroup(strings.Repeat("g", 65)); return err },
		},
		{
			name: "name another group already carries",
			call: func() error { _, err := service.CreateGroup("work"); return err },
		},
		{name: "rename to an empty name", call: func() error { return service.RenameGroup(taken, "") }},
	}
	for _, testCase := range nameCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); err == nil || err.Error() != failurePrefix+string(failureGroupNameInvalid) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	unknownCases := []struct {
		name string
		call func() error
	}{
		{name: "rename", call: func() error { return service.RenameGroup(unknownGroupID, "Home") }},
		{name: "delete", call: func() error { return service.DeleteGroup(unknownGroupID) }},
		{name: "rename an unreadable identifier", call: func() error { return service.RenameGroup("not a group", "Home") }},
		{name: "delete an unreadable identifier", call: func() error { return service.DeleteGroup("not a group") }},
		{
			name: "membership naming an unknown group",
			call: func() error { return service.SetItemGroups(credential, []string{unknownGroupID}) },
		},
		{
			name: "membership naming an unreadable identifier",
			call: func() error { return service.SetItemGroups(credential, []string{"not a group"}) },
		},
		{
			name: "creating a credential in an unknown group",
			call: func() error {
				_, err := service.CreateCredential(CredentialInput{Label: "Calendar", Password: "another"}, []string{unknownGroupID})
				return err
			},
		},
		{
			name: "updating a credential into an unknown group",
			call: func() error {
				return service.UpdateCredential(credential, CredentialInput{Label: "Mail", Password: "secret"}, []string{unknownGroupID}, nil)
			},
		},
		{
			name: "creating a credential in an unknown group beside a known one",
			call: func() error {
				_, err := service.CreateCredential(CredentialInput{Label: "Calendar", Password: "another"}, []string{taken, unknownGroupID})
				return err
			},
		},
	}
	for _, testCase := range unknownCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); err == nil || err.Error() != failurePrefix+string(failureGroupUnknown) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	groups, err := service.ListGroups()
	if err != nil || len(groups) != 1 || groups[0].Name != "Work" {
		t.Fatalf("refused calls changed the groups to %+v, error = %v", groups, err)
	}
	if got := summaryGroups(t, service, credential); len(got) != 0 {
		t.Fatalf("refused calls changed the membership to %v", got)
	}
}

func TestDefaultGroupFollowsTheVault(t *testing.T) {
	service := newReadyService(t)
	group, err := service.DefaultGroup()
	if err != nil || group != "" {
		t.Fatalf("default group of a new vault = %q, error = %v", group, err)
	}

	work := createTestGroup(t, service, "Work")
	if err := service.SetDefaultGroup(work); err != nil {
		t.Fatal(err)
	}
	if group, err = service.DefaultGroup(); err != nil || group != work {
		t.Fatalf("default group = %q, error = %v", group, err)
	}

	if err := service.DeleteGroup(work); err != nil {
		t.Fatal(err)
	}
	if group, err = service.DefaultGroup(); err != nil || group != "" {
		t.Fatalf("default group after its group was deleted = %q, error = %v", group, err)
	}

	for _, id := range []string{unknownGroupID, "not a group"} {
		if err := service.SetDefaultGroup(id); err == nil || err.Error() != failurePrefix+string(failureGroupUnknown) {
			t.Fatalf("default group set to %q reported %v", id, err)
		}
	}

	replacement := createTestGroup(t, service, "Home")
	if err := service.SetDefaultGroup(replacement); err != nil {
		t.Fatal(err)
	}
	if err := service.SetDefaultGroup(""); err != nil {
		t.Fatal(err)
	}
	if group, err = service.DefaultGroup(); err != nil || group != "" {
		t.Fatalf("default group after it was cleared = %q, error = %v", group, err)
	}
}
