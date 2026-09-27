package vaultservice

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestDeviceCacheServesOnlyTheOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	if _, err := service.OpenVaultID(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("identifier before a vault opens: %v", err)
	}
	createTestVault(t, service)
	id, err := service.OpenVaultID()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := service.SealDeviceCache(id, "site-icons", []byte("icons"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := service.OpenDeviceCache(id, "site-icons", sealed)
	if err != nil || !bytes.Equal(opened, []byte("icons")) {
		t.Fatalf("opened %q, %v", opened, err)
	}
	other := vault.ID{0xff}
	if _, err := service.SealDeviceCache(other, "site-icons", []byte("icons")); !errors.Is(err, ErrNotReady) {
		t.Fatalf("seal for another vault: %v", err)
	}
	if _, err := service.OpenDeviceCache(other, "site-icons", sealed); !errors.Is(err, ErrNotReady) {
		t.Fatalf("open for another vault: %v", err)
	}
	service.Lock()
	if _, err := service.OpenDeviceCache(id, "site-icons", sealed); !errors.Is(err, ErrNotReady) {
		t.Fatalf("open after lock: %v", err)
	}
	if _, err := service.SealDeviceCache(id, "site-icons", []byte("icons")); !errors.Is(err, ErrNotReady) {
		t.Fatalf("seal after lock: %v", err)
	}
}
