package authenticator

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/dortanes/ravenpass/packages/vault"
	"golang.org/x/net/publicsuffix"
)

var (
	// ErrInvalidOrigin reports an origin that is not serialized, not https (or http on localhost), or not named by a domain.
	ErrInvalidOrigin = errors.New("origin cannot use passkeys")
	// ErrInvalidRelyingParty reports a relying party ID that does not fit the origin.
	ErrInvalidRelyingParty = errors.New("relying party ID does not fit the origin")
)

const localhost = "localhost"

// CheckRelyingParty returns the rpID origin may use per WebAuthn §5.1.3 and HTML's registrable domain suffix rule; empty means the host.
func CheckRelyingParty(origin, rpID string) (string, error) {
	host, err := originHost(origin)
	if err != nil {
		return "", err
	}
	if rpID == "" {
		rpID = host
	} else if rpID, err = relyingPartyID(rpID); err != nil {
		return "", err
	}
	if rpID != host && !strings.HasSuffix(host, "."+rpID) {
		return "", ErrInvalidRelyingParty
	}
	if isPublicSuffix(rpID, host) {
		return "", ErrInvalidRelyingParty
	}
	return rpID, nil
}

// RelyingPartyID returns the ID of a request a platform checked against its origin, refusing one CheckRelyingParty refuses for every origin.
func RelyingPartyID(rpID string) (string, error) {
	id, err := relyingPartyID(rpID)
	if err != nil {
		return "", err
	}
	if isPublicSuffix(id, id) {
		return "", ErrInvalidRelyingParty
	}
	return id, nil
}

func originHost(origin string) (string, error) {
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", ErrInvalidOrigin
	}
	serialized := url.URL{Scheme: parsed.Scheme, Host: parsed.Host}
	if serialized.String() != origin {
		return "", ErrInvalidOrigin
	}
	host, ok := domain(parsed.Hostname())
	if !ok {
		return "", ErrInvalidOrigin
	}
	switch parsed.Scheme {
	case "https":
		return host, nil
	case "http":
		if isLocalhost(host) {
			return host, nil
		}
	}
	return "", ErrInvalidOrigin
}

func relyingPartyID(rpID string) (string, error) {
	parsed, err := url.Parse("https://" + rpID)
	if err != nil || parsed.Host != rpID || parsed.Port() != "" {
		return "", ErrInvalidRelyingParty
	}
	id, ok := domain(parsed.Hostname())
	if !ok {
		return "", ErrInvalidRelyingParty
	}
	return id, nil
}

// domain returns host as vault.HostASCII gives it, and false for an IP address.
func domain(host string) (string, bool) {
	if net.ParseIP(host) != nil {
		return "", false
	}
	return vault.HostASCII(host)
}

func isLocalhost(host string) bool {
	return host == localhost || strings.HasSuffix(host, "."+localhost)
}

// isPublicSuffix reports an rpID that is its own public suffix or part of the host's, per HTML's registrable domain suffix check.
func isPublicSuffix(rpID, host string) bool {
	if rpID == localhost {
		return false
	}
	own, _ := publicsuffix.PublicSuffix(rpID)
	hosts, _ := publicsuffix.PublicSuffix(host)
	return own == rpID || strings.HasSuffix(hosts, "."+rpID)
}
