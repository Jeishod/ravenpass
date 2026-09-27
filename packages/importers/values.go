package importers

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dortanes/ravenpass/packages/vault"
)

var numberSeparators = strings.NewReplacer(" ", "", "-", "")

// cardIssuerDigits is the length of an issuer identification number (ISO/IEC 7812-1:2017).
const cardIssuerDigits = 8

// cardIssuer is the issuer identification number of a card number: its first cardIssuerDigits digits.
func cardIssuer(number string) string {
	return number[:min(len(number), cardIssuerDigits)]
}

// CardNumber returns a card number without spaces and hyphens, false unless its digit count is one the vault accepts.
func CardNumber(written string) (string, bool) {
	number := numberSeparators.Replace(written)
	if !vault.Digits(number, vault.MinCardNumberLength, vault.MaxCardNumberLength) {
		return "", false
	}
	return number, true
}

// CardExpiry returns YYYY-MM from a month and a two- or four-digit year, empty for both blank, false for an expiry the vault refuses.
func CardExpiry(month, year string) (string, bool) {
	month, year = strings.TrimSpace(month), strings.TrimSpace(year)
	if month == "" && year == "" {
		return "", true
	}
	if !vault.Digits(month, 1, 2) || !vault.Digits(year, 2, 2) && !vault.Digits(year, 4, 4) {
		return "", false
	}
	number, err := strconv.Atoi(month)
	if err != nil || number < 1 || number > 12 {
		return "", false
	}
	if len(year) == 2 {
		year = "20" + year
	}
	expiry := fmt.Sprintf("%s-%02d", year, number)
	if !vault.ValidExpiry(expiry) {
		return "", false
	}
	return expiry, true
}

// SecurityCode returns a card's security code trimmed, false unless its digit count is one the vault accepts.
func SecurityCode(written string) (string, bool) {
	code := strings.TrimSpace(written)
	if !vault.Digits(code, vault.MinSecurityCodeLength, vault.MaxSecurityCodeLength) {
		return "", false
	}
	return code, true
}

// CardPIN returns a card's PIN trimmed, false unless its digit count is one the vault accepts.
func CardPIN(written string) (string, bool) {
	pin := strings.TrimSpace(written)
	if !vault.Digits(pin, vault.MinCardPINLength, vault.MaxCardPINLength) {
		return "", false
	}
	return pin, true
}

// billingNames are custom field names, in lower case, that exports use for a card's billing address.
var billingNames = []string{"address", "billing address", "адрес", "платежный адрес", "платёжный адрес"}

// BillingAddress reads a card's custom field named as a billing address as its street.
func BillingAddress(name, value string) (*vault.Address, bool) {
	street := strings.TrimSpace(value)
	if !slices.Contains(billingNames, strings.ToLower(strings.TrimSpace(name))) || street == "" || utf8.RuneCountInString(street) > vault.MaxStreetLength {
		return nil, false
	}
	return &vault.Address{Street: street}, true
}

// IdentityDate reads YYYY-MM-DD, or the date part of an RFC 3339 time, false for a date an identity cannot hold.
func IdentityDate(value string) (string, bool) {
	date := strings.TrimSpace(value)
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		moment, err := time.Parse(time.RFC3339, date)
		if err != nil {
			return "", false
		}
		date = moment.Format(time.DateOnly)
	}
	if !vault.ValidDate(date) {
		return "", false
	}
	return date, true
}
