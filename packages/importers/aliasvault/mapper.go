package aliasvault

import (
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// mapper turns AliasVault items into an export; now stamps every passkey it reads.
type mapper struct {
	labels importers.Labels
	now    time.Time
	export importers.Export
}

type conversion struct {
	origin  importers.Origin
	convert func(m *mapper, source entry, label string) vault.NewItem
}

var conversions = map[string]conversion{
	typeLogin:      {importers.OriginLogin, (*mapper).credential},
	typeAlias:      {importers.OriginAlias, (*mapper).credential},
	typeCreditCard: {importers.OriginCard, (*mapper).card},
	typeNote:       {importers.OriginNote, (*mapper).note},
}

// add counts every passkey as left behind; the credential mapping subtracts those it keeps.
func (m *mapper) add(source entry, folders []string) {
	m.export.Passkeys += len(source.passkeys)
	label := source.label()
	kind, known := conversions[source.kind]
	switch {
	case !known:
		m.skip(label, "", importers.ReasonUnsupported)
	case label == "":
		m.skip(label, kind.origin, importers.ReasonUnnamed)
	default:
		m.export.Items = append(m.export.Items, importers.Item{Content: kind.convert(m, source, label), Origin: kind.origin, Folders: folders})
	}
}

func (m *mapper) skip(label string, origin importers.Origin, reason importers.Reason) {
	m.export.Skipped = append(m.export.Skipped, importers.Skip{Label: label, Origin: origin, Reason: reason})
}

// notesOf is the item's own notes followed by the extras and then its fields.
func notesOf(source entry, extras *importers.Extras) string {
	for _, written := range source.fields {
		extras.Add(written.label, written.value)
	}
	return extras.Notes(source.notes)
}
