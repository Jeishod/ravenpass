package aliasvault

import (
	"slices"
	"strings"

	"github.com/dortanes/ravenpass/packages/importers"
)

// CSV header names AliasVault writes, in any order; older exports lack the card columns.
const (
	columnName         = "ServiceName"
	columnFolder       = "FolderPath"
	columnURLs         = "ServiceUrl"
	columnUsername     = "Username"
	columnPassword     = "CurrentPassword"
	columnEmail        = "AliasEmail"
	columnTOTP         = "TwoFactorSecret"
	columnGender       = "AliasGender"
	columnFirstName    = "AliasFirstName"
	columnLastName     = "AliasLastName"
	columnNickname     = "AliasNickName"
	columnBirthdate    = "AliasBirthDate"
	columnHolder       = "CardholderName"
	columnNumber       = "CardNumber"
	columnExpiryMonth  = "CardExpiryMonth"
	columnExpiryYear   = "CardExpiryYear"
	columnSecurityCode = "CardCvv"
	columnPIN          = "CardPin"
	columnNotes        = "Notes"
)

const urlSeparator = ","

func openCSV(text []byte) (*importers.ExportFile, error) {
	export := importers.CSV(text)
	if !export.Recognised(columnName, columnPassword) {
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
		mapped.add(read.entry(), read.folders())
	})
	if err != nil {
		return importers.Export{}, err
	}
	return mapped.export, nil
}

type csvRow struct {
	importers.Row
}

// entry infers the item type from the filled cells; the CSV export writes no type column.
func (r csvRow) entry() entry {
	read := entry{
		name:     r.Cell(columnName),
		username: r.Cell(columnUsername),
		email:    r.Cell(columnEmail),
		password: r.Cell(columnPassword),
		alias: alias{
			firstName: r.Cell(columnFirstName),
			lastName:  r.Cell(columnLastName),
			gender:    r.Cell(columnGender),
			birthdate: r.Cell(columnBirthdate),
			nickname:  r.Cell(columnNickname),
		},
		card: card{
			holder:       r.Cell(columnHolder),
			number:       r.Cell(columnNumber),
			expiryMonth:  r.Cell(columnExpiryMonth),
			expiryYear:   r.Cell(columnExpiryYear),
			securityCode: r.Cell(columnSecurityCode),
			pin:          r.Cell(columnPIN),
		},
		notes: r.Cell(columnNotes),
	}
	read.addWebsites(strings.Split(r.Cell(columnURLs), urlSeparator)...)
	if secret := r.Cell(columnTOTP); !importers.Blank(secret) {
		read.codes = []oneTimeCode{{secret: secret}}
	}
	read.kind = kindOf(read)
	return read
}

func kindOf(read entry) string {
	switch {
	case filled(read.card.holder, read.card.number, read.card.expiryMonth, read.card.expiryYear, read.card.securityCode, read.card.pin):
		return typeCreditCard
	case filled(read.alias.gender, read.alias.firstName, read.alias.lastName, read.alias.nickname, birthdate(read.alias.birthdate)):
		return typeAlias
	case len(read.websites) > 0 || len(read.codes) > 0 || filled(read.username, read.password, read.email):
		return typeLogin
	}
	return typeNote
}

func filled(values ...string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return !importers.Blank(value) })
}

func (r csvRow) folders() []string {
	if path := r.Cell(columnFolder); !importers.Blank(path) {
		return []string{path}
	}
	return nil
}
