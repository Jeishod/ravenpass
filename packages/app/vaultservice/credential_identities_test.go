package vaultservice

import (
	"errors"
	"reflect"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestCredentialIdentitiesListAPasswordPerSiteACodeBesideItAndPasskeysWithAUserHandle(t *testing.T) {
	mail, bank := vault.ID{1}, vault.ID{2}
	named := vault.PasskeyFace{CredentialID: []byte("key-1"), RPID: "mail.example", UserHandle: []byte("user-1"), UserName: "alex.l", Discoverable: true}
	unnamed := vault.PasskeyFace{CredentialID: []byte("key-2"), RPID: "mail.example", UserHandle: []byte("user-2"), Discoverable: true}
	entries := []vault.Entry{
		{
			ID: mail, Kind: vault.KindCredential, Label: "Mail", Detail: "alex", Email: "alex@mail.example",
			Sites: []string{"mail.example", "webmail.example"}, Code: vault.CodeFace{Digits: 6, Period: 30},
			Passkeys: []vault.PasskeyFace{
				named,
				unnamed,
				{CredentialID: []byte("key-3"), RPID: "mail.example", UserHandle: []byte("user-3"), UserName: "server"},
				{CredentialID: []byte("key-4"), RPID: "mail.example", UserName: "old", Discoverable: true},
			},
		},
		{ID: bank, Kind: vault.KindCredential, Label: "Bank", Email: "sam@bank.example", Sites: []string{"bank.example", "::1"}},
		{ID: vault.ID{3}, Kind: vault.KindNote, Label: "Note", Sites: []string{"note.example"}},
		{ID: vault.ID{4}, Kind: vault.KindCredential, Label: "Wi-Fi"},
	}
	want := []CredentialIdentity{
		{Kind: IdentityPassword, Site: "mail.example", User: "alex", Credential: mail},
		{Kind: IdentityCode, Site: "mail.example", User: "alex", Credential: mail},
		{Kind: IdentityPassword, Site: "webmail.example", User: "alex", Credential: mail},
		{Kind: IdentityCode, Site: "webmail.example", User: "alex", Credential: mail},
		{Kind: IdentityPasskey, Site: "mail.example", User: "alex.l", Credential: mail, CredentialID: []byte("key-1"), UserHandle: []byte("user-1")},
		{Kind: IdentityPasskey, Site: "mail.example", User: "alex", Credential: mail, CredentialID: []byte("key-2"), UserHandle: []byte("user-2")},
		{Kind: IdentityPasskey, Site: "mail.example", User: "server", Credential: mail, CredentialID: []byte("key-3"), UserHandle: []byte("user-3")},
		{Kind: IdentityPasskey, Site: "mail.example", User: "old", Credential: mail, CredentialID: []byte("key-4"), UserHandle: []byte("recorded-4")},
		{Kind: IdentityPassword, Site: "bank.example", User: "sam@bank.example", Credential: bank},
	}
	var read [][]byte
	recorded := func(credential vault.ID, credentialID []byte) []byte {
		read = append(read, credentialID)
		if credential == mail && string(credentialID) == "key-4" {
			return []byte("recorded-4")
		}
		return nil
	}
	if got := credentialIdentitiesOf(entries, recorded); !reflect.DeepEqual(got, want) {
		t.Fatalf("identities = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(read, [][]byte{[]byte("key-4")}) {
		t.Fatalf("records read for %q, want only the face without a handle", read)
	}
	if got := credentialIdentitiesOf(nil, recorded); len(got) != 0 {
		t.Fatalf("identities of no entries = %+v", got)
	}
	if got := credentialIdentitiesOf(entries, func(vault.ID, []byte) []byte { return nil }); len(got) != len(want)-1 {
		t.Fatalf("a record without a handle listed %d identities, want %d", len(got), len(want)-1)
	}
}

func TestCredentialIdentitiesAreReadFromTheOpenVaultsIndex(t *testing.T) {
	service, _ := readyVault(t)
	passkey := testPasskey(t, "example.com", "alex.l", true, 0)
	id := createTestCredential(t, service, vault.CredentialInput{
		Label: "Example", Websites: []string{"https://www.example.com/login"}, Login: "alex", Password: "never-listed",
		TOTP: standardSecret, Passkeys: []vault.Passkey{passkey},
	})
	identities, err := service.CredentialIdentities()
	if err != nil {
		t.Fatal(err)
	}
	want := []CredentialIdentity{
		{Kind: IdentityPassword, Site: "example.com", User: "alex", Credential: id},
		{Kind: IdentityCode, Site: "example.com", User: "alex", Credential: id},
		{Kind: IdentityPasskey, Site: "example.com", User: "alex.l", Credential: id, CredentialID: passkey.CredentialID, UserHandle: passkey.UserHandle},
	}
	if !reflect.DeepEqual(identities, want) {
		t.Fatalf("identities = %+v, want %+v", identities, want)
	}
	service.Lock()
	if _, err := service.CredentialIdentities(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("identities of a locked vault: got %v, want ErrNotReady", err)
	}
}
