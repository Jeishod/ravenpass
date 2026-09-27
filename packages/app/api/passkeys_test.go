package api

import (
	"encoding/base64"
	"slices"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/authenticator"
	"github.com/dortanes/ravenpass/packages/vault"
)

func testPasskey(t *testing.T, userName string) vault.Passkey {
	t.Helper()
	key, err := authenticator.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	return vault.Passkey{
		CredentialID: key.CredentialID, RPID: "example.com", UserHandle: []byte{1}, UserName: userName, UserDisplayName: "Alex",
		PrivateKey: key.PrivateKey, Discoverable: true, CreatedAt: time.Unix(1_790_000_000, 0),
	}
}

func TestTheListMarksOneTimeCodesAndPasskeys(t *testing.T) {
	service := newReadyService(t)
	if _, err := service.CreateCredential(CredentialInput{Label: "Plain", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateCredential(CredentialInput{Label: "Coded", Password: "secret", TOTP: "gezd gnbv gy3t qojq gezd gnbv gy3t qojq"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.vault.CreateCredential(vault.CredentialInput{Label: "Keyed", Passkeys: []vault.Passkey{testPasskey(t, "alex"), testPasskey(t, "sam")}}, nil); err != nil {
		t.Fatal(err)
	}
	summaries, err := service.ListCredentials()
	if err != nil {
		t.Fatal(err)
	}
	marks := make(map[string][2]any, len(summaries))
	for _, summary := range summaries {
		marks[summary.Label] = [2]any{summary.OneTimeCode, summary.Passkeys}
	}
	want := map[string][2]any{"Plain": {false, 0}, "Coded": {true, 0}, "Keyed": {false, 2}}
	for label, mark := range want {
		if marks[label] != mark {
			t.Fatalf("%s is marked %v, want %v", label, marks[label], mark)
		}
	}
}

func TestACredentialShowsItsPasskeysWithoutTheirKeys(t *testing.T) {
	service := newReadyService(t)
	plain, err := service.CreateCredential(CredentialInput{Label: "Plain", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if credential, err := service.ReadCredential(plain); err != nil || credential.Passkeys == nil || len(credential.Passkeys) != 0 {
		t.Fatalf("a credential without passkeys shows %#v, error = %v", credential.Passkeys, err)
	}
	first, second := testPasskey(t, "alex"), testPasskey(t, "sam")
	id, err := service.vault.CreateCredential(vault.CredentialInput{Label: "Example", Login: "alex", Passkeys: []vault.Passkey{first, second}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.ReadCredential(id.String())
	if err != nil {
		t.Fatal(err)
	}
	want := []PasskeyView{
		{ID: base64.RawURLEncoding.EncodeToString(first.CredentialID), Site: "example.com", Account: "alex", DisplayName: "Alex", CreatedAt: 1_790_000_000_000},
		{ID: base64.RawURLEncoding.EncodeToString(second.CredentialID), Site: "example.com", Account: "sam", DisplayName: "Alex", CreatedAt: 1_790_000_000_000},
	}
	if !slices.Equal(credential.Passkeys, want) {
		t.Fatalf("passkeys = %+v, want %+v", credential.Passkeys, want)
	}

	edit := CredentialInput{Label: "Renamed", Login: "alex", Password: "secret"}
	if err := service.UpdateCredential(id.String(), edit, nil, nil); err != nil {
		t.Fatal(err)
	}
	if credential, err := service.ReadCredential(id.String()); err != nil || !slices.Equal(credential.Passkeys, want) {
		t.Fatalf("after an edit the passkeys are %+v, error = %v", credential.Passkeys, err)
	}

	edit.Label = "Removed"
	if err := service.UpdateCredential(id.String(), edit, nil, []string{want[0].ID}); err != nil {
		t.Fatal(err)
	}
	if credential, err := service.ReadCredential(id.String()); err != nil || !slices.Equal(credential.Passkeys, want[1:]) || credential.Label != "Removed" {
		t.Fatalf("after removal = %+v, error = %v", credential, err)
	}
	edit.Label = "Refused"
	assertFailure(t, service.UpdateCredential(id.String(), edit, nil, []string{want[0].ID}), failureItemUnreadable)
	assertFailure(t, service.UpdateCredential(id.String(), edit, nil, []string{"not base64url!"}), failureItemUnreadable)
	if credential, err := service.ReadCredential(id.String()); err != nil || credential.Label != "Removed" {
		t.Fatalf("a refused removal saved %+v, error = %v", credential, err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	assertFailure(t, service.UpdateCredential(id.String(), edit, nil, []string{want[1].ID}), failureVaultLocked)
}
