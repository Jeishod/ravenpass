package importers

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestSiteRootIsTheSchemeAndHost(t *testing.T) {
	tests := map[string]string{
		"https://login.example.test/sign-in?return=home": "https://login.example.test",
		" http://example.test:8080/path ":                "http://example.test:8080",
		"androidapp://com.example":                       "androidapp://com.example",
		"example.test/path":                              "",
		"https://":                                       "",
		"https://" + strings.Repeat("a", vault.MaxOriginLength) + ".test/": "",
		"": "",
	}
	for address, want := range tests {
		if got := siteRoot(address); got != want {
			t.Errorf("siteRoot(%q) = %q, want %q", address, got, want)
		}
	}
}

func TestWebsitesPlacesAddressesOnACredential(t *testing.T) {
	long := "https://login.example.test/sign-in?return=" + strings.Repeat("a", vault.MaxOriginLength)
	many := make([]string, vault.MaxCredentialWebsites+2)
	for i := range many {
		many[i] = fmt.Sprintf("https://site%d.example", i)
	}
	tests := []struct {
		name      string
		addresses []string
		websites  []string
		notes     string
	}{
		{"trimmed in their order", []string{" https://a.example ", "", "  ", "androidapp://com.example"}, []string{"https://a.example", "androidapp://com.example"}, ""},
		{"none", nil, nil, ""},
		{"blank", []string{" "}, nil, ""},
		{"over-long keeps its site root", []string{long}, []string{"https://login.example.test"}, "Website: " + long},
		{"over-long without a site root", []string{strings.Repeat("a", vault.MaxOriginLength+1)}, nil, "Website: " + strings.Repeat("a", vault.MaxOriginLength+1)},
		{
			"beyond what a credential holds", many, many[:vault.MaxCredentialWebsites],
			fmt.Sprintf("Website: https://site%d.example\nWebsite: https://site%d.example", vault.MaxCredentialWebsites, vault.MaxCredentialWebsites+1),
		},
		{
			"over-long beyond what a credential holds", append(many[:vault.MaxCredentialWebsites:vault.MaxCredentialWebsites], long), many[:vault.MaxCredentialWebsites],
			"Website: " + long,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var extras Extras
			websites := Websites(test.addresses, "Website", &extras)
			if !reflect.DeepEqual(websites, test.websites) || extras.Notes("") != test.notes {
				t.Fatalf("Websites = %q with notes %q", websites, extras.Notes(""))
			}
		})
	}
}

func TestLoginLabelFallsBackToTheSiteThenTheFallbacks(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		websites  []string
		fallbacks []string
		want      string
	}{
		{"the name", " Example ", []string{"https://www.site.example"}, []string{"alex"}, "Example"},
		{"the site", " ", []string{"androidapp://com.example", "https://www.site.example/login"}, []string{"alex"}, "site.example"},
		{"the first fallback", "", []string{"androidapp://com.example"}, []string{" ", " alex ", "alex@example.test"}, "alex"},
		{"nothing", "", nil, []string{"", " "}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := LoginLabel(test.label, test.websites, test.fallbacks...); got != test.want {
				t.Fatalf("LoginLabel = %q, want %q", got, test.want)
			}
		})
	}
}
