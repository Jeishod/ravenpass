package bitwarden

import (
	"strings"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// login maps a login; a username that is an email address fills the email instead.
func (m *mapper) login(source item, label string) vault.NewItem {
	var extras importers.Extras
	input := vault.CredentialInput{Label: label, Password: source.Login.Password}
	input.Websites = importers.Websites(source.Login.uris(), m.labels.Website, &extras)
	if username := source.Login.Username; importers.IsEmail(username) {
		input.Email = username
	} else {
		input.Login = username
	}
	if setup, err := vault.NormalizeTOTP(source.Login.TOTP); err == nil {
		input.TOTP = setup
	} else {
		extras.Add(m.labels.OneTimeCode, source.Login.TOTP)
	}
	input.Notes = notesOf(source, &extras)
	input.Passkeys = m.passkeys(source.Login.Passkeys)
	return vault.NewItem{Credential: &input}
}

func (l login) uris() []string {
	var uris []string
	for _, entry := range l.URIs {
		if !importers.Blank(entry.URI) {
			uris = append(uris, strings.TrimSpace(entry.URI))
		}
	}
	return uris
}
