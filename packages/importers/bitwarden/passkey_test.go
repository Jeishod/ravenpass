package bitwarden

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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

const passkeyGUID = "00112233-4455-6677-8899-aabbccddeeff"

var passkeyGUIDBytes = []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

func pkcs8Key(t *testing.T, curve elliptic.Curve) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// fido2JSON writes a Bitwarden passkey holding key; changes override fields, nil drops one.
func fido2JSON(t *testing.T, key []byte, changes map[string]any) string {
	t.Helper()
	fields := map[string]any{
		"credentialId":    passkeyGUID,
		"keyType":         "public-key",
		"keyAlgorithm":    "ECDSA",
		"keyCurve":        "P-256",
		"keyValue":        base64.RawURLEncoding.EncodeToString(key),
		"rpId":            "example.test",
		"userHandle":      base64.RawURLEncoding.EncodeToString([]byte("user-1")),
		"userName":        "alex@example.test",
		"counter":         "7",
		"rpName":          "Example",
		"userDisplayName": "Alex",
		"discoverable":    "true",
		"creationDate":    "2024-05-06T07:08:09.123Z",
	}
	for name, value := range changes {
		if value == nil {
			delete(fields, name)
		} else {
			fields[name] = value
		}
	}
	written, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(written)
}

func loginWithPasskeys(passkeys ...string) string {
	return `{"type": 1, "name": "Example", "login": {"username": "alex", "fido2Credentials": [` + strings.Join(passkeys, ",") + `]}}`
}

// passkeysRead returns the passkeys one login keeps and how many the export leaves behind.
func passkeysRead(t *testing.T, passkeys ...string) ([]vault.Passkey, int) {
	t.Helper()
	export := readItems(t, loginWithPasskeys(passkeys...))
	if len(export.Items) != 1 || export.Items[0].Content.Credential == nil {
		t.Fatalf("read %+v", export)
	}
	return export.Items[0].Content.Credential.Passkeys, export.Passkeys
}

func TestPasskeyIsReadWithEveryField(t *testing.T) {
	key := pkcs8Key(t, elliptic.P256())
	passkeys, left := passkeysRead(t, fido2JSON(t, key, nil))
	want := vault.Passkey{
		CredentialID:    passkeyGUIDBytes,
		RPID:            "example.test",
		UserHandle:      []byte("user-1"),
		UserName:        "alex@example.test",
		UserDisplayName: "Alex",
		PrivateKey:      key,
		Counter:         7,
		Discoverable:    true,
		CreatedAt:       time.Date(2024, 5, 6, 7, 8, 9, 123_000_000, time.UTC),
	}
	if left != 0 || len(passkeys) != 1 || !reflect.DeepEqual(passkeys[0], want) {
		t.Fatalf("read %+v, left %d behind", passkeys, left)
	}
}

func TestPasskeyReadsEachFormOfItsFields(t *testing.T) {
	key := pkcs8Key(t, elliptic.P256())
	rawID := bytes.Repeat([]byte{0xfb}, 20)
	tests := []struct {
		name    string
		changes map[string]any
		check   func(vault.Passkey) bool
	}{
		{"GUID in upper case", map[string]any{"credentialId": strings.ToUpper(passkeyGUID)}, func(p vault.Passkey) bool { return bytes.Equal(p.CredentialID, passkeyGUIDBytes) }},
		{"base64url credential ID", map[string]any{"credentialId": "b64." + base64.RawURLEncoding.EncodeToString(rawID)}, func(p vault.Passkey) bool { return bytes.Equal(p.CredentialID, rawID) }},
		{"padded key and user handle", map[string]any{"keyValue": base64.URLEncoding.EncodeToString(key), "userHandle": base64.URLEncoding.EncodeToString([]byte("user-1"))}, func(p vault.Passkey) bool {
			return bytes.Equal(p.PrivateKey, key) && bytes.Equal(p.UserHandle, []byte("user-1"))
		}},
		{"zero counter", map[string]any{"counter": "0"}, func(p vault.Passkey) bool { return p.Counter == 0 }},
		{"largest counter", map[string]any{"counter": "4294967295"}, func(p vault.Passkey) bool { return p.Counter == 4294967295 }},
		{"not discoverable", map[string]any{"discoverable": "false"}, func(p vault.Passkey) bool { return !p.Discoverable }},
		{"no discoverability", map[string]any{"discoverable": nil}, func(p vault.Passkey) bool { return !p.Discoverable }},
		{"creation date with an offset", map[string]any{"creationDate": "2024-05-06T09:08:09+02:00"}, func(p vault.Passkey) bool {
			return p.CreatedAt.Equal(time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC))
		}},
		{"names in surrounding space", map[string]any{"userName": " alex ", "userDisplayName": " Alex ", "rpId": " example.test "}, func(p vault.Passkey) bool {
			return p.UserName == "alex" && p.UserDisplayName == "Alex" && p.RPID == "example.test"
		}},
		{"no names", map[string]any{"userName": nil, "userDisplayName": nil}, func(p vault.Passkey) bool { return p.UserName == "" && p.UserDisplayName == "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			passkeys, left := passkeysRead(t, fido2JSON(t, key, test.changes))
			if left != 0 || len(passkeys) != 1 || !test.check(passkeys[0]) {
				t.Fatalf("read %+v, left %d behind", passkeys, left)
			}
		})
	}
}

func TestPasskeyWithoutAReadableCreationDateIsCreatedAtTheImport(t *testing.T) {
	key := pkcs8Key(t, elliptic.P256())
	for _, date := range []any{nil, "", "yesterday", "2024-05-06", "1969-12-31T23:59:59Z"} {
		before := time.Now()
		passkeys, left := passkeysRead(t, fido2JSON(t, key, map[string]any{"creationDate": date}))
		after := time.Now()
		if left != 0 || len(passkeys) != 1 || passkeys[0].CreatedAt.Before(before) || passkeys[0].CreatedAt.After(after) {
			t.Fatalf("%v: read %+v, left %d behind", date, passkeys, left)
		}
	}
}

func TestPasskeyTheVaultCannotHoldIsLeftBehind(t *testing.T) {
	key := pkcs8Key(t, elliptic.P256())
	tests := map[string]map[string]any{
		"another key type":            {"keyType": "secret"},
		"another algorithm":           {"keyAlgorithm": "EdDSA"},
		"another curve":               {"keyCurve": "P-384"},
		"a key that is not base64":    {"keyValue": "not base64!"},
		"no key":                      {"keyValue": nil},
		"a P-384 key named P-256":     {"keyValue": base64.RawURLEncoding.EncodeToString(pkcs8Key(t, elliptic.P384()))},
		"a key that is not PKCS #8":   {"keyValue": base64.RawURLEncoding.EncodeToString([]byte("not a key"))},
		"no relying party":            {"rpId": nil},
		"an upper-case relying party": {"rpId": "Example.test"},
		"a relying party URL":         {"rpId": "https://example.test"},
		"no user handle":              {"userHandle": nil},
		"a user handle over 64":       {"userHandle": base64.RawURLEncoding.EncodeToString(make([]byte, 65))},
		"a malformed GUID":            {"credentialId": "not-a-guid"},
		"no credential ID":            {"credentialId": nil},
		"a malformed base64 ID":       {"credentialId": "b64.!!"},
		"a base64 ID under 16":        {"credentialId": "b64." + base64.RawURLEncoding.EncodeToString(make([]byte, 15))},
		"a negative counter":          {"counter": "-1"},
		"a counter over 32 bits":      {"counter": "4294967296"},
		"no counter":                  {"counter": nil},
		"a user name over its limit":  {"userName": strings.Repeat("a", vault.MaxLoginLength+1)},
	}
	for name, changes := range tests {
		t.Run(name, func(t *testing.T) {
			passkeys, left := passkeysRead(t, fido2JSON(t, key, changes))
			if left != 1 || passkeys != nil {
				t.Fatalf("read %+v, left %d behind", passkeys, left)
			}
		})
	}
}

func TestLoginTakesItsFirstEightDistinctPasskeys(t *testing.T) {
	idOf := func(i int) string { return fmt.Sprintf("00000000-0000-0000-0000-%012x", i) }
	written := []string{
		fido2JSON(t, pkcs8Key(t, elliptic.P256()), map[string]any{"credentialId": idOf(0)}),
		fido2JSON(t, pkcs8Key(t, elliptic.P256()), map[string]any{"credentialId": idOf(0), "userName": "repeated"}),
		fido2JSON(t, pkcs8Key(t, elliptic.P256()), map[string]any{"credentialId": idOf(1), "rpId": "Invalid.test"}),
	}
	for i := range vault.MaxCredentialPasskeys {
		written = append(written, fido2JSON(t, pkcs8Key(t, elliptic.P256()), map[string]any{"credentialId": idOf(i + 1)}))
	}

	passkeys, left := passkeysRead(t, written...)

	if left != 3 || len(passkeys) != vault.MaxCredentialPasskeys {
		t.Fatalf("took %d passkeys, left %d behind", len(passkeys), left)
	}
	for i, passkey := range passkeys {
		if id, _ := importers.GUIDBytes(idOf(i)); !bytes.Equal(passkey.CredentialID, id) || passkey.UserName == "repeated" || passkey.RPID != "example.test" {
			t.Fatalf("passkey %d is %+v", i, passkey)
		}
	}
}

func TestPasswordProtectedExportBringsItsPasskeys(t *testing.T) {
	key := pkcs8Key(t, elliptic.P256())
	opened := openContent(t, protectedExport(t, "password", fastPBKDF2, jsonExportOf(loginWithPasskeys(fido2JSON(t, key, nil), fido2JSON(t, key, map[string]any{"keyCurve": "P-384"})))))
	if err := opened.Unlock("password"); err != nil {
		t.Fatal(err)
	}
	export, err := opened.Read(english)
	if err != nil {
		t.Fatal(err)
	}
	passkeys := export.Items[0].Content.Credential.Passkeys
	if export.Passkeys != 1 || len(passkeys) != 1 || !bytes.Equal(passkeys[0].PrivateKey, key) || !bytes.Equal(passkeys[0].CredentialID, passkeyGUIDBytes) {
		t.Fatalf("read %+v, left %d behind", passkeys, export.Passkeys)
	}
}

func TestImportedPasskeyIsHeldByTheVault(t *testing.T) {
	key := pkcs8Key(t, elliptic.P256())
	export := readItems(t,
		loginWithPasskeys(fido2JSON(t, key, nil), fido2JSON(t, key, map[string]any{"keyValue": "!"})),
		`{"type": 2, "name": "Router", "login": {"fido2Credentials": [{"credentialId": "one"}]}}`,
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
	if want := export.Items[0].Content.Credential.Passkeys[0]; !bytes.Equal(read.PrivateKey, key) || read.Counter != want.Counter || read.RPID != want.RPID || !bytes.Equal(read.UserHandle, want.UserHandle) {
		t.Fatalf("the vault holds %+v", read)
	}
}
