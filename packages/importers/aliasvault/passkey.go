package aliasvault

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

// The only JSON Web Key kind from AliasVault that the vault holds.
const (
	keyTypeEC = "EC"
	curveP256 = "P-256"
)

// uncompressedPoint leads a public point written as its two coordinates, SEC 1 section 2.3.3.
const uncompressedPoint = 0x04

// passkey is a manifest passkey; PrivateKey is a JSON Web Key exported by WebCrypto.
type passkey struct {
	ID          string `json:"id"`
	RPID        string `json:"rpId"`
	UserHandle  string `json:"userHandle"`
	PrivateKey  string `json:"privateKey"`
	DisplayName string `json:"displayName"`
}

// jsonWebKey is what an elliptic-curve private JSON Web Key holds, RFC 7518 section 6.2.
type jsonWebKey struct {
	KeyType string `json:"kty"`
	Curve   string `json:"crv"`
	D       string `json:"d"`
	X       string `json:"x"`
	Y       string `json:"y"`
}

// read decodes the passkey for userName; AliasVault stores no counter and no creation date.
func (p passkey) read(userName string, now time.Time) (vault.Passkey, bool) {
	credentialID, readable := importers.GUIDBytes(p.ID)
	userHandle, err := base64.StdEncoding.DecodeString(p.UserHandle)
	if !readable || err != nil {
		return vault.Passkey{}, false
	}
	key, readable := pkcs8Of(p.PrivateKey)
	if !readable {
		return vault.Passkey{}, false
	}
	return vault.Passkey{
		CredentialID:    credentialID,
		RPID:            strings.TrimSpace(p.RPID),
		UserHandle:      userHandle,
		UserName:        userName,
		UserDisplayName: strings.TrimSpace(p.DisplayName),
		PrivateKey:      key,
		Discoverable:    true,
		CreatedAt:       now,
	}, true
}

// pkcs8Of converts a P-256 private JSON Web Key to PKCS #8, refusing a mismatched public point.
// Every reachable copy of the private scalar must be cleared.
func pkcs8Of(written string) ([]byte, bool) {
	text := []byte(written)
	defer clear(text)
	var key jsonWebKey
	if err := json.Unmarshal(text, &key); err != nil || key.KeyType != keyTypeEC || key.Curve != curveP256 {
		return nil, false
	}
	scalar, scalarErr := importers.DecodeBase64URL(key.D)
	defer clear(scalar)
	x, xErr := importers.DecodeBase64URL(key.X)
	y, yErr := importers.DecodeBase64URL(key.Y)
	if scalarErr != nil || xErr != nil || yErr != nil {
		return nil, false
	}
	private, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), scalar)
	if err != nil {
		return nil, false
	}
	point, err := private.PublicKey.Bytes()
	if err != nil || !bytes.Equal(point, slices.Concat([]byte{uncompressedPoint}, x, y)) {
		return nil, false
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	return der, err == nil
}

// passkeys returns the accepted passkeys and subtracts them from the left-behind count.
func (m *mapper) passkeys(written []passkey, userName string) []vault.Passkey {
	read := make([]vault.Passkey, 0, len(written))
	for _, source := range written {
		if passkey, readable := source.read(userName, m.now); readable {
			read = append(read, passkey)
		}
	}
	kept := importers.KeepPasskeys(read)
	m.export.Passkeys -= len(kept)
	return kept
}
