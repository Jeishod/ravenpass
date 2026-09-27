package bitwarden

import (
	"strconv"
	"strings"
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// mapper turns Bitwarden items into an export; now stamps a passkey with no creation date.
type mapper struct {
	labels importers.Labels
	now    time.Time
	export importers.Export
}

type conversion struct {
	origin  importers.Origin
	convert func(m *mapper, source item, label string) vault.NewItem
}

var conversions = map[int]conversion{
	typeLogin:          {importers.OriginLogin, (*mapper).login},
	typeSecureNote:     {importers.OriginNote, (*mapper).secureNote},
	typeCard:           {importers.OriginCard, (*mapper).card},
	typeIdentity:       {importers.OriginIdentity, (*mapper).identity},
	typeSSHKey:         {importers.OriginSSHKey, (*mapper).sshKey},
	typeBankAccount:    {importers.OriginBankAccount, (*mapper).bankAccount},
	typeDriversLicense: {importers.OriginDriversLicense, (*mapper).driversLicense},
	typePassport:       {importers.OriginPassport, (*mapper).passport},
}

// add counts every passkey as left behind; the credential mapping subtracts those it keeps.
func (m *mapper) add(source item, folders []string) {
	m.export.Passkeys += len(source.Login.Passkeys)
	label := labelOf(source)
	kind, known := conversions[source.Type]
	switch {
	case !known:
		m.skip(label, "", importers.ReasonUnsupported)
	case label == "":
		m.skip(label, kind.origin, importers.ReasonUnnamed)
	default:
		content := kind.convert(m, source, label)
		content.Pinned = source.Favorite
		m.export.Items = append(m.export.Items, importers.Item{Content: content, Origin: kind.origin, Folders: folders})
	}
}

func (m *mapper) skip(label string, origin importers.Origin, reason importers.Reason) {
	m.export.Skipped = append(m.export.Skipped, importers.Skip{Label: label, Origin: origin, Reason: reason})
}

// labelOf is the item's name, or for a login without one its site, then its username.
func labelOf(source item) string {
	if source.Type != typeLogin {
		return strings.TrimSpace(source.Name)
	}
	return importers.LoginLabel(source.Name, source.Login.uris(), source.Login.Username)
}

// line is one value written into the notes under a label.
type line struct {
	label string
	value string
}

// noteOf is a note whose body is the item's notes followed by lines and the custom fields.
func (m *mapper) noteOf(source item, label string, lines ...line) vault.NewItem {
	var extras importers.Extras
	for _, written := range lines {
		extras.Add(written.label, written.value)
	}
	note := vault.NoteInput{Label: label, Body: notesOf(source, &extras)}
	return vault.NewItem{Note: &note}
}

// notesOf is the item's own notes followed by the extras and then its custom fields.
func notesOf(source item, extras *importers.Extras) string {
	for _, custom := range source.Fields {
		custom.writeTo(extras)
	}
	return extras.Notes(source.Notes)
}

// writeTo writes the field under its name; a linked field holds no value of its own.
func (f field) writeTo(extras *importers.Extras) {
	switch f.Type {
	case fieldLinked:
	case fieldBoolean:
		extras.Add(f.Name, strconv.FormatBool(strings.EqualFold(strings.TrimSpace(f.Value), "true")))
	default:
		extras.Add(f.Name, f.Value)
	}
}
