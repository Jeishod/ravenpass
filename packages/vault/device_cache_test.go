package vault

import (
	"bytes"
	"errors"
	"testing"
)

func TestDeviceCacheRoundTripsAndSurvivesAReopen(t *testing.T) {
	created, _ := vaultWith(t)
	plaintext := []byte(`{"version":1,"sites":{"example.com":{"icon":"","checkedAt":1}}}`)
	sealed, err := created.Session.SealDeviceCache("site-icons", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("example.com")) {
		t.Fatal("the sealed cache carries its plaintext")
	}
	opened, err := created.Session.OpenDeviceCache("site-icons", sealed)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("opened %q, %v", opened, err)
	}
	container, _, err := created.Session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	created.Session.Lock()
	reopened, err := OpenWithRecovery(container, created.RecoveryPhrase)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Lock()
	if opened, err := reopened.OpenDeviceCache("site-icons", sealed); err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("opened after a reopen %q, %v", opened, err)
	}
	empty, err := reopened.SealDeviceCache("site-icons", nil)
	if err != nil {
		t.Fatal(err)
	}
	if opened, err := reopened.OpenDeviceCache("site-icons", empty); err != nil || len(opened) != 0 {
		t.Fatalf("empty cache opened as %q, %v", opened, err)
	}
}

func TestDeviceCacheRefusesAnotherVaultPurposeOrAlteration(t *testing.T) {
	first, _ := vaultWith(t)
	defer first.Session.Lock()
	second, _ := vaultWith(t)
	defer second.Session.Lock()
	sealed, err := first.Session.SealDeviceCache("site-icons", []byte("icons"))
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	relabeled := append([]byte(nil), sealed...)
	copy(relabeled[2:18], second.Session.vaultID[:])
	unknownSuite := append([]byte(nil), sealed...)
	unknownSuite[18] = 2
	tests := []struct {
		name    string
		session *Session
		purpose string
		sealed  []byte
		want    error
	}{
		{"another vault", second.Session, "site-icons", sealed, ErrAuthentication},
		{"another vault under its own identifier", second.Session, "site-icons", relabeled, ErrAuthentication},
		{"another purpose", first.Session, "other-cache", sealed, ErrAuthentication},
		{"altered ciphertext", first.Session, "site-icons", tampered, ErrAuthentication},
		{"unknown suite", first.Session, "site-icons", unknownSuite, ErrUnsupported},
		{"trailing bytes", first.Session, "site-icons", append(append([]byte(nil), sealed...), 0), ErrMalformed},
		{"truncated", first.Session, "site-icons", sealed[:len(sealed)-1], ErrMalformed},
		{"empty", first.Session, "site-icons", nil, ErrMalformed},
		{"no purpose", first.Session, "", sealed, ErrInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.session.OpenDeviceCache(test.purpose, test.sealed); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestDeviceCacheSizeLimit(t *testing.T) {
	session, _ := populatedSession(t)
	defer session.Lock()
	if _, err := session.SealDeviceCache("site-icons", make([]byte, maxDeviceCacheBytes)); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("seal over the limit: %v", err)
	}
	oversized := encodeEnvelope(session.vaultID, sealedBox{ciphertext: make([]byte, maxDeviceCacheBytes+1)})
	if _, err := session.OpenDeviceCache("site-icons", oversized); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("open over the limit: %v", err)
	}
}

func TestLockedSessionRefusesDeviceCache(t *testing.T) {
	session, _ := populatedSession(t)
	sealed, err := session.SealDeviceCache("site-icons", []byte("icons"))
	if err != nil {
		t.Fatal(err)
	}
	session.Lock()
	if _, err := session.SealDeviceCache("site-icons", []byte("icons")); !errors.Is(err, ErrLocked) {
		t.Fatalf("seal: %v", err)
	}
	if _, err := session.OpenDeviceCache("site-icons", sealed); !errors.Is(err, ErrLocked) {
		t.Fatalf("open: %v", err)
	}
}
