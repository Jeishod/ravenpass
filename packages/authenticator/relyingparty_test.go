package authenticator

import (
	"errors"
	"testing"
)

func TestARelyingPartyIDIsAcceptedOnlyWhenItFitsTheOrigin(t *testing.T) {
	for _, test := range []struct {
		name   string
		origin string
		rpID   string
		want   string
		err    error
	}{
		{name: "equal to the host", origin: "https://example.com", rpID: "example.com", want: "example.com"},
		{name: "parent of a subdomain origin", origin: "https://login.example.com", rpID: "example.com", want: "example.com"},
		{name: "empty, taking the host", origin: "https://login.example.com", want: "login.example.com"},
		{name: "a subdomain of the host", origin: "https://example.com", rpID: "login.example.com", err: ErrInvalidRelyingParty},
		{name: "another domain", origin: "https://example.com", rpID: "example.org", err: ErrInvalidRelyingParty},
		{name: "a suffix without a label boundary", origin: "https://example.com", rpID: "ample.com", err: ErrInvalidRelyingParty},
		{name: "a top-level public suffix", origin: "https://example.com", rpID: "com", err: ErrInvalidRelyingParty},
		{name: "a private public suffix", origin: "https://alex.github.io", rpID: "github.io", err: ErrInvalidRelyingParty},
		{name: "a host that is itself a public suffix", origin: "https://github.io", err: ErrInvalidRelyingParty},
		{name: "a sibling on a shared public suffix", origin: "https://alex.github.io", rpID: "sam.github.io", err: ErrInvalidRelyingParty},
		{name: "a subdomain on a shared public suffix", origin: "https://alex.github.io", want: "alex.github.io"},
		{name: "inside a wildcard public suffix", origin: "https://shop.ward.kawasaki.jp", rpID: "kawasaki.jp", err: ErrInvalidRelyingParty},
		{name: "localhost over http with a port", origin: "http://localhost:8080", rpID: "localhost", want: "localhost"},
		{name: "localhost by default", origin: "http://localhost:8080", want: "localhost"},
		{name: "a subdomain of localhost", origin: "http://app.localhost:3000", rpID: "localhost", want: "localhost"},
		{name: "localhost over https", origin: "https://localhost", want: "localhost"},
		{name: "http on a public host", origin: "http://example.com", rpID: "example.com", err: ErrInvalidOrigin},
		{name: "an origin port", origin: "https://example.com:8443", rpID: "example.com", want: "example.com"},
		{name: "an ID with a port", origin: "https://example.com:8443", rpID: "example.com:8443", err: ErrInvalidRelyingParty},
		{name: "upper case", origin: "https://Login.EXAMPLE.com", rpID: "Example.COM", want: "example.com"},
		{name: "trailing dots", origin: "https://example.com.", rpID: "example.com.", want: "example.com"},
		{name: "an internationalized ID", origin: "https://xn--bcher-kva.de", rpID: "bücher.de", want: "xn--bcher-kva.de"},
		{name: "an IPv4 origin", origin: "https://127.0.0.1", err: ErrInvalidOrigin},
		{name: "an IPv6 origin", origin: "https://[::1]:8443", err: ErrInvalidOrigin},
		{name: "an IP address ID", origin: "https://example.com", rpID: "127.0.0.1", err: ErrInvalidRelyingParty},
		{name: "an ID with a path", origin: "https://example.com", rpID: "example.com/sign-in", err: ErrInvalidRelyingParty},
		{name: "an ID with user info", origin: "https://example.com", rpID: "alex@example.com", err: ErrInvalidRelyingParty},
		{name: "an origin with a path", origin: "https://example.com/sign-in", err: ErrInvalidOrigin},
		{name: "an origin with a slash", origin: "https://example.com/", err: ErrInvalidOrigin},
		{name: "an origin with a query", origin: "https://example.com?next=1", err: ErrInvalidOrigin},
		{name: "an origin with a fragment", origin: "https://example.com#top", err: ErrInvalidOrigin},
		{name: "an origin with user info", origin: "https://alex@example.com", err: ErrInvalidOrigin},
		{name: "an origin without a host", origin: "https://", err: ErrInvalidOrigin},
		{name: "an origin without a scheme", origin: "example.com", err: ErrInvalidOrigin},
		{name: "an extension origin", origin: "chrome-extension://abcdefghijklmnop", err: ErrInvalidOrigin},
		{name: "an ftp origin", origin: "ftp://example.com", err: ErrInvalidOrigin},
		{name: "an origin with a space", origin: "https://exa mple.com", err: ErrInvalidOrigin},
		{name: "an empty origin", err: ErrInvalidOrigin},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := CheckRelyingParty(test.origin, test.rpID)
			if !errors.Is(err, test.err) {
				t.Fatalf("CheckRelyingParty(%q, %q): got error %v, want %v", test.origin, test.rpID, err, test.err)
			}
			if got != test.want {
				t.Fatalf("CheckRelyingParty(%q, %q) = %q, want %q", test.origin, test.rpID, got, test.want)
			}
		})
	}
}

func TestAPlatformsRelyingPartyIDIsTakenInLowerCaseUnlessNoOriginCouldUseIt(t *testing.T) {
	for _, test := range []struct {
		name string
		rpID string
		want string
		err  error
	}{
		{name: "a registrable domain", rpID: "example.com", want: "example.com"},
		{name: "a subdomain", rpID: "login.example.com", want: "login.example.com"},
		{name: "upper case", rpID: "Example.COM", want: "example.com"},
		{name: "a trailing dot", rpID: "example.com.", want: "example.com"},
		{name: "an internationalized ID", rpID: "bücher.de", want: "xn--bcher-kva.de"},
		{name: "localhost", rpID: "localhost", want: "localhost"},
		{name: "a subdomain on a shared public suffix", rpID: "alex.github.io", want: "alex.github.io"},
		{name: "empty", err: ErrInvalidRelyingParty},
		{name: "a top-level public suffix", rpID: "com", err: ErrInvalidRelyingParty},
		{name: "a private public suffix", rpID: "github.io", err: ErrInvalidRelyingParty},
		{name: "a wildcard public suffix", rpID: "ward.kawasaki.jp", err: ErrInvalidRelyingParty},
		{name: "an IP address", rpID: "127.0.0.1", err: ErrInvalidRelyingParty},
		{name: "a port", rpID: "example.com:443", err: ErrInvalidRelyingParty},
		{name: "a scheme", rpID: "https://example.com", err: ErrInvalidRelyingParty},
		{name: "a path", rpID: "example.com/sign-in", err: ErrInvalidRelyingParty},
		{name: "user info", rpID: "alex@example.com", err: ErrInvalidRelyingParty},
		{name: "a space", rpID: "exa mple.com", err: ErrInvalidRelyingParty},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := RelyingPartyID(test.rpID)
			if !errors.Is(err, test.err) || got != test.want {
				t.Fatalf("RelyingPartyID(%q) = %q, %v; want %q, %v", test.rpID, got, err, test.want, test.err)
			}
		})
	}
}
