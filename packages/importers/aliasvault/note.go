package aliasvault

import (
	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// note maps a note, or a seed when its notes hold only a seed phrase.
func (m *mapper) note(source entry, label string) vault.NewItem {
	var extras importers.Extras
	if words, found := importers.PhraseIn(source.notes); found {
		source.notes = ""
		seed := vault.SeedInput{Label: label, Format: vault.SeedPhrase, Words: words, Notes: notesOf(source, &extras)}
		return vault.NewItem{Seed: &seed}
	}
	note := vault.NoteInput{Label: label, Body: notesOf(source, &extras)}
	return vault.NewItem{Note: &note}
}
