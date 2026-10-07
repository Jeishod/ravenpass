package vault

import (
	"bytes"
	"errors"
	"testing"
)

func TestDeviceDataRoundTripsAndSurvivesAReopen(t *testing.T) {
	created, _ := vaultWith(t)
	plaintext := []byte(`{"history":[{"value":"hunter2"}]}`)
	sealed, err := created.Session.SealDeviceData("generator-history", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("hunter2")) {
		t.Fatal("the sealed data carries its plaintext")
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
	if opened, err := reopened.OpenDeviceData("generator-history", sealed); err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("opened after a reopen %q, %v", opened, err)
	}
}

func TestDeviceDataAndDeviceCacheNeverOpenEachOther(t *testing.T) {
	created, _ := vaultWith(t)
	data, err := created.Session.SealDeviceData("name", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	cache, err := created.Session.SealDeviceCache("name", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := created.Session.OpenDeviceCache("name", data); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("a device cache opened device data: %v", err)
	}
	if _, err := created.Session.OpenDeviceData("name", cache); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("device data opened a device cache: %v", err)
	}
}

func TestDeviceDataRefusesAnotherVaultOrName(t *testing.T) {
	created, _ := vaultWith(t)
	other, _ := vaultWith(t)
	sealed, err := created.Session.SealDeviceData("name", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := created.Session.OpenDeviceData("other", sealed); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("another name opened the data: %v", err)
	}
	if _, err := other.Session.OpenDeviceData("name", sealed); err == nil {
		t.Fatal("another vault opened the data")
	}
	if _, err := created.Session.SealDeviceData("", nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an empty name sealed: %v", err)
	}
}

func TestARekeySealsDeviceDataForTheNewKey(t *testing.T) {
	created, _ := vaultWith(t)
	session := created.Session
	old, err := session.SealDeviceData("name", []byte("before"))
	if err != nil {
		t.Fatal(err)
	}
	rekey, err := session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	resealed, err := rekey.SealDeviceData("name", []byte("before"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.OpenDeviceData("name", resealed); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("data sealed for the new key opened before the rekey committed: %v", err)
	}
	pending, err := session.PrepareRekey(rekey)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if opened, err := session.OpenDeviceData("name", resealed); err != nil || string(opened) != "before" {
		t.Fatalf("opened after the rekey %q, %v", opened, err)
	}
	if _, err := session.OpenDeviceData("name", old); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("data sealed for the replaced key still opened: %v", err)
	}
}

func TestLockedSessionAndDiscardedRekeyRefuseDeviceData(t *testing.T) {
	created, _ := vaultWith(t)
	rekey, err := created.Session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	rekey.Discard()
	if _, err := rekey.SealDeviceData("name", nil); !errors.Is(err, ErrLocked) {
		t.Fatalf("a discarded rekey sealed: %v", err)
	}
	created.Session.Lock()
	if _, err := created.Session.SealDeviceData("name", nil); !errors.Is(err, ErrLocked) {
		t.Fatalf("a locked session sealed: %v", err)
	}
	if _, err := created.Session.OpenDeviceData("name", nil); !errors.Is(err, ErrLocked) {
		t.Fatalf("a locked session opened: %v", err)
	}
}
