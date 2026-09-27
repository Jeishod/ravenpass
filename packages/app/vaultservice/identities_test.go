package vaultservice

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func testIdentity() vault.IdentityInput {
	return vault.IdentityInput{
		Label:     "Alex",
		FullName:  "Alex Example",
		Birthday:  "1990-04-17",
		Emails:    []string{"alex@example.com"},
		Phones:    []string{"+1 555 0100"},
		Addresses: []vault.Address{{Street: "1 Example Street", City: "Springfield"}},
		Documents: []vault.Document{{Type: vault.DocumentPassport, Number: "X1234567", ExpiresOn: "2030-01-14"}},
		Notes:     "kept at home",
	}
}

func readTestIdentity(t *testing.T, service *Service, id vault.ID) vault.Identity {
	t.Helper()
	selection, err := service.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := service.ReadSelectedIdentity(selection)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func TestIdentityWritesRoundTripAcrossLock(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	input := testIdentity()
	id, err := service.CreateIdentity(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{input.FullName, input.Phones[0], input.Documents[0].Number, input.Notes} {
		if bytes.Contains(files.data, []byte(value)) {
			t.Fatalf("%q appeared in the vault file", value)
		}
	}
	identity := readTestIdentity(t, service, id)
	stored := identity.Addresses[0].ID
	identity.Addresses[0].ID = vault.ID{}
	if stored == (vault.ID{}) || !reflect.DeepEqual(identity.IdentityInput, input) {
		t.Fatalf("identity = %+v", identity.IdentityInput)
	}
	replacement := vault.IdentityInput{Label: "Alex abroad", Phones: []string{"+44 20 7946 0000"}}
	if err := service.EditIdentity(id, replacement, nil); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	if identity := readTestIdentity(t, service, id); !reflect.DeepEqual(identity.IdentityInput, replacement) {
		t.Fatalf("identity after a lock = %+v", identity.IdentityInput)
	}
	if err := service.DeleteItem(id); err != nil {
		t.Fatal(err)
	}
	entries, err := service.List()
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries after delete = %+v, error = %v", entries, err)
	}
}

func TestItemMethodsActOnBothKinds(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	createTestVault(t, service)
	work := createTestGroup(t, service, "Work")
	credential, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := service.CreateIdentity(testIdentity(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []vault.ID{credential, identity} {
		if err := service.SetItemGroups(id, []vault.ID{work}); err != nil {
			t.Fatal(err)
		}
		if got := entryGroups(t, service, id); len(got) != 1 || got[0] != work {
			t.Fatalf("membership of %s = %v", id, got)
		}
		if err := service.SetPinned(id, true); err != nil {
			t.Fatal(err)
		}
		if err := service.MarkUsed(id); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := service.Usage()
	if err != nil || usage[credential] == 0 || usage[identity] == 0 {
		t.Fatalf("usage = %v, error = %v", usage, err)
	}
	for _, id := range []vault.ID{credential, identity} {
		if err := service.DeleteItem(id); err != nil {
			t.Fatal(err)
		}
	}
	if entries, err := service.List(); err != nil || len(entries) != 0 {
		t.Fatalf("entries after deleting both = %+v, error = %v", entries, err)
	}
}

func TestIdentityWritesRefuseACredential(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	credential, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	saved := bytes.Clone(files.data)
	if err := service.EditIdentity(credential, testIdentity(), nil); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("identity edit of a credential = %v", err)
	}
	selection, err := service.Select(credential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadSelectedIdentity(selection); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("identity read of a credential = %v", err)
	}
	if !bytes.Equal(files.data, saved) {
		t.Fatal("a refused identity write reached the vault file")
	}
}

func TestIdentityMethodsNeedAnOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	cases := []struct {
		name string
		call func() error
	}{
		{name: "ReadSelectedIdentity", call: func() error { _, err := service.ReadSelectedIdentity(vault.Selection{}); return err }},
		{name: "CreateIdentity", call: func() error { _, err := service.CreateIdentity(testIdentity(), nil); return err }},
		{name: "EditIdentity", call: func() error { return service.EditIdentity(vault.ID{1}, testIdentity(), nil) }},
		{name: "DeleteItem", call: func() error { return service.DeleteItem(vault.ID{1}) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); !errors.Is(err, ErrNotReady) {
				t.Fatalf("error before a vault is open = %v, want ErrNotReady", err)
			}
		})
	}
}
