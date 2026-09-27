package bitwarden

import (
	"encoding/csv"
	"strings"

	"github.com/dortanes/ravenpass/packages/importers"
)

// Bitwarden CSV header names, in any order; organization exports write collections, not folder.
const (
	columnFolder      = "folder"
	columnCollections = "collections"
	columnFavorite    = "favorite"
	columnType        = "type"
	columnName        = "name"
	columnNotes       = "notes"
	columnFields      = "fields"
	columnReprompt    = "reprompt"
	columnURI         = "login_uri"
	columnUsername    = "login_username"
	columnPassword    = "login_password"
	columnTOTP        = "login_totp"
)

const (
	// csvNote is the type of a secure note; a row of any other type is a login.
	csvNote = "note"
	// csvSet is how the favorite and reprompt columns write a set flag.
	csvSet = "1"
	// fieldSeparator splits a line of the fields column at its last occurrence.
	fieldSeparator = ": "
)

// openCSV owns text: the returned file holds it, or it is cleared.
func openCSV(text []byte) (*importers.ExportFile, error) {
	export := importers.CSV(text)
	if !export.Recognised(columnType, columnName) {
		export.Wipe()
		return nil, importers.ErrUnrecognized
	}
	return importers.NewExportFile(importers.FormatCSV, csvText{export}, 0), nil
}

type csvText struct {
	importers.CSV
}

// Read maps each row to an item, writing unmapped values under labels.
func (t csvText) Read(labels importers.Labels) (importers.Export, error) {
	mapped := mapper{labels: labels}
	err := t.Rows(func(row importers.Row) {
		read := csvRow{row}
		mapped.add(read.item(), read.folders())
	})
	if err != nil {
		return importers.Export{}, err
	}
	return mapped.export, nil
}

type csvRow struct {
	importers.Row
}

func (r csvRow) item() item {
	source := item{
		Type:     typeLogin,
		Name:     r.Cell(columnName),
		Notes:    r.Cell(columnNotes),
		Favorite: r.Cell(columnFavorite) == csvSet,
		Fields:   csvFields(r.Cell(columnFields)),
	}
	if r.Cell(columnReprompt) == csvSet {
		source.Reprompt = repromptPassword
	}
	if strings.TrimSpace(r.Cell(columnType)) == csvNote {
		source.Type = typeSecureNote
		return source
	}
	source.Login = login{
		URIs:     csvURIs(r.Cell(columnURI)),
		Username: r.Cell(columnUsername),
		Password: r.Cell(columnPassword),
		TOTP:     r.Cell(columnTOTP),
	}
	return source
}

func (r csvRow) folders() []string {
	var names []string
	if folder := r.Cell(columnFolder); !importers.Blank(folder) {
		names = append(names, folder)
	}
	for _, name := range strings.Split(r.Cell(columnCollections), ",") {
		if !importers.Blank(name) {
			names = append(names, name)
		}
	}
	return names
}

// csvFields reads "name: value" lines; a line without a separator is an unnamed value.
func csvFields(cell string) []field {
	var fields []field
	for _, text := range strings.Split(cell, "\n") {
		text = strings.TrimSuffix(text, "\r")
		if importers.Blank(text) {
			continue
		}
		name, value := "", text
		if at := strings.LastIndex(text, fieldSeparator); at >= 0 {
			name, value = text[:at], text[at+len(fieldSeparator):]
		}
		fields = append(fields, field{Name: name, Value: value})
	}
	return fields
}

// csvURIs parses login_uri as one nested CSV record, or as a single URI when it is not one.
func csvURIs(cell string) []uri {
	if importers.Blank(cell) {
		return nil
	}
	reader := csv.NewReader(strings.NewReader(cell))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	values, err := reader.Read()
	if err != nil {
		values = []string{cell}
	}
	uris := make([]uri, len(values))
	for i, value := range values {
		uris[i] = uri{URI: value}
	}
	return uris
}
