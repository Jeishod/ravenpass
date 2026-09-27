package vault

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// groupedVault commits one revision per group, so the returned head sits at 1+len(names).
func groupedVault(t *testing.T, names ...string) (Created, []ID) {
	t.Helper()
	created, err := Create()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]ID, 0, len(names))
	for _, name := range names {
		pending, id, err := created.Session.PrepareCreateGroup(name)
		if err != nil {
			t.Fatalf("create group %q: %v", name, err)
		}
		if err := created.Session.Commit(pending); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return created, ids
}

func commitCredential(t *testing.T, session *Session, input CredentialInput, groups []ID) ID {
	t.Helper()
	pending, id, err := session.PrepareCreate(input, groups)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	return id
}

func sortedIDs(ids ...ID) []ID {
	sorted := append([]ID(nil), ids...)
	slices.SortFunc(sorted, func(left, right ID) int { return bytes.Compare(left[:], right[:]) })
	return sorted
}

func membershipOf(t *testing.T, session *Session, id ID) []ID {
	t.Helper()
	entries, err := session.List()
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

func TestPrepareCreateGroupTrimsAndRefusesNames(t *testing.T) {
	created, _ := groupedVault(t, "  Work  ")
	session := created.Session
	defer session.Lock()
	groups, err := session.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Name != "Work" {
		t.Fatalf("name was not trimmed: %+v", groups)
	}
	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"spaces", "   "},
		{"tab and newline", "\t\n"},
		{"the same name", "Work"},
		{"the same name in another case", "wOrK"},
		{"the same name with space around it", "  work  "},
		{"over the name limit", strings.Repeat("ü", MaxGroupNameLength+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := session.PrepareCreateGroup(test.value); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("got %v", err)
			}
		})
	}
	atLimit := strings.Repeat("ü", MaxGroupNameLength)
	pending, _, err := session.PrepareCreateGroup(atLimit)
	if err != nil {
		t.Fatalf("a name of %d characters was refused: %v", MaxGroupNameLength, err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareCreateGroupRefusesPastTheLimit(t *testing.T) {
	created, _ := groupedVault(t)
	session := created.Session
	defer session.Lock()
	for i := range MaxGroups {
		pending, _, err := session.PrepareCreateGroup(fmt.Sprintf("Group %d", i))
		if err != nil {
			t.Fatalf("group %d: %v", i, err)
		}
		if err := session.Commit(pending); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := session.PrepareCreateGroup("One too many"); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("got %v", err)
	}
}

func TestPrepareRenameGroupTrimsAndRefusesNames(t *testing.T) {
	created, groupIDs := groupedVault(t, "Work", "Home")
	session := created.Session
	defer session.Lock()
	pending, err := session.PrepareRenameGroup(groupIDs[0], "  Office  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	groups, err := session.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].Name != "Office" || groups[1].Name != "Home" {
		t.Fatalf("rename lost the trimmed name or the creation order: %+v", groups)
	}
	same, err := session.PrepareRenameGroup(groupIDs[0], "office")
	if err != nil {
		t.Fatalf("renaming a group to its own name in another case: %v", err)
	}
	if err := session.Abort(same); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		id    ID
		value string
		want  error
	}{
		{"empty name", groupIDs[0], "   ", ErrInvalidInput},
		{"over the name limit", groupIDs[0], strings.Repeat("ü", MaxGroupNameLength+1), ErrInvalidInput},
		{"another group's name", groupIDs[0], "  home  ", ErrInvalidInput},
		{"unknown group", ID{0xff}, "Anything", ErrNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := session.PrepareRenameGroup(test.id, test.value); !errors.Is(err, test.want) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestPrepareDeleteGroupKeepsRecordsSealed(t *testing.T) {
	created, groupIDs := groupedVault(t, "Work", "Home")
	session := created.Session
	defer session.Lock()
	passwords := map[ID]string{}
	both := commitCredential(t, session, CredentialInput{Label: "Both", Password: "one"}, []ID{groupIDs[0], groupIDs[1]})
	passwords[both] = "one"
	only := commitCredential(t, session, CredentialInput{Label: "Only", Password: "two"}, []ID{groupIDs[0]})
	passwords[only] = "two"
	loose := commitCredential(t, session, CredentialInput{Label: "Loose", Password: "three"}, nil)
	passwords[loose] = "three"
	before, head, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := parseContainer(before)
	if err != nil {
		t.Fatal(err)
	}
	sealed := make([][]byte, len(raw.records))
	for i := range raw.records {
		sealed[i] = encodeBox(raw.records[i])
	}
	pending, err := session.PrepareDeleteGroup(groupIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	after, nextHead, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	if nextHead.Revision != head.Revision+1 {
		t.Fatalf("revision did not advance by one: %d then %d", head.Revision, nextHead.Revision)
	}
	nextRaw, err := parseContainer(after)
	if err != nil {
		t.Fatal(err)
	}
	for i := range nextRaw.records {
		if !bytes.Equal(encodeBox(nextRaw.records[i]), sealed[i]) {
			t.Fatalf("record %d was re-encrypted", i)
		}
	}
	groups, err := session.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].ID != groupIDs[1] {
		t.Fatalf("deleted group is still reported: %+v", groups)
	}
	want := map[ID][]ID{both: {groupIDs[1]}, only: nil, loose: nil}
	for id, membership := range want {
		if got := membershipOf(t, session, id); !reflect.DeepEqual(got, membership) {
			t.Fatalf("membership after deletion: got %v, want %v", got, membership)
		}
	}
	for id, password := range passwords {
		if value := selectCredential(t, session, id); value.Password != password {
			t.Fatalf("credential changed: %+v", value.CredentialInput)
		}
	}
	if _, err := session.PrepareDeleteGroup(groupIDs[0]); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting the group twice: %v", err)
	}
}

func TestCredentialMembershipIsSortedAndDeduplicated(t *testing.T) {
	created, groupIDs := groupedVault(t, "Work", "Home", "Travel")
	session := created.Session
	defer session.Lock()
	id := commitCredential(t, session, CredentialInput{Label: "Example", Password: "secret"},
		[]ID{groupIDs[2], groupIDs[0], groupIDs[2], groupIDs[1], groupIDs[0]})
	if got := membershipOf(t, session, id); !reflect.DeepEqual(got, sortedIDs(groupIDs...)) {
		t.Fatalf("membership on create: %v", got)
	}
	before, head, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := parseContainer(before)
	if err != nil {
		t.Fatal(err)
	}
	sealed := encodeBox(raw.records[0])
	pending, err := session.PrepareSetGroups(id, []ID{groupIDs[1], groupIDs[1], groupIDs[0]})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if got := membershipOf(t, session, id); !reflect.DeepEqual(got, sortedIDs(groupIDs[0], groupIDs[1])) {
		t.Fatalf("membership after set: %v", got)
	}
	after, nextHead, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	if nextHead.Revision != head.Revision+1 {
		t.Fatalf("revision did not advance by one: %d then %d", head.Revision, nextHead.Revision)
	}
	nextRaw, err := parseContainer(after)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodeBox(nextRaw.records[0]), sealed) {
		t.Fatal("record was re-encrypted")
	}
	if value := selectCredential(t, session, id); value.Password != "secret" {
		t.Fatalf("credential changed: %+v", value.CredentialInput)
	}
	cleared, err := session.PrepareSetGroups(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(cleared); err != nil {
		t.Fatal(err)
	}
	if got := membershipOf(t, session, id); len(got) != 0 {
		t.Fatalf("membership was not cleared: %v", got)
	}
}

func TestMembershipRefusesUnknownGroups(t *testing.T) {
	created, groupIDs := groupedVault(t, "Work")
	session := created.Session
	defer session.Lock()
	id := commitCredential(t, session, CredentialInput{Label: "Example", Password: "secret"}, groupIDs)
	unknown := []ID{{0xff}}
	if _, _, err := session.PrepareCreate(CredentialInput{Label: "Other", Password: "secret"}, unknown); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create: %v", err)
	}
	if _, err := session.PrepareSetGroups(id, unknown); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set: %v", err)
	}
	if _, err := session.PrepareEdit(id, CredentialPatch{Groups: &unknown}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("edit: %v", err)
	}
	if _, err := session.PrepareSetGroups(ID{0xfe}, groupIDs); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown credential: %v", err)
	}
	if got := membershipOf(t, session, id); !reflect.DeepEqual(got, groupIDs) {
		t.Fatalf("a refused change altered the membership: %v", got)
	}
}

func TestCredentialMembershipRefusesPastTheLimit(t *testing.T) {
	names := make([]string, MaxCredentialGroups+1)
	for i := range names {
		names[i] = fmt.Sprintf("Group %d", i)
	}
	created, groupIDs := groupedVault(t, names...)
	session := created.Session
	defer session.Lock()
	if _, _, err := session.PrepareCreate(CredentialInput{Label: "Too many", Password: "secret"}, groupIDs); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("create: %v", err)
	}
	id := commitCredential(t, session, CredentialInput{Label: "At the limit", Password: "secret"}, groupIDs[:MaxCredentialGroups])
	if _, err := session.PrepareSetGroups(id, groupIDs); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("set: %v", err)
	}
	if got := membershipOf(t, session, id); len(got) != MaxCredentialGroups {
		t.Fatalf("a credential lost groups at the limit: %d", len(got))
	}
}

func TestPrepareEditReplacesMembershipOnlyWhenPatched(t *testing.T) {
	created, groupIDs := groupedVault(t, "Work", "Home")
	session := created.Session
	defer session.Lock()
	id := commitCredential(t, session, CredentialInput{Label: "Example", Password: "secret"}, []ID{groupIDs[0]})
	password := "rotated"
	pending, err := session.PrepareEdit(id, CredentialPatch{Password: &password})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if got := membershipOf(t, session, id); !reflect.DeepEqual(got, []ID{groupIDs[0]}) {
		t.Fatalf("an unpatched membership changed: %v", got)
	}
	replacement := []ID{groupIDs[1], groupIDs[0]}
	pending, err = session.PrepareEdit(id, CredentialPatch{Groups: &replacement})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if got := membershipOf(t, session, id); !reflect.DeepEqual(got, sortedIDs(groupIDs...)) {
		t.Fatalf("a patched membership was not replaced: %v", got)
	}
	empty := []ID{}
	pending, err = session.PrepareEdit(id, CredentialPatch{Groups: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if got := membershipOf(t, session, id); len(got) != 0 {
		t.Fatalf("an empty patch left groups behind: %v", got)
	}
	if value := selectCredential(t, session, id); value.Password != password || value.Label != "Example" {
		t.Fatalf("membership edits changed the credential: %+v", value.CredentialInput)
	}
}

func TestGroupsAndMembershipSurviveReopening(t *testing.T) {
	created, groupIDs := groupedVault(t, "Zulu", "alpha", "Mike")
	session := created.Session
	want := []Group{{ID: groupIDs[0], Name: "Zulu"}, {ID: groupIDs[1], Name: "alpha"}, {ID: groupIDs[2], Name: "Mike"}}
	groups, err := session.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups are not reported in creation order: %+v", groups)
	}
	member := commitCredential(t, session, CredentialInput{Label: "Member", Password: "secret"}, []ID{groupIDs[2], groupIDs[0]})
	loose := commitCredential(t, session, CredentialInput{Label: "Loose", Password: "secret"}, nil)
	container, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	session.Lock()
	reopened, err := OpenWithRecovery(container, created.RecoveryPhrase)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Lock()
	groups, err = reopened.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups changed across a reopen: %+v", groups)
	}
	if got := membershipOf(t, reopened, member); !reflect.DeepEqual(got, sortedIDs(groupIDs[0], groupIDs[2])) {
		t.Fatalf("membership changed across a reopen: %v", got)
	}
	if got := membershipOf(t, reopened, loose); len(got) != 0 {
		t.Fatalf("a loose credential gained groups: %v", got)
	}
}

func TestLockedSessionRefusesGroupOperations(t *testing.T) {
	created, groupIDs := groupedVault(t, "Work")
	session := created.Session
	id := commitCredential(t, session, CredentialInput{Label: "Example", Password: "secret"}, groupIDs)
	session.Lock()
	if _, err := session.Groups(); !errors.Is(err, ErrLocked) {
		t.Fatalf("Groups: %v", err)
	}
	if _, _, err := session.PrepareCreateGroup("Home"); !errors.Is(err, ErrLocked) {
		t.Fatalf("PrepareCreateGroup: %v", err)
	}
	if _, err := session.PrepareRenameGroup(groupIDs[0], "Home"); !errors.Is(err, ErrLocked) {
		t.Fatalf("PrepareRenameGroup: %v", err)
	}
	if _, err := session.PrepareDeleteGroup(groupIDs[0]); !errors.Is(err, ErrLocked) {
		t.Fatalf("PrepareDeleteGroup: %v", err)
	}
	if _, err := session.PrepareSetGroups(id, groupIDs); !errors.Is(err, ErrLocked) {
		t.Fatalf("PrepareSetGroups: %v", err)
	}
}
