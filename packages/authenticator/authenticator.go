// Package authenticator is a stateless WebAuthn authenticator for ES256 passkeys in PKCS #8.
package authenticator

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
)

// AAGUID identifies Ravenpass as the authenticator: 877e374e-cf61-48cf-9902-8f2e792a0946.
var AAGUID = [16]byte{0x87, 0x7e, 0x37, 0x4e, 0xcf, 0x61, 0x48, 0xcf, 0x99, 0x02, 0x8f, 0x2e, 0x79, 0x2a, 0x09, 0x46}

// AlgorithmES256 is the COSE algorithm identifier of ECDSA over P-256 with SHA-256.
const AlgorithmES256 = -7

// Accepts reports whether a relying party that accepts the COSE algorithms accepts AlgorithmES256; none means the WebAuthn default, which includes it.
func Accepts(algorithms []int) bool {
	return len(algorithms) == 0 || slices.Contains(algorithms, AlgorithmES256)
}

// Authenticator data flags, WebAuthn §6.1.
const (
	flagUserPresent    byte = 0x01
	flagUserVerified   byte = 0x04
	flagBackupEligible byte = 0x08
	flagBackupState    byte = 0x10
	flagAttestedData   byte = 0x40
)

// Profile is the client a response goes through and decides its backup state; backup eligibility, fixed at creation, is always set.
type Profile int

const (
	// BrowserExtension answers through the browser extension, backup state clear.
	BrowserExtension Profile = iota
	// PlatformProvider answers through an OS credential provider, backup state set; macOS 27 fails registration without it (AuthorizationError 1000, Apple forums thread 745605).
	PlatformProvider
)

func (p Profile) flags(verified bool) byte {
	value := flagUserPresent | flagBackupEligible
	if verified {
		value |= flagUserVerified
	}
	if p == PlatformProvider {
		value |= flagBackupState
	}
	return value
}

// authenticatorData lays out WebAuthn §6.1: rpID SHA-256, flags, big-endian uint32 signature counter, attested credential data.
func authenticatorData(rpID string, flags byte, counter uint32, attested []byte) []byte {
	rpIDHash := sha256.Sum256([]byte(rpID))
	data := make([]byte, 0, len(rpIDHash)+1+4+len(attested))
	data = append(data, rpIDHash[:]...)
	data = append(data, flags)
	data = binary.BigEndian.AppendUint32(data, counter)
	return append(data, attested...)
}
