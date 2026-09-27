package aliasvault

import (
	"slices"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// card maps a credit card, or a hidden note of its values when the number is not a card number.
func (m *mapper) card(source entry, label string) vault.NewItem {
	values := source.card
	var extras importers.Extras
	number, isNumber := importers.CardNumber(values.number)
	if !isNumber {
		extras.Add(m.labels.CardholderName, values.holder)
		extras.Add(m.labels.CardNumber, values.number)
		extras.Add(m.labels.Expiry, values.writtenExpiry())
		extras.Add(m.labels.SecurityCode, values.securityCode)
		extras.Add(m.labels.PIN, values.pin)
		note := vault.NoteInput{Label: label, Body: notesOf(source, &extras), Hidden: true}
		return vault.NewItem{Note: &note}
	}
	input := vault.CardInput{Label: label, Holder: values.holder, Number: number}
	input.Billing, source.fields = billingIn(source.fields)
	if expiry, ok := importers.CardExpiry(values.expiryMonth, values.expiryYear); ok {
		input.Expiry = expiry
	} else {
		extras.Add(m.labels.Expiry, values.writtenExpiry())
	}
	if code, ok := importers.SecurityCode(values.securityCode); ok {
		input.SecurityCode = code
	} else {
		extras.Add(m.labels.SecurityCode, values.securityCode)
	}
	if pin, ok := importers.CardPIN(values.pin); ok {
		input.PIN = pin
	} else {
		extras.Add(m.labels.PIN, values.pin)
	}
	input.Notes = notesOf(source, &extras)
	return vault.NewItem{Card: &input}
}

// billingIn removes the first field importers.BillingAddress reads and returns it as an address.
func billingIn(fields []field) (*vault.Address, []field) {
	for i, written := range fields {
		if address, ok := importers.BillingAddress(written.label, written.value); ok {
			return address, slices.Delete(slices.Clone(fields), i, i+1)
		}
	}
	return nil, fields
}

func (c card) writtenExpiry() string {
	return importers.Joined("/", c.expiryMonth, c.expiryYear)
}
