package importers

import (
	"net/mail"
	"strings"
)

// Blank reports whether value holds nothing but white space.
func Blank(value string) bool {
	return strings.TrimSpace(value) == ""
}

// Joined joins the parts that are not blank, each trimmed, with separator.
func Joined(separator string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if !Blank(part) {
			kept = append(kept, strings.TrimSpace(part))
		}
	}
	return strings.Join(kept, separator)
}

// IsEmail reports whether value is an email address alone, with no display name around it.
func IsEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}
