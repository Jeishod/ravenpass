package aliasvault

import (
	"strings"
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// birthdateLayouts are the date forms AliasVault writes: ISO in the manifest, US in CSV.
var birthdateLayouts = []string{time.DateOnly, time.RFC3339, "2006-01-02T15:04:05", "01/02/2006 15:04:05", "01/02/2006"}

// credential maps a login or an alias; a bare email username moves to an empty email field.
func (m *mapper) credential(source entry, label string) vault.NewItem {
	var extras importers.Extras
	input := vault.CredentialInput{Label: label, Login: source.username, Email: source.email, Password: source.password}
	input.Websites = importers.Websites(source.websites, m.labels.Website, &extras)
	if importers.Blank(source.email) && importers.IsEmail(source.username) {
		input.Login, input.Email = "", source.username
	}
	input.TOTP = m.oneTimeCodes(source.codes, &extras)
	m.writeAlias(source.alias, &extras)
	input.Notes = notesOf(source, &extras)
	account := input.Login
	if importers.Blank(account) {
		account = input.Email
	}
	input.Passkeys = m.passkeys(source.passkeys, strings.TrimSpace(account))
	return vault.NewItem{Credential: &input}
}

// oneTimeCodes returns the first valid TOTP setup and writes every other code as a labelled line.
func (m *mapper) oneTimeCodes(codes []oneTimeCode, extras *importers.Extras) string {
	setup := ""
	for _, code := range codes {
		normalized, err := vault.NormalizeTOTP(code.secret)
		// vault.NormalizeTOTP returns "" without error for a blank secret, leaving setup open.
		if setup == "" && err == nil {
			setup = normalized
			continue
		}
		label := m.labels.OneTimeCode
		if !importers.Blank(code.name) {
			label += " (" + strings.TrimSpace(code.name) + ")"
		}
		extras.Add(label, code.secret)
	}
	return setup
}

func (m *mapper) writeAlias(values alias, extras *importers.Extras) {
	extras.Add(m.labels.Name, importers.Joined(" ", values.firstName, values.lastName))
	extras.Add(m.labels.Nickname, values.nickname)
	extras.Add(m.labels.Gender, values.gender)
	extras.Add(m.labels.Birthday, birthdate(values.birthdate))
}

// birthdate formats a date of birth as YYYY-MM-DD; AliasVault writes year 1 for no date.
func birthdate(written string) string {
	value := strings.TrimSpace(written)
	for _, layout := range birthdateLayouts {
		moment, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		if moment.Year() == 1 {
			return ""
		}
		return moment.Format(time.DateOnly)
	}
	return written
}
