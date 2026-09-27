package api

import "testing"

func TestWebAddressAcceptsOnlyWebSites(t *testing.T) {
	accepted := map[string]string{
		"github.com":                  "https://github.com",
		"  https://example.com/path ": "https://example.com/path",
		"http://localhost:8080":       "http://localhost:8080",
	}
	for address, want := range accepted {
		got, err := webAddress(address)
		if err != nil || got != want {
			t.Fatalf("webAddress(%q) = %q, %v; want %q", address, got, err, want)
		}
	}
	rejected := []string{
		"",
		"   ",
		"file:///Users/me/secrets",
		"javascript:alert(1)",
		"ftp://example.com",
		"https://",
	}
	for _, address := range rejected {
		if got, err := webAddress(address); err == nil {
			t.Fatalf("webAddress(%q) = %q, want a refusal", address, got)
		}
	}
}
