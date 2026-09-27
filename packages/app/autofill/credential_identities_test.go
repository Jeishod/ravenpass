package autofill

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestCredentialIdentitiesCarryNoValueAndEachFillsForItsSite(t *testing.T) {
	core, service := openVault(t, "")
	const password, setup = "never-listed-secret", "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	id := create(t, core, vault.CredentialInput{
		Label: "Mail", Websites: []string{"https://www.mail.example/login", "https://webmail.example"}, Login: "alex",
		Password: password, TOTP: setup,
	})
	create(t, core, vault.CredentialInput{Label: "Router", Password: "no-site"})
	created, err := service.CreatePasskey(newPasskeyCreation())
	if err != nil {
		t.Fatal(err)
	}
	identities, err := service.CredentialIdentities()
	if err != nil {
		t.Fatal(err)
	}
	want := []CredentialIdentity{
		{Kind: IdentityPassword, Site: "mail.example", User: "alex", Record: id},
		{Kind: IdentityCode, Site: "mail.example", User: "alex", Record: id},
		{Kind: IdentityPassword, Site: "webmail.example", User: "alex", Record: id},
		{Kind: IdentityCode, Site: "webmail.example", User: "alex", Record: id},
		{Kind: IdentityPassword, Site: passkeyRPID, User: "alex@example.com", Record: created.ID},
		{
			Kind: IdentityPasskey, Site: passkeyRPID, User: "alex@example.com", Record: created.ID,
			CredentialID: created.CredentialID, UserHandle: []byte{7, 7},
		},
	}
	if !reflect.DeepEqual(identities, want) {
		t.Fatalf("identities = %+v, want %+v", identities, want)
	}
	listed, err := json.Marshal(identities)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{password, setup, "no-site"} {
		if strings.Contains(string(listed), value) {
			t.Fatalf("the identities carry %q", value)
		}
	}
	for _, identity := range identities {
		page := Requester{Origin: "https://" + identity.Site}
		switch identity.Kind {
		case IdentityPassword:
			_, err = service.Fill(identity.Record, page)
		case IdentityCode:
			_, err = service.OneTimeCode(identity.Record, page)
		case IdentityPasskey:
			var choices []PasskeyChoice
			choices, err = service.Passkeys(identity.Site, [][]byte{identity.CredentialID})
			if err == nil && (len(choices) != 1 || choices[0].ID != identity.Record) {
				t.Fatalf("passkeys for %+v = %+v", identity, choices)
			}
		}
		if err != nil {
			t.Fatalf("%+v does not fill for its site: %v", identity, err)
		}
	}
}

func TestCredentialIdentitiesNeedAnOpenVault(t *testing.T) {
	core, service := openVault(t, "")
	create(t, core, vault.CredentialInput{Label: "Mail", Websites: []string{"https://mail.example"}, Login: "alex"})
	core.Lock()
	if _, err := service.CredentialIdentities(); !errors.Is(err, ErrLocked) {
		t.Fatalf("identities while locked: got %v, want ErrLocked", err)
	}
}
