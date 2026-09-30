package vaultservice

import (
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

func target(path string) storage.Target {
	return storage.Target{Kind: storage.LocalFile, Path: path}
}

func TestSwitchVaultClosesTheOpenVault(t *testing.T) {
	personal := target("/vaults/personal.rpv")
	work := target("/vaults/work.rpv")
	files := &memoryFiles{target: personal}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := service.SwitchVault(work); err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(); !errors.Is(err, ErrNotReady) {
		t.Fatal("switching left the previous vault open")
	}
	state, err := service.State()
	if err != nil || state.Phase != PhaseLocked || state.VaultExists {
		t.Fatalf("state at the new location: %+v, error = %v", state, err)
	}
	vaults, current := service.Vaults()
	if len(vaults) != 2 || current != work {
		t.Fatalf("known vaults = %+v, current = %+v", vaults, current)
	}

	createTestVault(t, service)
	if err := service.SwitchVault(personal); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	entries, err := service.List()
	if err != nil || len(entries) != 1 || entries[0].Label != "Mail" {
		t.Fatalf("entries after switching back = %+v, error = %v", entries, err)
	}
}

func TestPrepareVaultCreationRefusesAnOccupiedLocation(t *testing.T) {
	personal := target("/vaults/personal.rpv")
	files := &memoryFiles{target: personal}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)

	if err := service.PrepareVaultCreation(personal); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("creating over an existing vault: got %v, want ErrAlreadyInitialized", err)
	}
	if _, err := service.List(); err != nil {
		t.Fatalf("a refused creation closed the open vault: %v", err)
	}

	work := target("/vaults/work.rpv")
	if err := service.PrepareVaultCreation(work); err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(); !errors.Is(err, ErrNotReady) {
		t.Fatal("preparing a new vault left the previous one open")
	}
	state, err := service.State()
	if err != nil || state.VaultExists {
		t.Fatalf("state at a prepared location: %+v, error = %v", state, err)
	}
	createTestVault(t, service)
}

func TestHoldsAnyVaultLooksAtEveryKnownLocation(t *testing.T) {
	files := &memoryFiles{target: target("/vaults/default.rpv")}
	service := newTestService(t, files, newMemoryKeys())
	if service.HoldsAnyVault() {
		t.Fatal("a fresh device reports a vault")
	}

	if err := service.PrepareVaultCreation(target("/vaults/work.rpv")); err != nil {
		t.Fatal(err)
	}
	if service.HoldsAnyVault() {
		t.Fatal("two locations without a file report a vault")
	}

	createTestVault(t, service)
	if err := service.PrepareVaultCreation(target("/vaults/third.rpv")); err != nil {
		t.Fatal(err)
	}
	if !service.HoldsAnyVault() {
		t.Fatal("a vault at a location that is not current went unnoticed")
	}
}

func TestCancellingAnotherVaultReturnsToTheVaultBefore(t *testing.T) {
	personal := target("/vaults/personal.rpv")
	work := target("/vaults/work.rpv")
	chosen := target("/vaults/chosen.rpv")
	files := &memoryFiles{target: personal}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)

	if free, err := service.FreeLocation(personal); err != nil || free {
		t.Fatalf("the open vault's location is free = %v, error = %v", free, err)
	}
	if free, err := service.FreeLocation(work); err != nil || !free {
		t.Fatalf("an unlisted empty location is free = %v, error = %v", free, err)
	}
	if err := service.PrepareVaultCreation(work); err != nil {
		t.Fatal(err)
	}
	if err := service.PrepareVaultCreation(chosen); err != nil {
		t.Fatal(err)
	}
	if err := service.CancelVaultCreation(); err != nil {
		t.Fatal(err)
	}
	if _, current := service.Vaults(); current != personal {
		t.Fatalf("after cancelling, the current vault is %+v", current)
	}
	state, err := service.State()
	if err != nil || state.Phase != PhaseLocked || !state.VaultExists {
		t.Fatalf("state after cancelling = %+v, error = %v", state, err)
	}
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatalf("the vault returned to did not open: %v", err)
	}
}

func TestCancellingIsRefusedWithoutAnotherVaultOrOnceWritten(t *testing.T) {
	files := &memoryFiles{target: target("/vaults/default.rpv")}
	service := newTestService(t, files, newMemoryKeys())
	if err := service.CancelVaultCreation(); !errors.Is(err, ErrNoOtherVault) {
		t.Fatalf("cancelling the only vault's setup: got %v, want ErrNoOtherVault", err)
	}
	createTestVault(t, service)
	if err := service.CancelVaultCreation(); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("cancelling after the vault was written: got %v, want ErrAlreadyInitialized", err)
	}
}

func TestForgetVaultKeepsTheFileAndTheDeviceRecords(t *testing.T) {
	personal := target("/vaults/personal.rpv")
	work := target("/vaults/work.rpv")
	files := &memoryFiles{target: personal}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)
	if err := service.SwitchVault(work); err != nil {
		t.Fatal(err)
	}
	createTestVault(t, service)

	if err := service.ForgetVault(personal); err != nil {
		t.Fatal(err)
	}
	vaults, current := service.Vaults()
	if len(vaults) != 1 || current != work {
		t.Fatalf("known vaults after forgetting = %+v, current = %+v", vaults, current)
	}
	id := head.VaultID.String()
	if _, exists := keys.policy[id]; !exists {
		t.Fatal("forgetting deleted the device unlock record")
	}
	if data, err := files.LoadFrom(personal); err != nil || data == nil {
		t.Fatalf("forgetting removed the vault file: %v", err)
	}

	if err := service.SwitchVault(personal); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatalf("a forgotten vault could not be reopened: %v", err)
	}
}

func TestDeleteVaultErasesTheFileAndEveryDeviceRecord(t *testing.T) {
	personal := target("/vaults/personal.rpv")
	work := target("/vaults/work.rpv")
	files := &memoryFiles{target: personal}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := service.SetPIN(openingOf(service), testPIN); err != nil {
		t.Fatal(err)
	}
	if err := service.SwitchVault(work); err != nil {
		t.Fatal(err)
	}
	_, workHead := createTestVault(t, service)

	deletion, err := service.DeleteVault(personal)
	if err != nil {
		t.Fatal(err)
	}
	if deletion.Target != personal || !deletion.DeviceRecordsRemoved {
		t.Fatalf("deletion = %+v", deletion)
	}
	id := head.VaultID.String()
	for name, records := range map[string]map[string][]byte{
		"head witness":  keys.witness,
		"usage record":  keys.usage,
		"export record": keys.export,
		"unlock record": keys.policy,
	} {
		if _, exists := records[id]; exists {
			t.Errorf("%s survived the deletion", name)
		}
	}
	if _, exists := keys.policy[workHead.VaultID.String()]; !exists {
		t.Fatal("deleting one vault removed another vault's unlock record")
	}
	if _, err := files.LoadFrom(personal); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted vault file remains: %v", err)
	}
	vaults, current := service.Vaults()
	if len(vaults) != 1 || current != work {
		t.Fatalf("known vaults after deleting = %+v, current = %+v", vaults, current)
	}
	if _, err := service.List(); err != nil {
		t.Fatalf("deleting another vault closed the open one: %v", err)
	}
}

func TestDeleteVaultClosesTheVaultItDeletes(t *testing.T) {
	personal := target("/vaults/personal.rpv")
	files := &memoryFiles{target: personal}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)

	if _, err := service.DeleteVault(personal); err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(); !errors.Is(err, ErrNotReady) {
		t.Fatal("deleting the current vault left it open")
	}
	if _, exists := keys.witness[head.VaultID.String()]; exists {
		t.Fatal("the deleted vault's witness survived")
	}
	state, err := service.State()
	if err != nil || state.VaultExists {
		t.Fatalf("state after deleting the only vault: %+v, error = %v", state, err)
	}
}

func TestDeleteVaultRefusesAnUnreadableContainer(t *testing.T) {
	personal := target("/vaults/personal.rpv")
	missing := target("/vaults/missing.rpv")
	files := &memoryFiles{target: personal}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	if err := service.BindStorage(missing); err == nil {
		// Binding an unknown empty location is allowed; the vault stays open otherwise.
		t.Log("binding an empty location succeeded")
	}

	if _, err := service.DeleteVault(missing); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleting a vault without a container: got %v, want ErrNotFound", err)
	}
}
