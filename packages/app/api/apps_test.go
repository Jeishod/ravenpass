package api

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestACredentialShowsItsLinkedAppsAndAnEditDropsButNeverAddsThem(t *testing.T) {
	service := newReadyService(t)
	mail := vault.App{Package: "com.example.mail", Signer: [32]byte{0xab}}
	rotated := vault.App{Package: "com.example.mail", Signer: [32]byte{0xcd}}
	created, err := service.vault.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret", Apps: []vault.App{mail, rotated}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := created.String()
	credential, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	linked := []LinkedApp{
		{Package: "com.example.mail", Signer: "ab" + strings.Repeat("00", 31)},
		{Package: "com.example.mail", Signer: "cd" + strings.Repeat("00", 31)},
	}
	if !slices.Equal(credential.Apps, linked) {
		t.Fatalf("apps = %+v, want %+v", credential.Apps, linked)
	}

	input := credential.CredentialInput
	for _, refused := range [][]LinkedApp{
		{linked[0], {Package: "com.attacker.mail", Signer: linked[0].Signer}},
		{{Package: "com.example.mail", Signer: strings.ToUpper(linked[0].Signer)}},
		{{Package: "com.example.mail", Signer: "ab"}},
	} {
		input.Apps = refused
		assertFailure(t, service.UpdateCredential(id, input, nil, nil), failureInvalidItem)
	}
	if held, err := service.ReadCredential(id); err != nil || !slices.Equal(held.Apps, linked) {
		t.Fatalf("apps after refused edits = %+v, error = %v", held.Apps, err)
	}

	input.Apps = linked[1:]
	if err := service.UpdateCredential(id, input, nil, nil); err != nil {
		t.Fatal(err)
	}
	if held, err := service.ReadCredential(id); err != nil || !slices.Equal(held.Apps, linked[1:]) {
		t.Fatalf("apps after dropping one = %+v, error = %v", held.Apps, err)
	}
	input.Apps = nil
	if err := service.UpdateCredential(id, input, nil, nil); err != nil {
		t.Fatal(err)
	}
	held, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(held.Apps)
	if err != nil || string(encoded) != "[]" {
		t.Fatalf("no apps are sent as %s, error = %v", encoded, err)
	}
}

type installedApps map[string]string

func (apps installedApps) Name(pkg string) string { return apps[pkg] }

func TestALinkedAppIsNamedAsTheAppInstalledOnTheDevice(t *testing.T) {
	service := newReadyService(t)
	service.apps = installedApps{"com.example.mail": "Example Mail"}
	created, err := service.vault.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret", Apps: []vault.App{
		{Package: "com.example.mail", Signer: [32]byte{0xab}},
		{Package: "com.example.chat", Signer: [32]byte{0xcd}},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.ReadCredential(created.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(credential.Apps) != 2 || credential.Apps[0].Name != "Example Mail" || credential.Apps[1].Name != "" {
		t.Fatalf("apps = %+v, want the installed one named and the other not", credential.Apps)
	}
	input := credential.CredentialInput
	if err := service.UpdateCredential(created.String(), input, nil, nil); err != nil {
		t.Fatalf("an edit that sends back the named apps failed: %v", err)
	}
}

func TestANewCredentialNamesNoApp(t *testing.T) {
	service := newReadyService(t)
	_, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret", Apps: []LinkedApp{{Package: "com.example.mail", Signer: "ab" + strings.Repeat("00", 31)}}}, nil)
	assertFailure(t, err, failureInvalidItem)
	if summaries, err := service.ListCredentials(); err != nil || len(summaries) != 0 {
		t.Fatalf("summaries = %+v, error = %v", summaries, err)
	}
}
