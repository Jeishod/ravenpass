package storage_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

func vaultAt(t *testing.T, name string) storage.Target {
	t.Helper()
	return local(filepath.Join(t.TempDir(), name))
}

func TestManagerRefusesASelectionOfAnotherVersion(t *testing.T) {
	manager, selectionPath, _ := newTestManager(t)
	kept := filepath.Join(t.TempDir(), "kept.rpv")
	record := `{"version":1,"kind":"local-file","path":"` + kept + `"}`
	if err := os.WriteFile(selectionPath, []byte(record), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Open(); !errors.Is(err, storage.ErrSelectionInvalid) {
		t.Fatalf("a selection of another version: got %v, want ErrSelectionInvalid", err)
	}
	if status := manager.Status(); status.Available || status.Reason != storage.ReasonSelectionUnusable {
		t.Fatalf("status from a selection of another version: %+v", status)
	}
}

func TestManagerKeepsEveryKnownVault(t *testing.T) {
	manager, selectionPath, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x01})

	work := vaultAt(t, "work.rpv")
	if err := manager.Bind(work); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x02})
	if err := manager.Bind(work); err != nil {
		t.Fatal(err)
	}
	status := manager.Status()
	if len(status.Vaults) != 2 || status.Current != work {
		t.Fatalf("known vaults: %+v", status)
	}

	reopened := newManager(t, selectionPath, defaultPath)
	if err := reopened.Open(); err != nil {
		t.Fatal(err)
	}
	status = reopened.Status()
	if len(status.Vaults) != 2 || status.Current != work {
		t.Fatalf("known vaults after a restart: %+v", status)
	}
	loaded, err := reopened.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, []byte{0x02}) {
		t.Fatalf("current vault after a restart: %x, %v", loaded, err)
	}
	personal, err := reopened.LoadFrom(local(defaultPath))
	if err != nil || !bytes.Equal(personal, []byte{0x01}) {
		t.Fatalf("other vault read without binding: %x, %v", personal, err)
	}
}

func TestManagerListsOnlyVaultsAndTheCurrentLocation(t *testing.T) {
	manager, selectionPath, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	copied := vaultAt(t, "copy.rpv")
	if err := os.WriteFile(copied.Path, []byte{0x05}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Bind(copied); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); len(status.Vaults) != 1 || status.Vaults[0] != copied {
		t.Fatalf("opening a vault kept the empty default location: %+v", status)
	}

	proposed := vaultAt(t, "proposed.rpv")
	if err := manager.Bind(proposed); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); len(status.Vaults) != 2 || status.Current != proposed {
		t.Fatalf("a location being set up left the list: %+v", status)
	}
	if err := manager.Bind(copied); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); len(status.Vaults) != 1 || status.Vaults[0] != copied {
		t.Fatalf("an abandoned location stayed listed: %+v", status)
	}

	stale := `{"version":2,"current":"` + copied.Path + `","vaults":[{"kind":"local-file","path":"` +
		defaultPath + `"},{"kind":"local-file","path":"` + copied.Path + `"}]}`
	if err := os.WriteFile(selectionPath, []byte(stale), 0600); err != nil {
		t.Fatal(err)
	}
	reopened := newManager(t, selectionPath, defaultPath)
	if err := reopened.Open(); err != nil {
		t.Fatal(err)
	}
	if status := reopened.Status(); len(status.Vaults) != 1 || status.Vaults[0] != copied {
		t.Fatalf("a record listing an empty location was not tidied: %+v", status)
	}
}

func TestManagerKeepsAVaultOpenedAtALocationThatReadsAbsent(t *testing.T) {
	manager, selectionPath, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	work := vaultAt(t, "work.rpv")
	if err := manager.Bind(work); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x01})
	if err := manager.Identify("work-vault"); err != nil {
		t.Fatal(err)
	}
	work.Vault = "work-vault"
	personal := vaultAt(t, "personal.rpv")
	if err := manager.Bind(personal); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x02})
	if err := os.Remove(work.Path); err != nil {
		t.Fatal(err)
	}

	proposed := vaultAt(t, "proposed.rpv")
	if err := manager.Bind(proposed); err != nil {
		t.Fatal(err)
	}
	if err := manager.Bind(personal); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); !slices.Equal(status.Vaults, []storage.Target{work, personal}) {
		t.Fatalf("known vaults after binding past an absent file: %+v", status.Vaults)
	}

	reopened := newManager(t, selectionPath, defaultPath)
	if err := reopened.Open(); err != nil {
		t.Fatal(err)
	}
	if status := reopened.Status(); !slices.Equal(status.Vaults, []storage.Target{work, personal}) {
		t.Fatalf("known vaults after a restart with an absent file: %+v", status.Vaults)
	}
}

func TestManagerForgetKeepsTheFileAndBindsTheNextVault(t *testing.T) {
	manager, _, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x01})
	work := vaultAt(t, "work.rpv")
	if err := manager.Bind(work); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x02})

	if err := manager.Forget(work); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(work.Path); err != nil {
		t.Fatalf("forgetting removed the vault file: %v", err)
	}
	status := manager.Status()
	if len(status.Vaults) != 1 || status.Current.Path != defaultPath || !status.Available {
		t.Fatalf("status after forgetting the current vault: %+v", status)
	}
	loaded, err := manager.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, []byte{0x01}) {
		t.Fatalf("remaining vault: %x, %v", loaded, err)
	}
	if err := manager.Forget(work); !errors.Is(err, storage.ErrUnknownVault) {
		t.Fatalf("forgetting an unknown vault: got %v, want ErrUnknownVault", err)
	}
}

func TestManagerForgetTheOnlyVaultFallsBackToTheDefaultLocation(t *testing.T) {
	manager, _, defaultPath := newTestManager(t)
	only := vaultAt(t, "only.rpv")
	if err := manager.Bind(only); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x07})
	if err := manager.Forget(only); err != nil {
		t.Fatal(err)
	}
	status := manager.Status()
	if len(status.Vaults) != 1 || status.Vaults[0].Path != defaultPath || !status.Available {
		t.Fatalf("status after forgetting the only vault: %+v", status)
	}
	if _, err := manager.LoadCiphertext(); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("default location after forgetting: got %v, want ErrNotFound", err)
	}
}

func TestManagerEraseRemovesTheVaultFile(t *testing.T) {
	manager, _, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x01})
	work := vaultAt(t, "work.rpv")
	if err := manager.Bind(work); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x02})

	if err := manager.Erase(local(defaultPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(defaultPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("erased vault file remains: %v", err)
	}
	status := manager.Status()
	if len(status.Vaults) != 1 || status.Current != work {
		t.Fatalf("status after erasing another vault: %+v", status)
	}
	loaded, err := manager.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, []byte{0x02}) {
		t.Fatalf("current vault after erasing another: %x, %v", loaded, err)
	}

	if err := manager.Erase(work); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(work.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("erased current vault file remains: %v", err)
	}
	status = manager.Status()
	if len(status.Vaults) != 1 || status.Vaults[0].Path != defaultPath {
		t.Fatalf("status after erasing the last vault: %+v", status)
	}
}

func TestManagerRelocateKeepsOneEntryPerVault(t *testing.T) {
	manager, _, _ := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	ciphertext := []byte{0x31}
	write(t, manager, ciphertext)
	moved := vaultAt(t, "moved.rpv")
	if _, err := manager.Relocate(moved, func(destination storage.Ciphertext) error {
		return destination.CommitCiphertext(nil, ciphertext, func() error { return nil })
	}); err != nil {
		t.Fatal(err)
	}
	status := manager.Status()
	if len(status.Vaults) != 1 || status.Vaults[0] != moved {
		t.Fatalf("known vaults after a move: %+v", status)
	}

	work := vaultAt(t, "work.rpv")
	if err := manager.Bind(work); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x41})
	if _, err := manager.Relocate(moved, func(storage.Ciphertext) error { return nil }); !errors.Is(err, storage.ErrTargetOccupied) {
		t.Fatalf("moving onto a known vault: got %v, want ErrTargetOccupied", err)
	}
}

// documents is a document provider in memory: a document exists empty once a picker created it.
type documents struct {
	files     map[string][]byte
	discarded []string
}

func (*documents) Kind() storage.Kind { return storage.Document }

func (*documents) Check(path string) error {
	if !strings.HasPrefix(path, "content://") {
		return storage.ErrInvalidPath
	}
	return nil
}

func (provider *documents) Open(target storage.Target, _ int64) (storage.Store, error) {
	return &document{provider: provider, path: target.Path}, nil
}

type document struct {
	provider *documents
	path     string
}

func (d *document) LoadCiphertext() ([]byte, error) {
	data, found := d.provider.files[d.path]
	if !found {
		return nil, errors.New("document is gone")
	}
	if len(data) == 0 {
		return nil, storage.ErrNotFound
	}
	return slices.Clone(data), nil
}

func (d *document) CommitCiphertext(expected *[sha256.Size]byte, candidate []byte, finalize func() error) error {
	current, err := d.LoadCiphertext()
	if err := storage.CheckHead(expected, current, err); err != nil {
		return err
	}
	d.provider.files[d.path] = slices.Clone(candidate)
	return storage.Finish(finalize)
}

func (d *document) ReconcileCiphertext(expected [sha256.Size]byte, finalize func() error) error {
	current, err := d.LoadCiphertext()
	if err := storage.CheckHead(&expected, current, err); err != nil {
		return err
	}
	return storage.Finish(finalize)
}

func (*document) Restricted() bool { return false }

func (d *document) Remove() error {
	delete(d.provider.files, d.path)
	return nil
}

func (d *document) DiscardEmpty() error {
	d.provider.discarded = append(d.provider.discarded, d.path)
	return d.Remove()
}

func TestManagerRecordsHowADocumentIsNamed(t *testing.T) {
	home := t.TempDir()
	selectionPath := filepath.Join(home, "storage.json")
	defaultPath := filepath.Join(home, "Ravenpass", "vault.rpv")
	provider := &documents{files: map[string][]byte{"content://drive/1": nil}}
	manager := newManager(t, selectionPath, defaultPath, provider)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	if kinds := manager.Status().Kinds; !slices.Equal(kinds, []storage.Kind{storage.LocalFile, storage.Document}) {
		t.Fatalf("offered kinds: %v", kinds)
	}

	chosen := storage.Target{
		Kind:  storage.Document,
		Path:  "content://drive/1",
		Label: storage.Label{Name: "Personal.rpv", Place: "Drive"},
	}
	if err := manager.Bind(chosen); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x01})
	if err := manager.Bind(storage.Target{Kind: storage.Document, Path: chosen.Path}); err != nil {
		t.Fatal(err)
	}
	if current := manager.Status().Current; current != chosen {
		t.Fatalf("binding a known document without its name: %+v", current)
	}
	renamed := chosen
	renamed.Label.Name = "Family.rpv"
	if err := manager.Bind(renamed); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); len(status.Vaults) != 1 || status.Vaults[0] != renamed {
		t.Fatalf("a document renamed at its provider: %+v", status)
	}

	reopened := newManager(t, selectionPath, defaultPath, provider)
	if err := reopened.Open(); err != nil {
		t.Fatal(err)
	}
	if current := reopened.Status().Current; current != renamed {
		t.Fatalf("document after a restart: %+v", current)
	}
	if err := reopened.Bind(storage.Target{Kind: storage.Document, Path: "/sdcard/vault.rpv"}); !errors.Is(err, storage.ErrInvalidPath) {
		t.Fatalf("a document without an address: got %v, want ErrInvalidPath", err)
	}
}

func TestManagerDiscardsAnEmptyDocumentItLeaves(t *testing.T) {
	home := t.TempDir()
	defaultPath := filepath.Join(home, "Ravenpass", "vault.rpv")
	provider := &documents{files: map[string][]byte{"content://drive/2": nil}}
	manager := newManager(t, filepath.Join(home, "storage.json"), defaultPath, provider)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	created := storage.Target{Kind: storage.Document, Path: "content://drive/2", Label: storage.Label{Name: "vault.rpv"}}
	if err := manager.Bind(created); err != nil {
		t.Fatal(err)
	}
	if err := manager.Bind(local(defaultPath)); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); len(status.Vaults) != 1 || status.Current != local(defaultPath) {
		t.Fatalf("an empty document stayed listed: %+v", status)
	}
	if _, kept := provider.files[created.Path]; kept || !slices.Equal(provider.discarded, []string{created.Path}) {
		t.Fatalf("empty document: kept=%t discarded=%v", kept, provider.discarded)
	}
}

func TestManagerNeverDiscardsADocumentAVaultWasOpenedAt(t *testing.T) {
	home := t.TempDir()
	defaultPath := filepath.Join(home, "Ravenpass", "vault.rpv")
	provider := &documents{files: map[string][]byte{"content://drive/3": nil}}
	manager := newManager(t, filepath.Join(home, "storage.json"), defaultPath, provider)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	opened := storage.Target{Kind: storage.Document, Path: "content://drive/3", Label: storage.Label{Name: "vault.rpv"}}
	if err := manager.Bind(opened); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x01})
	if err := manager.Identify("document-vault"); err != nil {
		t.Fatal(err)
	}
	provider.files[opened.Path] = nil

	if err := manager.Bind(local(defaultPath)); err != nil {
		t.Fatal(err)
	}
	opened.Vault = "document-vault"
	if status := manager.Status(); !slices.Contains(status.Vaults, opened) {
		t.Fatalf("a document reading empty left the list: %+v", status.Vaults)
	}
	if _, kept := provider.files[opened.Path]; !kept || len(provider.discarded) != 0 {
		t.Fatalf("known document: kept=%t discarded=%v", kept, provider.discarded)
	}
}
