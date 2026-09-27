package bitwarden

import (
	"slices"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// networks maps the brands Bitwarden writes onto the payment networks a card holds.
var networks = map[string]vault.CardNetwork{
	"Visa":        vault.NetworkVisa,
	"Mastercard":  vault.NetworkMastercard,
	"Amex":        vault.NetworkAmericanExpress,
	"Discover":    vault.NetworkDiscover,
	"Diners Club": vault.NetworkDinersClub,
	"JCB":         vault.NetworkJCB,
	"Maestro":     vault.NetworkMaestro,
	"UnionPay":    vault.NetworkUnionPay,
	"Mir":         vault.NetworkMir,
	"Elo":         vault.NetworkElo,
	"Hipercard":   vault.NetworkHipercard,
}

// card maps a card, or a hidden note of its values when the number is not a card number.
func (m *mapper) card(source item, label string) vault.NewItem {
	values := source.Card
	number, isNumber := importers.CardNumber(values.Number)
	if !isNumber {
		return hidden(m.noteOf(source, label,
			line{m.labels.CardholderName, values.CardholderName},
			line{m.labels.CardNumber, values.Number},
			line{m.labels.Expiry, values.writtenExpiry()},
			line{m.labels.SecurityCode, values.Code},
			line{m.labels.Brand, values.Brand},
		))
	}
	var extras importers.Extras
	input := vault.CardInput{Label: label, Holder: values.CardholderName, Number: number}
	input.Billing, source.Fields = billingIn(source.Fields)
	if expiry, ok := importers.CardExpiry(values.ExpMonth, values.ExpYear); ok {
		input.Expiry = expiry
	} else {
		extras.Add(m.labels.Expiry, values.writtenExpiry())
	}
	if code, ok := importers.SecurityCode(values.Code); ok {
		input.SecurityCode = code
	} else {
		extras.Add(m.labels.SecurityCode, values.Code)
	}
	if network, known := networks[values.Brand]; known {
		input.Network = network
	} else {
		extras.Add(m.labels.Brand, values.Brand)
	}
	input.Notes = notesOf(source, &extras)
	return vault.NewItem{Card: &input}
}

// billingIn removes the first text field importers.BillingAddress reads and returns its address.
func billingIn(fields []field) (*vault.Address, []field) {
	for i, custom := range fields {
		if custom.Type == fieldBoolean || custom.Type == fieldLinked {
			continue
		}
		if address, ok := importers.BillingAddress(custom.Name, custom.Value); ok {
			return address, slices.Delete(slices.Clone(fields), i, i+1)
		}
	}
	return nil, fields
}

func (c card) writtenExpiry() string {
	return importers.Joined("/", c.ExpMonth, c.ExpYear)
}
