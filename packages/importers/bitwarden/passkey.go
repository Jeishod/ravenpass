package bitwarden

import (
	"strconv"
	"strings"
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// The only passkey key kind from Bitwarden that the vault holds.
const (
	keyTypePublicKey  = "public-key"
	keyAlgorithmECDSA = "ECDSA"
	keyCurveP256      = "P-256"
)

// credentialIDBase64 prefixes a base64url credential ID; any other ID is a GUID.
const credentialIDBase64 = "b64."

// fido2Credential is a JSON export passkey; every value is a string, and any may be null.
type fido2Credential struct {
	CredentialID    string `json:"credentialId"`
	KeyType         string `json:"keyType"`
	KeyAlgorithm    string `json:"keyAlgorithm"`
	KeyCurve        string `json:"keyCurve"`
	KeyValue        string `json:"keyValue"`
	RPID            string `json:"rpId"`
	UserHandle      string `json:"userHandle"`
	UserName        string `json:"userName"`
	UserDisplayName string `json:"userDisplayName"`
	Counter         string `json:"counter"`
	Discoverable    string `json:"discoverable"`
	CreationDate    string `json:"creationDate"`
}

// passkey decodes an ECDSA P-256 passkey; importers.KeepPasskeys decides whether it is kept.
func (c fido2Credential) passkey(now time.Time) (vault.Passkey, bool) {
	if c.KeyType != keyTypePublicKey || c.KeyAlgorithm != keyAlgorithmECDSA || c.KeyCurve != keyCurveP256 {
		return vault.Passkey{}, false
	}
	credentialID, readable := c.credentialID()
	userHandle, handleErr := importers.DecodeBase64URL(c.UserHandle)
	counter, counterErr := strconv.ParseUint(c.Counter, 10, 32)
	if !readable || handleErr != nil || counterErr != nil {
		return vault.Passkey{}, false
	}
	key, err := importers.DecodeBase64URL(c.KeyValue)
	if err != nil {
		clear(key)
		return vault.Passkey{}, false
	}
	return vault.Passkey{
		CredentialID:    credentialID,
		RPID:            strings.TrimSpace(c.RPID),
		UserHandle:      userHandle,
		UserName:        strings.TrimSpace(c.UserName),
		UserDisplayName: strings.TrimSpace(c.UserDisplayName),
		PrivateKey:      key,
		Counter:         uint32(counter),
		Discoverable:    c.Discoverable == "true",
		CreatedAt:       c.createdAt(now),
	}, true
}

// credentialID reads a GUID as its 16 bytes, or base64url after credentialIDBase64.
func (c fido2Credential) credentialID() ([]byte, bool) {
	if encoded, prefixed := strings.CutPrefix(c.CredentialID, credentialIDBase64); prefixed {
		decoded, err := importers.DecodeBase64URL(encoded)
		return decoded, err == nil
	}
	return importers.GUIDBytes(c.CredentialID)
}

// createdAt reads a creation date before the Unix epoch, or not RFC 3339, as now.
func (c fido2Credential) createdAt(now time.Time) time.Time {
	created, err := time.Parse(time.RFC3339, c.CreationDate)
	if err != nil || created.Unix() < 0 {
		return now
	}
	return created
}

// passkeys returns the accepted passkeys and subtracts them from the left-behind count.
func (m *mapper) passkeys(written []fido2Credential) []vault.Passkey {
	read := make([]vault.Passkey, 0, len(written))
	for _, credential := range written {
		if passkey, readable := credential.passkey(m.now); readable {
			read = append(read, passkey)
		}
	}
	kept := importers.KeepPasskeys(read)
	m.export.Passkeys -= len(kept)
	return kept
}
