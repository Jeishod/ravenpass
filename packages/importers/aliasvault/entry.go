package aliasvault

import (
	"strings"

	"github.com/dortanes/ravenpass/packages/importers"
)

// AliasVault item types.
const (
	typeLogin      = "Login"
	typeAlias      = "Alias"
	typeCreditCard = "CreditCard"
	typeNote       = "Note"
)

// entry is one AliasVault item, filled alike from the manifest and from CSV.
type entry struct {
	kind     string
	name     string
	websites []string // not blank, trimmed, in their order
	username string
	email    string
	password string
	codes    []oneTimeCode
	alias    alias
	card     card
	notes    string
	fields   []field   // custom fields, then values under a key this reader does not know
	passkeys []passkey // a CSV export carries none
}

type oneTimeCode struct {
	name   string
	secret string
}

// alias is the identity of an alias item; only older CSV exports carry a nickname.
type alias struct {
	firstName string
	lastName  string
	gender    string
	birthdate string
	nickname  string
}

type card struct {
	holder       string
	number       string
	expiryMonth  string
	expiryYear   string
	securityCode string
	pin          string
}

// field is a value written into the notes under its label, or alone when it has none.
type field struct {
	label string
	value string
}

func (e *entry) addWebsites(addresses ...string) {
	for _, address := range addresses {
		if !importers.Blank(address) {
			e.websites = append(e.websites, strings.TrimSpace(address))
		}
	}
}

// label is the item name; a login or alias without one falls back to site, username, then email.
func (e entry) label() string {
	if e.kind != typeLogin && e.kind != typeAlias {
		return strings.TrimSpace(e.name)
	}
	return importers.LoginLabel(e.name, e.websites, e.username, e.email)
}
