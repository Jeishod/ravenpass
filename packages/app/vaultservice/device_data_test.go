package vaultservice

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

// heldData keeps one sealed value in memory as a holder of device data keeps a file.
type heldData struct {
	sealed  []byte
	failing bool
	written int
}

func (h *heldData) ResealDeviceData(_ vault.ID, open, seal func(string, []byte) ([]byte, error)) (func() error, error) {
	if h.failing {
		return nil, errors.New("reseal failed")
	}
	plaintext, err := open("held", h.sealed)
	if err != nil {
		return nil, err
	}
	resealed, err := seal("held", plaintext)
	if err != nil {
		return nil, err
	}
	return func() error {
		h.sealed = resealed
		h.written++
		return nil
	}, nil
}

func TestAKeyChangeSealsDeviceDataAgainOnceTheFileHoldsTheNewKey(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	_, head := createTestVault(t, service)
	sealed, err := service.SealDeviceData(head.VaultID, "held", []byte("generated"))
	if err != nil {
		t.Fatal(err)
	}
	held := &heldData{sealed: sealed}
	failing := &heldData{sealed: bytes.Clone(sealed), failing: true}
	service.KeepDeviceData(failing)
	service.KeepDeviceData(held)
	phrase, err := service.BeginRekey(openingOf(service), "")
	if err != nil {
		t.Fatal(err)
	}
	if held.written != 0 {
		t.Fatal("device data was written before the key change was confirmed")
	}
	if _, err := service.ConfirmRekey(phrase); err != nil {
		t.Fatalf("a holder that failed stopped the key change: %v", err)
	}
	if held.written != 1 {
		t.Fatalf("written %d times, want once", held.written)
	}
	opened, err := service.OpenDeviceData(head.VaultID, "held", held.sealed)
	if err != nil || string(opened) != "generated" {
		t.Fatalf("opened %q, %v after the key change", opened, err)
	}
	if _, err := service.OpenDeviceData(head.VaultID, "held", failing.sealed); !errors.Is(err, vault.ErrAuthentication) {
		t.Fatalf("data left under the replaced key: got %v, want ErrAuthentication", err)
	}
}

func TestADiscardedKeyChangeWritesNoDeviceData(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	_, head := createTestVault(t, service)
	sealed, err := service.SealDeviceData(head.VaultID, "held", []byte("generated"))
	if err != nil {
		t.Fatal(err)
	}
	held := &heldData{sealed: sealed}
	service.KeepDeviceData(held)
	if _, err := service.BeginRekey(openingOf(service), ""); err != nil {
		t.Fatal(err)
	}
	service.DiscardRekey()
	if held.written != 0 {
		t.Fatal("a discarded key change wrote device data")
	}
	if _, err := service.OpenDeviceData(head.VaultID, "held", held.sealed); err != nil {
		t.Fatalf("device data no longer opens: %v", err)
	}
}
