package bitwarden

import (
	"slices"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// identity maps an identity, with its passport, licence and tax numbers as documents.
func (m *mapper) identity(source item, label string) vault.NewItem {
	values := source.Identity
	input := vault.IdentityInput{Label: label, FullName: importers.Joined(" ", values.FirstName, values.MiddleName, values.LastName)}
	if !importers.Blank(values.Email) {
		input.Emails = []string{values.Email}
	}
	if !importers.Blank(values.Phone) {
		input.Phones = []string{values.Phone}
	}
	address := vault.Address{
		Street:     importers.Joined("\n", values.Address1, values.Address2, values.Address3),
		City:       values.City,
		Region:     values.State,
		PostalCode: values.PostalCode,
		Country:    values.Country,
	}
	parts := []string{address.Street, address.City, address.Region, address.PostalCode, address.Country}
	if slices.ContainsFunc(parts, func(part string) bool { return !importers.Blank(part) }) {
		input.Addresses = []vault.Address{address}
	}
	for _, document := range []vault.Document{
		{Type: vault.DocumentPassport, Number: values.PassportNumber},
		{Type: vault.DocumentDriversLicense, Number: values.LicenseNumber},
		{Type: vault.DocumentTaxNumber, Number: values.SSN},
	} {
		if !importers.Blank(document.Number) {
			input.Documents = append(input.Documents, document)
		}
	}
	var extras importers.Extras
	extras.Add(m.labels.Title, values.Title)
	extras.Add(m.labels.Company, values.Company)
	extras.Add(m.labels.Username, values.Username)
	input.Notes = notesOf(source, &extras)
	return vault.NewItem{Identity: &input}
}

// driversLicense maps a driver's licence onto an identity holding it as a document.
func (m *mapper) driversLicense(source item, label string) vault.NewItem {
	values := source.DriversLicense
	issuer := importers.Joined(", ", values.IssuingAuthority, values.IssuingState, values.IssuingCountry)
	return m.documentIdentity(source, label,
		importers.Joined(" ", values.FirstName, values.MiddleName, values.LastName),
		values.DateOfBirth,
		heldDocument{kind: vault.DocumentDriversLicense, number: values.LicenseNumber, issuer: issuer, issued: values.IssueDate, expires: values.ExpirationDate},
		line{m.labels.LicenseClass, values.LicenseClass},
	)
}

// passport maps a passport onto an identity holding it as a document.
func (m *mapper) passport(source item, label string) vault.NewItem {
	values := source.Passport
	issuer := importers.Joined(", ", values.IssuingAuthority, values.IssuingCountry)
	return m.documentIdentity(source, label,
		importers.Joined(" ", values.GivenName, values.Surname),
		values.DateOfBirth,
		heldDocument{kind: vault.DocumentPassport, number: values.PassportNumber, issuer: issuer, issued: values.IssueDate, expires: values.ExpirationDate},
		line{m.labels.Sex, values.Sex},
		line{m.labels.BirthPlace, values.BirthPlace},
		line{m.labels.Nationality, values.Nationality},
		line{m.labels.PassportType, values.PassportType},
		line{m.labels.NationalID, values.NationalIdentificationNumber},
	)
}

// heldDocument is a licence or passport document with its values as the export writes them.
type heldDocument struct {
	kind    vault.DocumentType
	number  string
	issuer  string
	issued  string
	expires string
}

func (m *mapper) documentIdentity(source item, label, fullName, birthday string, document heldDocument, lines ...line) vault.NewItem {
	var extras importers.Extras
	input := vault.IdentityInput{Label: label, FullName: fullName}
	if date, ok := importers.IdentityDate(birthday); ok {
		input.Birthday = date
	} else {
		extras.Add(m.labels.Birthday, birthday)
	}
	if placed, ok := m.place(document, &extras); ok {
		input.Documents = []vault.Document{placed}
	}
	for _, written := range lines {
		extras.Add(written.label, written.value)
	}
	input.Notes = notesOf(source, &extras)
	return vault.NewItem{Identity: &input}
}

// place writes dates the vault refuses, including an issue date after expiry, as labelled lines.
func (m *mapper) place(document heldDocument, extras *importers.Extras) (vault.Document, bool) {
	if importers.Blank(document.number) {
		extras.Add(m.labels.Issuer, document.issuer)
		extras.Add(m.labels.IssuedOn, document.issued)
		extras.Add(m.labels.ExpiresOn, document.expires)
		return vault.Document{}, false
	}
	placed := vault.Document{Type: document.kind, Number: document.number, Issuer: document.issuer}
	issued, issuedHeld := importers.IdentityDate(document.issued)
	expires, expiresHeld := importers.IdentityDate(document.expires)
	// Both dates are YYYY-MM-DD: text order is calendar order.
	if issuedHeld && expiresHeld && issued > expires {
		issuedHeld, expiresHeld = false, false
	}
	if issuedHeld {
		placed.IssuedOn = issued
	} else {
		extras.Add(m.labels.IssuedOn, document.issued)
	}
	if expiresHeld {
		placed.ExpiresOn = expires
	} else {
		extras.Add(m.labels.ExpiresOn, document.expires)
	}
	return placed, true
}
