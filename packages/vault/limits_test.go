package vault

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func fieldLimits() map[string]struct {
	limit int
	build func(string) CredentialInput
} {
	return map[string]struct {
		limit int
		build func(string) CredentialInput
	}{
		"label": {MaxLabelLength, func(value string) CredentialInput {
			return CredentialInput{Label: value, Password: "secret"}
		}},
		"website": {MaxOriginLength, func(value string) CredentialInput {
			return CredentialInput{Label: "Example", Websites: []string{"https://example.com", value}, Password: "secret"}
		}},
		"login": {MaxLoginLength, func(value string) CredentialInput {
			return CredentialInput{Label: "Example", Login: value, Password: "secret"}
		}},
		"email": {MaxEmailLength, func(value string) CredentialInput {
			return CredentialInput{Label: "Example", Email: value, Password: "secret"}
		}},
		"password": {MaxPasswordLength, func(value string) CredentialInput {
			return CredentialInput{Label: "Example", Password: value}
		}},
		"notes": {MaxNotesLength, func(value string) CredentialInput {
			return CredentialInput{Label: "Example", Password: "secret", Notes: value}
		}},
	}
}

func TestEachFieldAcceptsItsLimitAndRefusesMore(t *testing.T) {
	for name, field := range fieldLimits() {
		session, _ := populatedSession(t)
		atLimit := strings.Repeat("a", field.limit)
		pending, _, err := session.PrepareCreate(field.build(atLimit), nil)
		if err != nil {
			t.Errorf("%s at its limit was refused: %v", name, err)
			continue
		}
		if err := session.Commit(pending); err != nil {
			t.Fatal(err)
		}
		if _, _, err := session.PrepareCreate(field.build(atLimit+"a"), nil); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s over its limit: got %v, want ErrInvalidInput", name, err)
		}
	}
}

func TestLimitsCountCharactersNotBytes(t *testing.T) {
	session, _ := populatedSession(t)
	// Cyrillic letters take two UTF-8 bytes each.
	label := strings.Repeat("п", MaxLabelLength)
	if _, _, err := session.PrepareCreate(CredentialInput{Label: label, Password: "secret"}, nil); err != nil {
		t.Fatalf("a label of %d Cyrillic characters was refused: %v", MaxLabelLength, err)
	}
}

func TestLabelIsRequired(t *testing.T) {
	session, id := populatedSession(t)
	for _, label := range []string{"", " ", "\t\n"} {
		if _, _, err := session.PrepareCreate(CredentialInput{Label: label, Password: "secret"}, nil); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("label %q: got %v, want ErrInvalidInput", label, err)
		}
	}
	empty := ""
	if _, err := session.PrepareEdit(id, CredentialPatch{Label: &empty}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("clearing a label: got %v, want ErrInvalidInput", err)
	}
}

func numberedWebsites(count int) []string {
	websites := make([]string, count)
	for i := range websites {
		websites[i] = fmt.Sprintf("https://site%d.example", i)
	}
	return websites
}

func TestWebsitesAcceptTheirLimitAndRefuseMore(t *testing.T) {
	session, _ := populatedSession(t)
	pending, id, err := session.PrepareCreate(CredentialInput{Label: "Example", Websites: numberedWebsites(MaxCredentialWebsites)}, nil)
	if err != nil {
		t.Fatalf("%d websites were refused: %v", MaxCredentialWebsites, err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if got := selectCredential(t, session, id).Websites; !slices.Equal(got, numberedWebsites(MaxCredentialWebsites)) {
		t.Fatalf("websites read back as %q", got)
	}
	if _, _, err := session.PrepareCreate(CredentialInput{Label: "Example", Websites: numberedWebsites(MaxCredentialWebsites + 1)}, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("%d websites: got %v, want ErrInvalidInput", MaxCredentialWebsites+1, err)
	}
	if entries, _ := session.List(); len(entries) != 2 {
		t.Fatalf("a refused credential changed the list to %d entries", len(entries))
	}
}

func TestBlankWebsiteIsRefused(t *testing.T) {
	session, _ := populatedSession(t)
	for _, blank := range []string{"", " ", "\t\n"} {
		input := CredentialInput{Label: "Example", Websites: []string{"https://example.com", blank}}
		if _, _, err := session.PrepareCreate(input, nil); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("website %q: got %v, want ErrInvalidInput", blank, err)
		}
	}
}

func TestEditRefusesWebsitesOutOfBoundsAndKeepsTheCredential(t *testing.T) {
	created, ids := vaultWith(t,
		CredentialInput{Label: "Example", Websites: []string{"https://www.example.com", "https://admin.example.com"}, Password: "secret"},
	)
	session := created.Session
	defer session.Lock()
	for name, websites := range map[string][]string{
		"too many": numberedWebsites(MaxCredentialWebsites + 1),
		"blank":    {"https://other.example", " "},
		"too long": {strings.Repeat("a", MaxOriginLength+1)},
	} {
		if _, err := session.PrepareEdit(ids[0], CredentialPatch{Websites: &websites}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v, want ErrInvalidInput", name, err)
		}
	}
	if got := selectCredential(t, session, ids[0]).Websites; !slices.Equal(got, []string{"https://www.example.com", "https://admin.example.com"}) {
		t.Fatalf("a refused edit changed the websites to %q", got)
	}
	if entry := listedEntry(t, session, ids[0]); entry.Site != "example.com" {
		t.Fatalf("a refused edit changed the site to %q", entry.Site)
	}
}

func TestEditRefusesAValueOverItsLimitAndKeepsTheCredential(t *testing.T) {
	session, id := populatedSession(t)
	tooLong := strings.Repeat("a", MaxNotesLength+1)
	if _, err := session.PrepareEdit(id, CredentialPatch{Notes: &tooLong}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("notes over the limit: got %v, want ErrInvalidInput", err)
	}
	ticket, err := session.BeginSelection(id)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := session.ReadSelected(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Label != "Example" || credential.Password != "secret" || credential.Notes != "" {
		t.Fatalf("a refused edit changed the credential: %+v", credential.CredentialInput)
	}
}
