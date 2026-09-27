package vaultservice

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

func TestSwitchingBetweenRealVaultsKeepsBothUnlockable(t *testing.T) {
	home := t.TempDir()
	first := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, localfile.DefaultVaultName)}
	second := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, "work.rpv")}
	files, err := storage.NewManager(filepath.Join(home, "storage.json"), first, 256<<20, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, files, newMemoryKeys())

	createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := service.PrepareVaultCreation(second); err != nil {
		t.Fatal(err)
	}
	createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Work", Password: "other"}, nil); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := service.SwitchVault(first); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Unlock(testReason); err != nil {
			t.Fatalf("unlocking the first vault: %v", err)
		}
		entries, err := service.List()
		if err != nil || len(entries) != 1 || entries[0].Label != "Mail" {
			t.Fatalf("first vault entries = %+v, error = %v", entries, err)
		}

		if err := service.SwitchVault(second); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Unlock(testReason); err != nil {
			t.Fatalf("unlocking the second vault: %v", err)
		}
		entries, err = service.List()
		if err != nil || len(entries) != 1 || entries[0].Label != "Work" {
			t.Fatalf("second vault entries = %+v, error = %v", entries, err)
		}
	}
}

func TestOpeningASecondCopyOfAKnownVaultIsRefused(t *testing.T) {
	home := t.TempDir()
	live := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, localfile.DefaultVaultName)}
	files, err := storage.NewManager(filepath.Join(home, "storage.json"), live, 256<<20, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}

	// An encrypted export is a complete copy of the same vault.
	container, _, err := service.Export()
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(home, "Ravenpass vault.rpv")
	if err := os.WriteFile(copyPath, container, 0600); err != nil {
		t.Fatal(err)
	}
	copied := storage.Target{Kind: storage.LocalFile, Path: copyPath}

	if err := service.SwitchVault(copied); !errors.Is(err, ErrSecondCopy) {
		t.Fatalf("adopting a copy of a known vault: got %v, want ErrSecondCopy", err)
	}
	vaults, current := service.Vaults()
	if len(vaults) != 1 || !current.Same(live) {
		t.Fatalf("known vaults = %+v, current = %+v", vaults, current)
	}
	if _, err := service.List(); err != nil {
		t.Fatalf("a refused adoption closed the open vault: %v", err)
	}
}

func TestOlderCopyExplainsItselfAtUnlock(t *testing.T) {
	home := t.TempDir()
	live := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, localfile.DefaultVaultName)}
	files, err := storage.NewManager(filepath.Join(home, "storage.json"), live, 256<<20, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	older, _, err := service.Export()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	service.Lock()

	if err := os.WriteFile(live.Path, older, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Unlock(testReason); !errors.Is(err, ErrOlderCopy) {
		t.Fatalf("unlocking an older copy: got %v, want ErrOlderCopy", err)
	}
}

func TestMovedVaultUnlocksAfterALock(t *testing.T) {
	home := t.TempDir()
	first := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, localfile.DefaultVaultName)}
	files, err := storage.NewManager(filepath.Join(home, "storage.json"), first, 256<<20, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}

	moved := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(t.TempDir(), "moved.rpv")}
	if _, err := service.MoveStorage(moved); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Calendar", Password: "another"}, nil); err != nil {
		t.Fatalf("saving after a move: %v", err)
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatalf("unlocking after a move: %v", err)
	}
	entries, err := service.List()
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries after a move = %+v, error = %v", entries, err)
	}
}

func TestExportAfterSwitchingVaults(t *testing.T) {
	home := t.TempDir()
	first := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, localfile.DefaultVaultName)}
	second := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, "work.rpv")}
	files, err := storage.NewManager(filepath.Join(home, "storage.json"), first, 256<<20, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	id, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.PrepareVaultCreation(second); err != nil {
		t.Fatal(err)
	}
	createTestVault(t, service)
	if err := service.SwitchVault(first); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}

	ticket, err := service.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadSelected(ticket); err != nil {
		t.Fatal(err)
	}
	if err := service.MarkUsed(id); err != nil {
		t.Fatal(err)
	}
	for attempt := range 3 {
		data, head, err := service.Export()
		if err != nil {
			t.Fatalf("export %d after switching: %v", attempt, err)
		}
		if err := service.RecordExport(head, sha256.Sum256(data)); err != nil {
			t.Fatalf("recording export %d: %v", attempt, err)
		}
	}
}

func TestDeletingACopyKeepsTheKeysOfTheVaultItShares(t *testing.T) {
	home := t.TempDir()
	live := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(home, localfile.DefaultVaultName)}
	files, err := storage.NewManager(filepath.Join(home, "storage.json"), live, 256<<20, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}

	// A file copied outside Ravenpass can put a copy of the live vault beside it in the list.
	container, _, err := service.Export()
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(home, "Ravenpass vault.rpv")
	if err := os.WriteFile(copyPath, container, 0600); err != nil {
		t.Fatal(err)
	}
	copied := storage.Target{Kind: storage.LocalFile, Path: copyPath}
	if err := files.Bind(copied); err != nil {
		t.Fatal(err)
	}
	if err := files.Bind(live); err != nil {
		t.Fatal(err)
	}

	deletion, err := service.DeleteVault(copied)
	if err != nil {
		t.Fatal(err)
	}
	if !deletion.SharedIdentity || deletion.DeviceRecordsRemoved {
		t.Fatalf("deleting a copy = %+v", deletion)
	}
	if _, exists := keys.policy[head.VaultID.String()]; !exists {
		t.Fatal("deleting a copy removed the live vault's unlock record")
	}
	if _, err := os.Stat(copyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the copy's file remains: %v", err)
	}
	if _, _, err := service.Export(); err != nil {
		t.Fatalf("exporting after deleting a copy: %v", err)
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatalf("unlocking after deleting a copy: %v", err)
	}
}
