package importers

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/dortanes/ravenpass/packages/vault"
)

// siteRoot is the scheme and host of a web address, or empty when it names none the vault holds.
func siteRoot(address string) string {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	root := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
	if utf8.RuneCountInString(root) > vault.MaxOriginLength {
		return ""
	}
	return root
}

// Websites returns the addresses as a credential's websites; one overlong or past the limit goes to extras, an overlong one leaving its site root.
func Websites(addresses []string, label string, extras *Extras) []string {
	var websites []string
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		website := address
		if utf8.RuneCountInString(address) > vault.MaxOriginLength {
			extras.Add(label, address)
			website = siteRoot(address)
		}
		switch {
		case website == "":
		case len(websites) < vault.MaxCredentialWebsites:
			websites = append(websites, website)
		case website == address:
			extras.Add(label, address)
		}
	}
	return websites
}

// LoginLabel is the label of a login: its name, else the site of its first website that names one, else the first non-blank fallback.
func LoginLabel(name string, websites []string, fallbacks ...string) string {
	if label := strings.TrimSpace(name); label != "" {
		return label
	}
	if site := (vault.CredentialInput{Websites: websites}).Site(); site != "" {
		return site
	}
	for _, fallback := range fallbacks {
		if label := strings.TrimSpace(fallback); label != "" {
			return label
		}
	}
	return ""
}
