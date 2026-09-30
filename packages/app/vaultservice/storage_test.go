package vaultservice

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

func TestMoveStorageCarriesTheVaultToTheNewLocation(t *testing.T) {
	files := &memoryFiles{target: storage.Target{Kind: storage.LocalFile, Path: "/vaults/first.rpv"}}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(files.data)

	target := storage.Target{Kind: storage.LocalFile, Path: "/vaults/second.rpv"}
	relocation, err := service.MoveStorage(target)
	if err != nil {
		t.Fatal(err)
	}
	if relocation.Target != target || relocation.Previous.Path != "/vaults/first.rpv" || !relocation.PreviousRemoved {
		t.Fatalf("relocation = %+v", relocation)
	}
	if !bytes.Equal(files.data, before) {
		t.Fatal("moved vault differs from the vault that was open")
	}
	if status := service.Storage(); status.Current.Path != target.Path {
		t.Fatalf("storage after the move = %+v", status)
	}
	entries, err := service.List()
	if err != nil || len(entries) != 1 || entries[0].Label != "Mail" {
		t.Fatalf("entries after the move = %+v, error = %v", entries, err)
	}
	reopenTestVault(t, service)
	if entries, err = service.List(); err != nil || len(entries) != 1 {
		t.Fatalf("entries after reopening = %+v, error = %v", entries, err)
	}
}

func TestMoveStorageRefusesWhatItCannotProve(t *testing.T) {
	target := storage.Target{Kind: storage.LocalFile, Path: "/vaults/second.rpv"}

	locked := newTestService(t, &memoryFiles{}, newMemoryKeys())
	if _, err := locked.MoveStorage(target); !errors.Is(err, ErrNotReady) {
		t.Fatalf("locked vault: got %v, want ErrNotReady", err)
	}

	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	replaced := bytes.Clone(files.data)
	replaced[len(replaced)-1] ^= 0xff
	files.data = replaced
	if _, err := service.MoveStorage(target); !errors.Is(err, storage.ErrStaleHead) {
		t.Fatalf("changed vault file: got %v, want ErrStaleHead", err)
	}
	if _, err := service.List(); !errors.Is(err, ErrNotReady) {
		t.Fatal("a refused move left the vault open")
	}

	witnessless := &memoryFiles{}
	keys := newMemoryKeys()
	other := newTestService(t, witnessless, keys)
	_, head := createTestVault(t, other)
	delete(keys.witness, head.VaultID.String())
	if _, err := other.MoveStorage(target); !errors.Is(err, vault.ErrWitnessMismatch) {
		t.Fatalf("missing witness: got %v, want ErrWitnessMismatch", err)
	}
}

func TestMoveStorageKeepsTheVaultWhenTheDestinationRefuses(t *testing.T) {
	refused := errors.New("destination refused the vault")
	files := &memoryFiles{
		target:        storage.Target{Kind: storage.LocalFile, Path: "/vaults/first.rpv"},
		relocateError: refused,
	}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	if _, err := service.MoveStorage(storage.Target{Kind: storage.LocalFile, Path: "/vaults/second.rpv"}); !errors.Is(err, refused) {
		t.Fatalf("refused destination: got %v, want the destination's error", err)
	}
	if status := service.Storage(); status.Current.Path != "/vaults/first.rpv" {
		t.Fatalf("storage after a refused move = %+v", status)
	}
	if _, err := service.List(); err != nil {
		t.Fatalf("a refused move closed the vault: %v", err)
	}
	if len(files.discarded) != 1 || files.discarded[0].Path != "/vaults/second.rpv" {
		t.Fatalf("a failed move left its destination: discarded %+v", files.discarded)
	}
}

func TestBindStorageOnlyBeforeAVaultIsOpen(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	chosen := storage.Target{Kind: storage.LocalFile, Path: "/vaults/chosen.rpv"}
	if err := service.BindStorage(chosen); err != nil {
		t.Fatal(err)
	}
	if status := service.Storage(); status.Current.Path != chosen.Path {
		t.Fatalf("storage after binding = %+v", status)
	}
	createTestVault(t, service)
	if err := service.BindStorage(storage.Target{Kind: storage.LocalFile, Path: "/vaults/other.rpv"}); !errors.Is(err, ErrSetupInProgress) {
		t.Fatalf("binding with an open vault: got %v, want ErrSetupInProgress", err)
	}
	if status := service.Storage(); status.Current.Path != chosen.Path {
		t.Fatalf("a refused bind changed storage: %+v", status)
	}
}

func TestMovedVaultOpensFromItsNewFile(t *testing.T) {
	first := filepath.Join(t.TempDir(), localfile.DefaultVaultName)
	files := newFileStorage(t, first)
	keys := newMemoryKeys()
	device := newTestDevice()
	service := newTestServiceOn(t, files, keys, device)
	createTestVault(t, service)
	if _, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}

	second := filepath.Join(t.TempDir(), "moved.rpv")
	if _, err := service.MoveStorage(storage.Target{Kind: storage.LocalFile, Path: second}); err != nil {
		t.Fatal(err)
	}
	service.Lock()

	reopened := newFileStorage(t, second)
	next := newTestServiceOn(t, reopened, keys, device)
	if _, err := next.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	entries, err := next.List()
	if err != nil || len(entries) != 1 || entries[0].Label != "Mail" {
		t.Fatalf("entries at the new location = %+v, error = %v", entries, err)
	}
}
