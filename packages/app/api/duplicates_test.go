package api

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestADuplicateIsNamedInTheChosenLanguage(t *testing.T) {
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	copied, err := service.DuplicateItem(id)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.ReadCredential(copied)
	if err != nil || credential.Label != "Mail (копия)" || credential.Password != "secret" {
		t.Fatalf("the copy reads %q with password %q, error = %v", credential.Label, credential.Password, err)
	}
	_, err = service.DuplicateItem("not an item")
	assertFailure(t, err, failureItemUnreadable)
	_, err = service.DuplicateItem(vault.ID{0x01}.String())
	assertFailure(t, err, failureItemUnreadable)
}

func TestACopyOfALongNameFitsTheLabelLimit(t *testing.T) {
	name := strings.Repeat("Я", vault.MaxLabelLength)
	label := copyLabel("{name} (копия)", name)
	if utf8.RuneCountInString(label) != vault.MaxLabelLength || !strings.HasSuffix(label, " (копия)") {
		t.Fatalf("the copy of a long name is %d characters: %q", utf8.RuneCountInString(label), label)
	}
	if got := copyLabel("{name} (copy)", "Mail"); got != "Mail (copy)" {
		t.Fatalf("a short name: %q", got)
	}
}
