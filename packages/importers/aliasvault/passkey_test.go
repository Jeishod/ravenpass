package aliasvault

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/vault"
)

const passkeyGUID = "a1b2c3d4-e5f6-0718-293a-4b5c6d7e8f90"

var passkeyGUIDBytes = []byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18, 0x29, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f, 0x90}

func ecdsaKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func pkcs8Form(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// ecJWK is the private JSON Web Key WebCrypto exports of an elliptic-curve key.
func ecJWK(t *testing.T, key *ecdsa.PrivateKey) map[string]any {
	t.Helper()
	scalar, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	point, err := key.PublicKey.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	size := (len(point) - 1) / 2
	return map[string]any{
		"kty":     "EC",
		"crv":     key.Curve.Params().Name,
		"d":       base64.RawURLEncoding.EncodeToString(scalar),
		"x":       base64.RawURLEncoding.EncodeToString(point[1 : 1+size]),
		"y":       base64.RawURLEncoding.EncodeToString(point[1+size:]),
		"ext":     true,
		"key_ops": []string{"sign"},
	}
}

func jsonText(t *testing.T, value any) string {
	t.Helper()
	written, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(written)
}

// passkeyJSON writes a manifest passkey holding jwk; changes override fields, nil writes null.
func passkeyJSON(t *testing.T, jwk map[string]any, changes map[string]any) string {
	t.Helper()
	fields := map[string]any{
		"id":          passkeyGUID,
		"itemId":      "l1",
		"rpId":        "example.test",
		"userHandle":  base64.StdEncoding.EncodeToString([]byte("user-1")),
		"publicKey":   `{"kty":"EC","crv":"P-256"}`,
		"privateKey":  jsonText(t, jwk),
		"prfKey":      nil,
		"displayName": "Alex",
		"createdAt":   "2026-03-19T08:59:16.938Z",
		"isDeleted":   false,
	}
	for name, value := range changes {
		fields[name] = value
	}
	return jsonText(t, fields)
}

func loginWithPasskeys(username, email string, passkeys ...string) string {
	return fmt.Sprintf(`{"id": "l1", "name": "Example", "itemType": "Login", "fieldValues": [
		{"fieldKey": "login.username", "value": %q, "weight": 0},
		{"fieldKey": "login.email", "value": %q, "weight": 0}
	], "totpCodes": [], "passkeys": [%s]}`, username, email, strings.Join(passkeys, ","))
}

// passkeysRead returns the passkeys one login keeps and how many the export leaves behind.
func passkeysRead(t *testing.T, username, email string, passkeys ...string) ([]vault.Passkey, int) {
	t.Helper()
	export := readItems(t, loginWithPasskeys(username, email, passkeys...))
	if len(export.Items) != 1 || export.Items[0].Content.Credential == nil {
		t.Fatalf("read %+v", export)
	}
	return export.Items[0].Content.Credential.Passkeys, export.Passkeys
}

func TestPasskeyIsReadFromAnECJSONWebKey(t *testing.T) {
	key := ecdsaKey(t, elliptic.P256())
	before := time.Now()
	passkeys, left := passkeysRead(t, "alex", "alex@example.test", passkeyJSON(t, ecJWK(t, key), nil))
	after := time.Now()
	if left != 0 || len(passkeys) != 1 {
		t.Fatalf("read %+v, left %d behind", passkeys, left)
	}
	got := passkeys[0]
	if got.CreatedAt.Before(before) || got.CreatedAt.After(after) {
		t.Fatalf("created at %v, not at the import", got.CreatedAt)
	}
	got.CreatedAt = time.Time{}
	want := vault.Passkey{
		CredentialID:    passkeyGUIDBytes,
		RPID:            "example.test",
		UserHandle:      []byte("user-1"),
		UserName:        "alex",
		UserDisplayName: "Alex",
		PrivateKey:      pkcs8Form(t, key),
		Discoverable:    true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read %+v, want %+v", got, want)
	}
}

func TestPasskeyAccountIsTheLoginElseTheEmail(t *testing.T) {
	jwk := ecJWK(t, ecdsaKey(t, elliptic.P256()))
	tests := []struct {
		name            string
		username, email string
		account         string
	}{
		{"login and email", " alex ", "alex@example.test", "alex"},
		{"email alone", "", "alex@example.test", "alex@example.test"},
		{"username that is an email", "alex@example.test", "", "alex@example.test"},
		{"neither", "", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			passkeys, _ := passkeysRead(t, test.username, test.email, passkeyJSON(t, jwk, nil))
			if len(passkeys) != 1 || passkeys[0].UserName != test.account {
				t.Fatalf("read %+v, want account %q", passkeys, test.account)
			}
		})
	}
}

func TestPasskeyTheVaultCannotHoldIsLeftBehind(t *testing.T) {
	key := ecdsaKey(t, elliptic.P256())
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaJWK := map[string]any{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		"e":   "AQAB",
		"d":   base64.RawURLEncoding.EncodeToString(rsaKey.D.Bytes()),
	}
	mismatched := ecJWK(t, key)
	other := ecJWK(t, ecdsaKey(t, elliptic.P256()))
	mismatched["x"], mismatched["y"] = other["x"], other["y"]
	namedP256 := ecJWK(t, ecdsaKey(t, elliptic.P384()))
	namedP256["crv"] = "P-256"
	withScalar := func(scalar string) map[string]any {
		jwk := ecJWK(t, key)
		jwk["d"] = scalar
		return jwk
	}
	tests := map[string]string{
		"an RSA key":                   passkeyJSON(t, rsaJWK, nil),
		"a P-384 key":                  passkeyJSON(t, ecJWK(t, ecdsaKey(t, elliptic.P384())), nil),
		"a P-384 key named P-256":      passkeyJSON(t, namedP256, nil),
		"a public point of another":    passkeyJSON(t, mismatched, nil),
		"a scalar that is not base64":  passkeyJSON(t, withScalar("!!"), nil),
		"a zero scalar":                passkeyJSON(t, withScalar(base64.RawURLEncoding.EncodeToString(make([]byte, 32))), nil),
		"no scalar":                    passkeyJSON(t, withScalar(""), nil),
		"a private key that is no JWK": passkeyJSON(t, nil, map[string]any{"privateKey": "{"}),
		"no private key":               passkeyJSON(t, nil, map[string]any{"privateKey": nil}),
		"an ID that is not a GUID":     passkeyJSON(t, ecJWK(t, key), map[string]any{"id": "l1"}),
		"no ID":                        passkeyJSON(t, ecJWK(t, key), map[string]any{"id": nil}),
		"a user handle in base64url":   passkeyJSON(t, ecJWK(t, key), map[string]any{"userHandle": "-_8"}),
		"no user handle":               passkeyJSON(t, ecJWK(t, key), map[string]any{"userHandle": nil}),
		"no relying party":             passkeyJSON(t, ecJWK(t, key), map[string]any{"rpId": nil}),
		"an upper-case relying party":  passkeyJSON(t, ecJWK(t, key), map[string]any{"rpId": "Example.test"}),
	}
	for name, written := range tests {
		t.Run(name, func(t *testing.T) {
			passkeys, left := passkeysRead(t, "alex", "", written)
			if left != 1 || passkeys != nil {
				t.Fatalf("read %+v, left %d behind", passkeys, left)
			}
		})
	}
}

func TestImportedPasskeyIsHeldByTheVault(t *testing.T) {
	key := ecdsaKey(t, elliptic.P256())
	rsaJWK := map[string]any{"kty": "RSA", "n": "AQAB", "e": "AQAB"}
	export := readItems(t,
		loginWithPasskeys("alex", "", passkeyJSON(t, ecJWK(t, key), nil), passkeyJSON(t, rsaJWK, map[string]any{"id": "00000000-0000-0000-0000-000000000001"})),
		`{"id": "n1", "name": "Router", "itemType": "Note", "passkeys": [{"rpId": "example.test"}]}`,
	)
	plan := importers.NewPlan(export, nil, nil)
	if preview := plan.Preview(); preview.ImportedPasskeys != 1 || preview.Passkeys != 2 {
		t.Fatalf("preview imports %d passkeys and leaves %d behind", preview.ImportedPasskeys, preview.Passkeys)
	}
	created, err := vault.Create()
	if err != nil {
		t.Fatal(err)
	}
	session := created.Session
	defer session.Lock()
	pending, result, err := session.PrepareAddItems(plan.Items(importers.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	read, err := session.ReadPasskey(result.Items[0], passkeyGUIDBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read.PrivateKey, pkcs8Form(t, key)) || read.UserName != "alex" || read.Counter != 0 || !read.Discoverable {
		t.Fatalf("the vault holds %+v", read)
	}
}
