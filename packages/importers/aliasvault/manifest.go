package aliasvault

import (
	"cmp"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
)

// An .avux archive holds manifest.json at its root and attached files under attachments/.
const (
	archiveManifest    = "manifest.json"
	archiveAttachments = "attachments/"
)

const manifestMajor = "1"

// folderSeparator joins folder names as AliasVault writes FolderPath in CSV.
const folderSeparator = "/"

// System field keys AliasVault writes.
const (
	keyURL          = "login.url"
	keyUsername     = "login.username"
	keyEmail        = "login.email"
	keyPassword     = "login.password"
	keyFirstName    = "alias.first_name"
	keyLastName     = "alias.last_name"
	keyGender       = "alias.gender"
	keyBirthdate    = "alias.birthdate"
	keyHolder       = "card.cardholder_name"
	keyNumber       = "card.number"
	keyExpiryMonth  = "card.expiry_month"
	keyExpiryYear   = "card.expiry_year"
	keySecurityCode = "card.cvv"
	keyPIN          = "card.pin"
	keyNotes        = "notes.content"
)

func openArchive(r io.ReaderAt, size int64) (*importers.ExportFile, error) {
	document, attachments, err := importers.ReadZIP(r, size, archiveManifest, archiveAttachments)
	if err != nil {
		return nil, err
	}
	text := importers.TrimByteOrderMark(document)
	var head struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(text, &head); err != nil {
		clear(document)
		return nil, importers.Unrecognized(err)
	}
	if major, _, _ := strings.Cut(head.Version, "."); major != manifestMajor {
		clear(document)
		return nil, importers.ErrUnrecognized
	}
	return importers.NewExportFile(importers.FormatZIP, manifestText(text), attachments), nil
}

type manifest struct {
	Items            []item            `json:"items"`
	Folders          []folder          `json:"folders"`
	Tags             []tag             `json:"tags"`
	ItemTags         []itemTag         `json:"itemTags"`
	FieldDefinitions []fieldDefinition `json:"fieldDefinitions"`
}

// item is one manifest item; any string may be null, which decodes as empty.
type item struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	ItemType    string       `json:"itemType"`
	FolderID    string       `json:"folderId"`
	FieldValues []fieldValue `json:"fieldValues"`
	TOTPCodes   []totpCode   `json:"totpCodes"`
	Passkeys    []passkey    `json:"passkeys"`
}

// fieldValue names a system field by FieldKey, or a custom field by FieldDefinitionID.
type fieldValue struct {
	FieldKey          string `json:"fieldKey"`
	FieldDefinitionID string `json:"fieldDefinitionId"`
	Value             string `json:"value"`
	Weight            int    `json:"weight"`
}

type totpCode struct {
	Name      string `json:"name"`
	SecretKey string `json:"secretKey"`
}

type folder struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ParentFolderID string `json:"parentFolderId"`
}

type tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type itemTag struct {
	ItemID string `json:"itemId"`
	TagID  string `json:"tagId"`
}

type fieldDefinition struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Weight int    `json:"weight"`
}

type manifestText []byte

// Read maps the manifest's items, writing unmapped values under labels.
func (t manifestText) Read(labels importers.Labels) (importers.Export, error) {
	var written manifest
	if err := json.Unmarshal(t, &written); err != nil {
		return importers.Export{}, importers.Unrecognized(err)
	}
	folders := foldersByID(written.Folders)
	tags := tagsByItem(written.Tags, written.ItemTags)
	definitions := definitionsByID(written.FieldDefinitions)
	mapped := mapper{labels: labels, now: time.Now()}
	for _, source := range written.Items {
		var filed []string
		if path := folders.path(source.FolderID); !importers.Blank(path) {
			filed = append(filed, path)
		}
		mapped.add(source.entry(definitions), append(filed, tags[source.ID]...))
	}
	return mapped.export, nil
}

// Wipe zeroes the manifest's bytes in place.
func (t manifestText) Wipe() {
	clear(t)
}

// entry orders custom fields by definition weight, orphans after them, and unknown keys last.
func (i item) entry(definitions map[string]fieldDefinition) entry {
	read := entry{kind: i.ItemType, name: i.Name, passkeys: i.Passkeys}
	for _, code := range i.TOTPCodes {
		read.codes = append(read.codes, oneTimeCode{name: code.Name, secret: code.SecretKey})
	}
	values := slices.Clone(i.FieldValues)
	slices.SortStableFunc(values, func(a, b fieldValue) int { return cmp.Compare(a.Weight, b.Weight) })
	var custom []fieldValue
	var unknown []field
	for _, value := range values {
		switch {
		case value.FieldKey == "":
			custom = append(custom, value)
		case !read.set(value.FieldKey, value.Value):
			unknown = append(unknown, field{label: value.FieldKey, value: value.Value})
		}
	}
	slices.SortStableFunc(custom, func(a, b fieldValue) int {
		aMissing, aWeight := definitionOrder(definitions, a)
		bMissing, bWeight := definitionOrder(definitions, b)
		return cmp.Or(cmp.Compare(aMissing, bMissing), cmp.Compare(aWeight, bWeight))
	})
	for _, value := range custom {
		read.fields = append(read.fields, field{label: definitions[value.FieldDefinitionID].Label, value: value.Value})
	}
	read.fields = append(read.fields, unknown...)
	return read
}

// definitionOrder sorts a value whose definition is missing after every defined one.
func definitionOrder(definitions map[string]fieldDefinition, value fieldValue) (int, int) {
	definition, found := definitions[value.FieldDefinitionID]
	if !found {
		return 1, 0
	}
	return 0, definition.Weight
}

// set stores a system field value and reports whether key is known.
func (e *entry) set(key, value string) bool {
	switch key {
	case keyURL:
		e.addWebsites(value)
	case keyUsername:
		e.username = value
	case keyEmail:
		e.email = value
	case keyPassword:
		e.password = value
	case keyFirstName:
		e.alias.firstName = value
	case keyLastName:
		e.alias.lastName = value
	case keyGender:
		e.alias.gender = value
	case keyBirthdate:
		e.alias.birthdate = value
	case keyHolder:
		e.card.holder = value
	case keyNumber:
		e.card.number = value
	case keyExpiryMonth:
		e.card.expiryMonth = value
	case keyExpiryYear:
		e.card.expiryYear = value
	case keySecurityCode:
		e.card.securityCode = value
	case keyPIN:
		e.card.pin = value
	case keyNotes:
		e.notes = value
	default:
		return false
	}
	return true
}

// folderTree maps folder IDs to folders; the empty ID means no folder and is never a key.
type folderTree map[string]folder

func foldersByID(list []folder) folderTree {
	byID := make(folderTree, len(list))
	for _, written := range list {
		if written.ID != "" {
			byID[written.ID] = written
		}
	}
	return byID
}

// path joins folder names from the top down; a parent cycle ends the path.
func (f folderTree) path(id string) string {
	var names []string
	passed := make(map[string]bool)
	for current, found := f[id]; found && !passed[current.ID]; current, found = f[current.ParentFolderID] {
		passed[current.ID] = true
		names = append(names, current.Name)
	}
	slices.Reverse(names)
	return strings.Join(names, folderSeparator)
}

// tagsByItem maps item IDs to tag names in link order, skipping unnamed tags.
func tagsByItem(tags []tag, links []itemTag) map[string][]string {
	names := make(map[string]string, len(tags))
	for _, written := range tags {
		if written.ID != "" {
			names[written.ID] = written.Name
		}
	}
	filed := make(map[string][]string)
	for _, link := range links {
		if name := names[link.TagID]; !importers.Blank(name) {
			filed[link.ItemID] = append(filed[link.ItemID], name)
		}
	}
	return filed
}

func definitionsByID(list []fieldDefinition) map[string]fieldDefinition {
	byID := make(map[string]fieldDefinition, len(list))
	for _, definition := range list {
		if definition.ID != "" {
			byID[definition.ID] = definition
		}
	}
	return byID
}
