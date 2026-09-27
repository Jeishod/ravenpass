package authenticator

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"testing"
)

func newKey(t *testing.T) Key {
	t.Helper()
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestARelyingPartyAcceptsTheKeyWhenItListsES256OrNoAlgorithm(t *testing.T) {
	for _, test := range []struct {
		algorithms []int
		want       bool
	}{
		{nil, true},
		{[]int{-8, AlgorithmES256, -257}, true},
		{[]int{-257, -8}, false},
	} {
		if got := Accepts(test.algorithms); got != test.want {
			t.Errorf("Accepts(%v) = %t, want %t", test.algorithms, got, test.want)
		}
	}
}

func TestANewKeyIsAP256KeyWithASixteenByteCredentialID(t *testing.T) {
	key := newKey(t)
	if len(key.CredentialID) != 16 {
		t.Fatalf("credential ID is %d bytes, want 16", len(key.CredentialID))
	}
	parsed, err := x509.ParsePKCS8PrivateKey(key.PrivateKey)
	if err != nil {
		t.Fatalf("private key is not PKCS #8: %v", err)
	}
	private, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || private.Curve != elliptic.P256() {
		t.Fatalf("private key is %T, want an ECDSA P-256 key", parsed)
	}
	other := newKey(t)
	if bytes.Equal(key.CredentialID, other.CredentialID) || bytes.Equal(key.PrivateKey, other.PrivateKey) {
		t.Fatal("two new keys are the same")
	}
}

func pkcs8(t *testing.T, key any) []byte {
	t.Helper()
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestOnlyAP256PrivateKeyAttestsOrSigns(t *testing.T) {
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, edwards, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p256, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sec1, err := x509.MarshalECPrivateKey(p256)
	if err != nil {
		t.Fatal(err)
	}
	for name, privateKey := range map[string][]byte{
		"empty":            nil,
		"not DER":          []byte("private key"),
		"a P-384 key":      pkcs8(t, p384),
		"an Ed25519 key":   pkcs8(t, edwards),
		"an RSA key":       pkcs8(t, rsaKey),
		"SEC 1, not PKCS8": sec1,
	} {
		if _, err := BrowserExtension.Attest("example.com", newKey(t).CredentialID, privateKey, true); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Attest with %s: got %v, want ErrInvalidKey", name, err)
		}
		if _, err := BrowserExtension.Assert("example.com", privateKey, 0, true, ClientData(Get, nil, "https://example.com")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Assert with %s: got %v, want ErrInvalidKey", name, err)
		}
	}
}

func TestACredentialIDOfTheWrongLengthIsNotAttested(t *testing.T) {
	key := newKey(t)
	for name, credentialID := range map[string][]byte{
		"empty":           nil,
		"over 1023 bytes": make([]byte, 1024),
	} {
		if _, err := BrowserExtension.Attest("example.com", credentialID, key.PrivateKey, true); !errors.Is(err, ErrInvalidCredentialID) {
			t.Errorf("%s: got %v, want ErrInvalidCredentialID", name, err)
		}
	}
	if _, err := BrowserExtension.Attest("example.com", make([]byte, 1023), key.PrivateKey, true); err != nil {
		t.Errorf("a 1023-byte credential ID: %v", err)
	}
}

func TestTheAttestationIsWrittenInCanonicalCBOR(t *testing.T) {
	key := newKey(t)
	for _, tested := range profiles {
		attestation, err := tested.profile.Attest("example.com", key.CredentialID, key.PrivateKey, false)
		if err != nil {
			t.Fatal(err)
		}
		// {"fmt": "none", "attStmt": {}, "authData": 148-byte string}: keys sorted by length.
		header := []byte{0xa3, 0x63, 'f', 'm', 't', 0x64, 'n', 'o', 'n', 'e', 0x67, 'a', 't', 't', 'S', 't', 'm', 't', 0xa0, 0x68, 'a', 'u', 't', 'h', 'D', 'a', 't', 'a', 0x58, 148}
		if !bytes.HasPrefix(attestation.AttestationObject, header) {
			t.Fatalf("%s: attestation object starts % x, want % x", tested.name, attestation.AttestationObject[:len(header)], header)
		}
		if !bytes.Equal(attestation.AttestationObject[len(header):], attestation.AuthenticatorData) {
			t.Fatalf("%s: the attestation object does not carry the authenticator data", tested.name)
		}
		// rpIdHash (32), flags, counter (4), AAGUID (16), ID length (2), ID (16), COSE key.
		if aaguid := attestation.AuthenticatorData[37:53]; !bytes.Equal(aaguid, AAGUID[:]) {
			t.Fatalf("%s: AAGUID % x, want % x", tested.name, aaguid, AAGUID)
		}
		coseKey := attestation.AuthenticatorData[32+1+4+16+2+16:]
		// {1: 2, 3: -7, -1: 1, -2: x, -3: y}
		if !bytes.HasPrefix(coseKey, []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}) ||
			!bytes.Equal(coseKey[10+32:10+32+3], []byte{0x22, 0x58, 0x20}) || len(coseKey) != 77 {
			t.Fatalf("%s: COSE key is % x", tested.name, coseKey)
		}
	}
}
