package importers

import (
	"bytes"
	"encoding/base64"
	"slices"
	"strings"

	"github.com/dortanes/ravenpass/packages/vault"
	"github.com/google/uuid"
)

// guidLength is a GUID written 8-4-4-4-12; uuid.Parse also takes braced, URN and bare forms.
const guidLength = 36

// KeepPasskeys returns, in order, the valid passkeys one credential takes, without repeated credential IDs, and clears every other private key.
func KeepPasskeys(passkeys []vault.Passkey) []vault.Passkey {
	var kept []vault.Passkey
	for _, passkey := range passkeys {
		repeated := slices.ContainsFunc(kept, func(held vault.Passkey) bool { return bytes.Equal(held.CredentialID, passkey.CredentialID) })
		if len(kept) == vault.MaxCredentialPasskeys || repeated || !vault.ValidPasskey(passkey) {
			clear(passkey.PrivateKey)
			continue
		}
		kept = append(kept, passkey)
	}
	return kept
}

// GUIDBytes reads a GUID written 8-4-4-4-12, and returns its 16 bytes in the order written.
func GUIDBytes(text string) ([]byte, bool) {
	if len(text) != guidLength {
		return nil, false
	}
	id, err := uuid.Parse(text)
	if err != nil {
		return nil, false
	}
	return id[:], true
}

// DecodeBase64URL reads base64url written with or without its padding.
func DecodeBase64URL(text string) ([]byte, error) {
	if strings.HasSuffix(text, "=") {
		return base64.URLEncoding.DecodeString(text)
	}
	return base64.RawURLEncoding.DecodeString(text)
}
