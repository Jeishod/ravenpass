package vaultservice

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

// sharedFile is a vault file whose selection record survives a restart of the service.
type sharedFile struct {
	path      string
	selection string
}

func newSharedFile(t *testing.T) sharedFile {
	t.Helper()
	return sharedFile{
		path:      filepath.Join(t.TempDir(), localfile.DefaultVaultName),
		selection: filepath.Join(t.TempDir(), "storage.json"),
	}
}

func (f sharedFile) open(t *testing.T) *storage.Manager {
	t.Helper()
	manager, err := storage.NewManager(f.selection, storage.Target{Kind: storage.LocalFile, Path: f.path}, 256<<20, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	return manager
}

func (f sharedFile) remove(t *testing.T) {
	t.Helper()
	if err := os.Remove(f.path); err != nil {
		t.Fatal(err)
	}
}

func TestASaveAfterTheFileWentReportsItMissing(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	createTestVault(t, service)
	file.remove(t)

	_, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("saving without a file: got %v, want ErrVaultMissing", err)
	}
	if _, err := service.List(); !errors.Is(err, ErrNotReady) {
		t.Fatal("the vault stayed open without its file")
	}
	state, err := service.State()
	if err != nil || state.Phase != PhaseLocked || state.VaultExists || !state.VaultMissing {
		t.Fatalf("state without the file = %+v, error = %v", state, err)
	}
}

func TestOpeningAVaultWhoseFileWentReportsItMissing(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	phrase, _ := createTestVaultWith(t, service, MethodChoice{Biometry: true, PIN: testPIN})
	service.Lock()
	file.remove(t)
	if _, err := service.Unlock(testReason); !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("device authentication: got %v, want ErrVaultMissing", err)
	}
	if _, err := service.UnlockWithPIN(testPIN); !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("the PIN: got %v, want ErrVaultMissing", err)
	}
	if _, err := service.BeginRecovery(phrase); !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("recovery: got %v, want ErrVaultMissing", err)
	}
}

func TestAnEmptyFileWhereTheVaultWasOpenedReportsItMissingAndIsNotReplaced(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	createTestVault(t, service)
	service.Lock()
	if err := os.WriteFile(file.path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Unlock(testReason); !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("an empty file: got %v, want ErrVaultMissing", err)
	}
	if _, err := service.BeginCreation(); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("creating over an empty file where a vault was opened: got %v, want ErrAlreadyInitialized", err)
	}
}

func TestAnOpenVaultWaitsWhileItsFileIsEmpty(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	createTestVault(t, service)
	written, err := os.ReadFile(file.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file.path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if change := service.Follow(); change != FileUnchanged || !service.Unlocked() {
		t.Fatalf("a file rewritten in place: %v", change)
	}
	if err := os.WriteFile(file.path, written, 0o600); err != nil {
		t.Fatal(err)
	}
	if change := service.Follow(); change != FileUnchanged || !service.Unlocked() {
		t.Fatalf("the file back whole: %v", change)
	}
}

func TestANewVaultReplacesAnEmptyFile(t *testing.T) {
	file := newSharedFile(t)
	if err := os.WriteFile(file.path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, file.open(t), newMemoryKeys())
	if state, err := service.State(); err != nil || state.VaultExists || state.VaultMissing {
		t.Fatalf("an empty file reads as %+v, error = %v", state, err)
	}
	createTestVault(t, service)
}

func TestAChangedFileIsNotReportedMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), localfile.DefaultVaultName)
	keys := newMemoryKeys()
	device := newTestDevice()
	first := newTestServiceOn(t, newFileStorage(t, path), keys, device)
	createTestVault(t, first)
	second := newTestServiceOn(t, newFileStorage(t, path), keys, device)
	if _, err := second.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	if _, err := second.CreateCredential(vault.CredentialInput{Label: "Second", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	_, err := first.CreateCredential(vault.CredentialInput{Label: "First", Password: "secret"}, nil)
	if !errors.Is(err, vault.ErrWitnessMismatch) && !errors.Is(err, storage.ErrStaleHead) {
		t.Fatalf("saving over a changed file: got %v, want a stale head", err)
	}
	if errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("a changed file was reported missing: %v", err)
	}
}

func TestFollowingLocksAVaultWhoseFileWent(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	createTestVault(t, service)
	file.remove(t)
	if change := service.Follow(); change != FileRefused {
		t.Fatalf("following a file that went = %v, want FileRefused", change)
	}
	if service.Unlocked() {
		t.Fatal("the vault stayed open without its file")
	}
}

func TestExportAndMoveReportAFileThatWent(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	createTestVault(t, service)
	file.remove(t)
	if _, _, err := service.Export(); !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("exporting without a file: got %v, want ErrVaultMissing", err)
	}

	reopened := newSharedFile(t)
	moving := newTestService(t, reopened.open(t), newMemoryKeys())
	createTestVault(t, moving)
	reopened.remove(t)
	target := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(t.TempDir(), "moved.rpv")}
	if _, err := moving.MoveStorage(target); !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("moving without a file: got %v, want ErrVaultMissing", err)
	}
	if _, err := os.Stat(target.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a move without a file wrote the destination: %v", err)
	}
}

func TestALockedVaultWhoseFileWentIsReportedAfterARestart(t *testing.T) {
	file := newSharedFile(t)
	keys := newMemoryKeys()
	device := newTestDevice()
	service := newTestServiceOn(t, file.open(t), keys, device)
	_, head := createTestVault(t, service)
	service.Lock()
	file.remove(t)

	restarted := newTestServiceOn(t, file.open(t), keys, device)
	state, err := restarted.State()
	if err != nil || state.Phase != PhaseLocked || state.VaultExists || !state.VaultMissing {
		t.Fatalf("state after a restart = %+v, error = %v", state, err)
	}
	if missing, err := restarted.VaultMissing(); err != nil || !missing {
		t.Fatalf("missing after a restart = %t, error = %v", missing, err)
	}

	// Creation stays refused there, so the owner is offered the vault's file or forgetting the location.
	delete(keys.policy, head.VaultID.String())
	if missing, err := restarted.VaultMissing(); err != nil || !missing {
		t.Fatalf("missing without an unlock record = %t, error = %v", missing, err)
	}
	if _, err := restarted.BeginCreation(); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("creating where the vault was opened: got %v, want ErrAlreadyInitialized", err)
	}
}

func TestNoVaultIsCreatedWhereThisDeviceOpenedOne(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	createTestVault(t, service)
	service.Lock()
	file.remove(t)
	if _, err := service.BeginCreation(); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("creating where a vault's file is absent: got %v, want ErrAlreadyInitialized", err)
	}
	if _, err := os.Stat(file.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused creation wrote the file: %v", err)
	}

	other := newSharedFile(t)
	preparing := newTestService(t, other.open(t), newMemoryKeys())
	createTestVault(t, preparing)
	work := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(t.TempDir(), "work.rpv")}
	if err := preparing.PrepareVaultCreation(work); err != nil {
		t.Fatal(err)
	}
	other.remove(t)
	known := storage.Target{Kind: storage.LocalFile, Path: other.path}
	if err := preparing.PrepareVaultCreation(known); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("preparing where a vault's file is absent: got %v, want ErrAlreadyInitialized", err)
	}
	if _, current := preparing.Vaults(); !current.Same(work) {
		t.Fatalf("a refused preparation bound %+v", current)
	}
}

func TestALocationNeverOpenedIsNotMissing(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	state, err := service.State()
	if err != nil || state.Phase != PhaseLocked || state.VaultExists || state.VaultMissing {
		t.Fatalf("state of an unused location = %+v, error = %v", state, err)
	}
	if missing, err := service.VaultMissing(); err != nil || missing {
		t.Fatalf("missing at an unused location = %t, error = %v", missing, err)
	}
}

func TestAPresentFileIsNotMissing(t *testing.T) {
	file := newSharedFile(t)
	service := newTestService(t, file.open(t), newMemoryKeys())
	createTestVault(t, service)
	service.Lock()
	if missing, err := service.VaultMissing(); err != nil || missing {
		t.Fatalf("missing with the file in place = %t, error = %v", missing, err)
	}
}
