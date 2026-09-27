package importers

import (
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestCardNumberDropsSpacesAndHyphens(t *testing.T) {
	tests := []struct {
		written string
		number  string
		valid   bool
	}{
		{"4111 1111-1111 1111", "4111111111111111", true},
		{"411111111111", "411111111111", true},
		{"4111111111111111111", "4111111111111111111", true},
		{"41111111111", "", false},
		{"41111111111111111111", "", false},
		{"4111.1111.1111.1111", "", false},
		{"1234-5678", "", false},
		{"", "", false},
	}
	for _, test := range tests {
		if number, valid := CardNumber(test.written); number != test.number || valid != test.valid {
			t.Errorf("CardNumber(%q) = %q, %v", test.written, number, valid)
		}
	}
}

func TestCardExpiryReadsAMonthAndAYear(t *testing.T) {
	tests := []struct {
		month, year string
		expiry      string
		valid       bool
	}{
		{"1", "27", "2027-01", true},
		{" 01 ", " 2027 ", "2027-01", true},
		{"12", "2030", "2030-12", true},
		{"", "", "", true},
		{" ", "", "", true},
		{"13", "2027", "", false},
		{"0", "2027", "", false},
		{"1", "7", "", false},
		{"1", "", "", false},
		{"", "2027", "", false},
		{"1", "1850", "", false},
		{"Jan", "2027", "", false},
		{"001", "2027", "", false},
	}
	for _, test := range tests {
		if expiry, valid := CardExpiry(test.month, test.year); expiry != test.expiry || valid != test.valid {
			t.Errorf("CardExpiry(%q, %q) = %q, %v", test.month, test.year, expiry, valid)
		}
	}
}

func TestSecurityCodeAndCardPINAreDigits(t *testing.T) {
	tests := []struct {
		read    func(string) (string, bool)
		name    string
		written string
		value   string
		valid   bool
	}{
		{SecurityCode, "SecurityCode", " 123 ", "123", true},
		{SecurityCode, "SecurityCode", "1234", "1234", true},
		{SecurityCode, "SecurityCode", "12", "", false},
		{SecurityCode, "SecurityCode", "12345", "", false},
		{SecurityCode, "SecurityCode", "12a", "", false},
		{SecurityCode, "SecurityCode", "", "", false},
		{CardPIN, "CardPIN", " 1234 ", "1234", true},
		{CardPIN, "CardPIN", "123456789012", "123456789012", true},
		{CardPIN, "CardPIN", "123", "", false},
		{CardPIN, "CardPIN", "1234567890123", "", false},
		{CardPIN, "CardPIN", "12-34", "", false},
		{CardPIN, "CardPIN", "", "", false},
	}
	for _, test := range tests {
		if value, valid := test.read(test.written); value != test.value || valid != test.valid {
			t.Errorf("%s(%q) = %q, %v", test.name, test.written, value, valid)
		}
	}
}

func TestBillingAddressReadsAFieldNamedAsAnAddress(t *testing.T) {
	long := strings.Repeat("s", vault.MaxStreetLength+1)
	tests := []struct {
		name, value string
		street      string
	}{
		{" Address ", " 1 Main St ", "1 Main St"},
		{"BILLING ADDRESS", "1 Main St", "1 Main St"},
		{"Адрес", "ул. Ленина, 1", "ул. Ленина, 1"},
		{"Платёжный адрес", "ул. Ленина, 1", "ул. Ленина, 1"},
		{"платежный адрес", "ул. Ленина, 1", "ул. Ленина, 1"},
		{"Home address", "1 Main St", ""},
		{"Address", "  ", ""},
		{"Address", long, ""},
	}
	for _, test := range tests {
		address, ok := BillingAddress(test.name, test.value)
		if ok != (test.street != "") || ok && *address != (vault.Address{Street: test.street}) {
			t.Errorf("BillingAddress(%q, %q) = %+v, %v", test.name, test.value, address, ok)
		}
	}
}

func TestIdentityDateReadsADateOrATime(t *testing.T) {
	tests := []struct {
		value string
		date  string
		held  bool
	}{
		{"2020-01-15", "2020-01-15", true},
		{" 2020-01-15 ", "2020-01-15", true},
		{"2020-01-15T00:00:00.000Z", "2020-01-15", true},
		{"2020-01-15T23:30:00-08:00", "2020-01-15", true},
		{"15/01/2020", "", false},
		{"2020-1-5", "", false},
		{"2020-02-30", "", false},
		{"1850-01-01", "", false},
		{"", "", false},
	}
	for _, test := range tests {
		if date, held := IdentityDate(test.value); date != test.date || held != test.held {
			t.Errorf("IdentityDate(%q) = %q, %v; want %q, %v", test.value, date, held, test.date, test.held)
		}
	}
}
