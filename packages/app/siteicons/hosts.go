package siteicons

import (
	"net/netip"
	"strings"
)

// neverPublic reports localhost names (RFC 6761) and non-public IP literals; names resolving to such addresses are refused when the fetcher dials.
func neverPublic(site string) bool {
	if site == "localhost" || strings.HasSuffix(site, ".localhost") {
		return true
	}
	address, err := netip.ParseAddr(site)
	if err != nil {
		return false
	}
	address = address.Unmap()
	return address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() || address.IsUnspecified()
}
