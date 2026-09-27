package bitwarden

import (
	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// secureNote maps a note, or a seed for a bare seed phrase; a reprompt note becomes hidden.
func (m *mapper) secureNote(source item, label string) vault.NewItem {
	if words, found := importers.PhraseIn(source.Notes); found {
		return m.seedOf(source, label, words)
	}
	content := m.noteOf(source, label)
	content.Note.Hidden = source.Reprompt == repromptPassword
	return content
}

// seedOf builds a seed whose notes are the item's custom fields; its text is the phrase.
func (m *mapper) seedOf(source item, label string, words []string) vault.NewItem {
	var extras importers.Extras
	source.Notes = ""
	seed := vault.SeedInput{Label: label, Format: vault.SeedPhrase, Words: words, Notes: notesOf(source, &extras)}
	return vault.NewItem{Seed: &seed}
}

// sshKey maps an SSH key onto a hidden note of its keys and fingerprint.
func (m *mapper) sshKey(source item, label string) vault.NewItem {
	values := source.SSHKey
	return hidden(m.noteOf(source, label,
		line{m.labels.PrivateKey, values.PrivateKey},
		line{m.labels.PublicKey, values.PublicKey},
		line{m.labels.Fingerprint, values.KeyFingerprint},
	))
}

// bankAccount maps a bank account onto a hidden note of its values.
func (m *mapper) bankAccount(source item, label string) vault.NewItem {
	values := source.BankAccount
	return hidden(m.noteOf(source, label,
		line{m.labels.BankName, values.BankName},
		line{m.labels.AccountHolder, values.NameOnAccount},
		line{m.labels.AccountType, values.AccountType},
		line{m.labels.AccountNumber, values.AccountNumber},
		line{m.labels.RoutingNumber, values.RoutingNumber},
		line{m.labels.BranchNumber, values.BranchNumber},
		line{m.labels.PIN, values.PIN},
		line{m.labels.SWIFT, values.SwiftCode},
		line{m.labels.IBAN, values.IBAN},
		line{m.labels.BankPhone, values.BankContactPhone},
	))
}

// hidden marks a note hidden, keeping its first line out of the item list.
func hidden(content vault.NewItem) vault.NewItem {
	content.Note.Hidden = true
	return content
}
