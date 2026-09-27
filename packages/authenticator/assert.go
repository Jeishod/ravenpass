package authenticator

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
)

// Assertion is what a sign-in returns to the page.
type Assertion struct {
	AuthenticatorData []byte
	// Signature is ASN.1 DER ECDSA over SHA-256(AuthenticatorData || SHA-256(client data)), WebAuthn §6.3.3.
	Signature []byte
}

// Assert signs clientData with a passkey for rpID, as AssertHash signs its SHA-256 hash.
func (p Profile) Assert(rpID string, privateKey []byte, counter uint32, verified bool, clientData []byte) (Assertion, error) {
	return p.AssertHash(rpID, privateKey, counter, verified, sha256.Sum256(clientData))
}

// AssertHash signs with a passkey for rpID over clientDataHash, the SHA-256 of client data a platform built.
func (p Profile) AssertHash(rpID string, privateKey []byte, counter uint32, verified bool, clientDataHash [32]byte) (Assertion, error) {
	key, err := parseKey(privateKey)
	if err != nil {
		return Assertion{}, err
	}
	authData := authenticatorData(rpID, p.flags(verified), counter, nil)
	signed := sha256.New()
	signed.Write(authData)
	signed.Write(clientDataHash[:])
	signature, err := ecdsa.SignASN1(rand.Reader, key, signed.Sum(nil))
	if err != nil {
		return Assertion{}, err
	}
	return Assertion{AuthenticatorData: authData, Signature: signature}, nil
}
