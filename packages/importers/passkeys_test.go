package importers

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"reflect"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// testPasskey is a passkey the vault accepts whose credential ID is 16 bytes of mark.
func testPasskey(t *testing.T, mark byte) vault.Passkey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return vault.Passkey{
		CredentialID: bytes.Repeat([]byte{mark}, 16),
		RPID:         "example.test",
		UserHandle:   []byte{mark},
		UserName:     "alex",
		PrivateKey:   der,
		Discoverable: true,
		CreatedAt:    time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
	}
}

func TestKeepPasskeysKeepsWhatOneCredentialTakes(t *testing.T) {
	invalid := testPasskey(t, 1)
	invalid.RPID = "Example.test"
	first := testPasskey(t, 2)
	repeated := testPasskey(t, 2)
	passkeys := []vault.Passkey{invalid, first, repeated}
	for i := range vault.MaxCredentialPasskeys {
		passkeys = append(passkeys, testPasskey(t, byte(i+3)))
	}
	beyond := passkeys[len(passkeys)-1]
	leftBehind := []vault.Passkey{invalid, repeated, beyond}

	kept := KeepPasskeys(passkeys)

	want := append([]vault.Passkey{first}, passkeys[3:3+vault.MaxCredentialPasskeys-1]...)
	if !reflect.DeepEqual(kept, want) {
		t.Fatalf("kept %d passkeys, want %d", len(kept), len(want))
	}
	for _, passkey := range leftBehind {
		if !bytes.Equal(passkey.PrivateKey, make([]byte, len(passkey.PrivateKey))) {
			t.Fatalf("the key of %x left behind survives", passkey.CredentialID)
		}
	}
	if _, err := vault.PreviewNewItem(vault.NewItem{Credential: &vault.CredentialInput{Label: "Kept", Passkeys: kept}}); err != nil {
		t.Fatalf("the vault refuses the kept passkeys: %v", err)
	}
	if none := KeepPasskeys(nil); none != nil {
		t.Fatalf("no passkeys kept %v", none)
	}
}

func TestGUIDBytesReadsTheDigitsInWrittenOrder(t *testing.T) {
	want := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	for _, written := range []string{"00112233-4455-6677-8899-aabbccddeeff", "00112233-4455-6677-8899-AABBCCDDEEFF"} {
		if got, readable := GUIDBytes(written); !readable || !bytes.Equal(got, want) {
			t.Fatalf("%s read %x, %v", written, got, readable)
		}
	}
	for _, written := range []string{
		"",
		"00112233445566778899aabbccddeeff",
		"{00112233-4455-6677-8899-aabbccddeeff}",
		"0011223-34455-6677-8899-aabbccddeeff",
		"00112233-4455-6677-8899-aabbccddeefg",
		"00112233-4455-6677-8899-aabbccdd-eff",
	} {
		if got, readable := GUIDBytes(written); readable {
			t.Fatalf("%q read %x", written, got)
		}
	}
}

func TestDecodeBase64URLTakesItWithOrWithoutPadding(t *testing.T) {
	for _, written := range []string{"-_8", "-_8="} {
		if got, err := DecodeBase64URL(written); err != nil || !bytes.Equal(got, []byte{0xfb, 0xff}) {
			t.Fatalf("%q read %x, %v", written, got, err)
		}
	}
	for _, written := range []string{"+/8=", "-_8==", "-_8=x"} {
		if got, err := DecodeBase64URL(written); err == nil {
			t.Fatalf("%q read %x", written, got)
		}
	}
}
