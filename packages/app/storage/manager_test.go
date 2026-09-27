package storage_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/storage"
)

const maxBytes = 1 << 20

func newTestManager(t *testing.T) (*storage.Manager, string, string) {
	t.Helper()
	home := t.TempDir()
	selectionPath := filepath.Join(home, "storage.json")
	defaultPath := filepath.Join(home, "Ravenpass", localfile.DefaultVaultName)
	return newManager(t, selectionPath, defaultPath), selectionPath, defaultPath
}

func newManager(t *testing.T, selectionPath, defaultPath string, extra ...storage.Backend) *storage.Manager {
	t.Helper()
	backends := append([]storage.Backend{localfile.Backend{Home: filepath.Dir(defaultPath)}}, extra...)
	manager, err := storage.NewManager(selectionPath, local(defaultPath), maxBytes, backends...)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func local(path string) storage.Target {
	return storage.Target{Kind: storage.LocalFile, Path: path}
}

func write(t *testing.T, store storage.Ciphertext, ciphertext []byte) {
	t.Helper()
	if err := store.CommitCiphertext(nil, ciphertext, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestManagerOpensDefaultLocationWithoutSelection(t *testing.T) {
	manager, selectionPath, defaultPath := newTestManager(t)
	if status := manager.Status(); status.Available {
		t.Fatal("manager reported storage before opening it")
	}
	if _, err := manager.LoadCiphertext(); !errors.Is(err, storage.ErrUnavailable) {
		t.Fatalf("unbound load: got %v, want ErrUnavailable", err)
	}
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	status := manager.Status()
	if !status.Available || status.Current != local(defaultPath) || status.Default != local(defaultPath) || !status.Restricted {
		t.Fatalf("status after opening the default location: %+v", status)
	}
	if len(status.Kinds) != 1 || status.Kinds[0] != storage.LocalFile {
		t.Fatalf("offered kinds: %v", status.Kinds)
	}
	if _, err := os.Stat(selectionPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("default location wrote a selection record: %v", err)
	}
}

func TestManagerBindRecordsAndReopensLocation(t *testing.T) {
	manager, selectionPath, _ := newTestManager(t)
	chosen := filepath.Join(t.TempDir(), "personal.rpv")
	if err := manager.Bind(local(chosen)); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x01, 0x02, 0x03})

	info, err := os.Stat(selectionPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("selection record mode: got %04o, want 0600", got)
	}
	record, err := os.ReadFile(selectionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(record), chosen) || strings.Contains(string(record), "\x01") {
		t.Fatalf("selection record: %s", record)
	}

	reopened := newManager(t, selectionPath, filepath.Join(t.TempDir(), "unused.rpv"))
	if err := reopened.Open(); err != nil {
		t.Fatal(err)
	}
	if status := reopened.Status(); status.Current.Path != chosen {
		t.Fatalf("reopened location: got %s, want %s", status.Current.Path, chosen)
	}
	loaded, err := reopened.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("reopened vault: got %x, %v", loaded, err)
	}
	if err := manager.Bind(storage.Target{Kind: "public-node", Path: chosen}); !errors.Is(err, storage.ErrUnsupportedKind) {
		t.Fatalf("unsupported kind: got %v, want ErrUnsupportedKind", err)
	}
}

// selectionOf is a selection record naming the one local vault at path.
func selectionOf(path string) []byte {
	return []byte(`{"version":2,"current":"` + path + `","vaults":[{"kind":"local-file","path":"` + path + `"}]}`)
}

func TestManagerReportsUnreachableAndUnusableSelection(t *testing.T) {
	manager, selectionPath, _ := newTestManager(t)
	missing := filepath.Join(t.TempDir(), "gone", "vault.rpv")
	if err := os.WriteFile(selectionPath, selectionOf(missing), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(selectionPath), 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(selectionPath), 0700) })
	if err := manager.Open(); err == nil {
		t.Fatal("unreachable location opened")
	}
	if status := manager.Status(); status.Available || status.Reason != storage.ReasonUnreachable {
		t.Fatalf("unreachable status: %+v", status)
	}

	if err := os.Chmod(filepath.Dir(selectionPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selectionPath, selectionOf("relative.rpv"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Open(); !errors.Is(err, storage.ErrSelectionInvalid) {
		t.Fatalf("malformed selection: got %v, want ErrSelectionInvalid", err)
	}
	status := manager.Status()
	if status.Available || status.Reason != storage.ReasonSelectionUnusable || status.Current.Path != "" {
		t.Fatalf("unusable selection status: %+v", status)
	}
}

func TestManagerRelocatesVaultAndRemovesPreviousFile(t *testing.T) {
	manager, _, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	ciphertext := []byte{0x0a, 0x0b, 0x0c}
	write(t, manager, ciphertext)

	target := local(filepath.Join(t.TempDir(), "moved.rpv"))
	relocation, err := manager.Relocate(target, func(destination storage.Ciphertext) error {
		return destination.CommitCiphertext(nil, ciphertext, func() error { return nil })
	})
	if err != nil {
		t.Fatal(err)
	}
	if relocation.Previous != local(defaultPath) || !relocation.PreviousRemoved {
		t.Fatalf("relocation: %+v", relocation)
	}
	if _, err := os.Stat(defaultPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("previous vault file remains: %v", err)
	}
	loaded, err := manager.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, ciphertext) {
		t.Fatalf("relocated vault: got %x, %v", loaded, err)
	}
	if status := manager.Status(); status.Current != target {
		t.Fatalf("status after relocation: %+v", status)
	}
	if _, err := manager.Relocate(target, func(storage.Ciphertext) error { return nil }); !errors.Is(err, storage.ErrSameLocation) {
		t.Fatalf("relocation to the same location: got %v, want ErrSameLocation", err)
	}
}

func TestManagerKeepsCurrentLocationWhenRelocationFails(t *testing.T) {
	manager, _, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	ciphertext := []byte{0x11, 0x22}
	write(t, manager, ciphertext)

	occupied := filepath.Join(t.TempDir(), "taken.rpv")
	if err := os.WriteFile(occupied, []byte{0x99}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Relocate(local(occupied), func(storage.Ciphertext) error {
		t.Error("write ran against an occupied destination")
		return nil
	}); !errors.Is(err, storage.ErrTargetOccupied) {
		t.Fatalf("occupied destination: got %v, want ErrTargetOccupied", err)
	}
	if data, err := os.ReadFile(occupied); err != nil || !bytes.Equal(data, []byte{0x99}) {
		t.Fatalf("occupied destination changed: %x, %v", data, err)
	}

	refused := errors.New("destination write refused")
	failed := filepath.Join(t.TempDir(), "failed.rpv")
	if _, err := manager.Relocate(local(failed), func(destination storage.Ciphertext) error {
		if err := destination.CommitCiphertext(nil, ciphertext, func() error { return nil }); err != nil {
			return err
		}
		return refused
	}); !errors.Is(err, refused) {
		t.Fatalf("failed write: got %v, want the writer's error", err)
	}
	if _, err := os.Stat(failed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed relocation left a vault copy: %v", err)
	}

	status := manager.Status()
	if !status.Available || status.Current.Path != defaultPath {
		t.Fatalf("status after failed relocation: %+v", status)
	}
	loaded, err := manager.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, ciphertext) {
		t.Fatalf("vault after failed relocation: got %x, %v", loaded, err)
	}
}

func TestManagerReportsUnrestrictedLocation(t *testing.T) {
	manager, _, _ := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); !status.Restricted {
		t.Fatalf("local disk reported as unrestricted: %+v", status)
	}
}

func TestManagerRejectsInvalidConstruction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	vault := local(filepath.Join(t.TempDir(), localfile.DefaultVaultName))
	files := localfile.Backend{}
	if _, err := storage.NewManager("", vault, maxBytes, files); err == nil {
		t.Error("missing selection path was accepted")
	}
	if _, err := storage.NewManager(path, storage.Target{Kind: "relay", Path: vault.Path}, maxBytes, files); !errors.Is(err, storage.ErrUnsupportedKind) {
		t.Error("unsupported default kind was accepted")
	}
	if _, err := storage.NewManager(path, vault, maxBytes); !errors.Is(err, storage.ErrUnsupportedKind) {
		t.Error("a manager without the default's kind was accepted")
	}
	if _, err := storage.NewManager(path, vault, maxBytes, files, files); err == nil {
		t.Error("a kind offered twice was accepted")
	}
	if _, err := storage.NewManager(path, vault, 0, files); err == nil {
		t.Error("invalid size limit was accepted")
	}
}

func TestManagerDelegatesReconciliation(t *testing.T) {
	manager, _, _ := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	ciphertext := []byte{0x31, 0x32}
	write(t, manager, ciphertext)
	called := false
	if err := manager.ReconcileCiphertext(sha256.Sum256(ciphertext), func() error {
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("reconcile: called=%t err=%v", called, err)
	}
	if err := manager.ReconcileCiphertext(sha256.Sum256([]byte{0xff}), func() error { return nil }); !errors.Is(err, storage.ErrStaleHead) {
		t.Fatalf("stale reconcile: got %v, want ErrStaleHead", err)
	}
}

func TestCheckHead(t *testing.T) {
	held := []byte{0x01}
	head := sha256.Sum256(held)
	other := sha256.Sum256([]byte{0x02})
	failure := errors.New("unreadable")
	for _, test := range []struct {
		name     string
		expected *[sha256.Size]byte
		current  []byte
		readErr  error
		want     error
	}{
		{"matching head", &head, held, nil, nil},
		{"changed head", &other, held, nil, storage.ErrStaleHead},
		{"vault where none was expected", nil, held, nil, storage.ErrStaleHead},
		{"no vault where none was expected", nil, nil, storage.ErrNotFound, nil},
		{"no vault where one was expected", &head, nil, storage.ErrNotFound, storage.ErrStaleHead},
		{"unreadable location", &head, nil, failure, failure},
	} {
		err := storage.CheckHead(test.expected, test.current, test.readErr)
		if !errors.Is(err, test.want) {
			t.Errorf("%s: got %v, want %v", test.name, err, test.want)
		}
		if gone := test.name == "no vault where one was expected"; errors.Is(err, storage.ErrVaultMissing) != gone {
			t.Errorf("%s: missing vault reported = %t, want %t", test.name, !gone, gone)
		}
	}
}

func TestManagerRemovesThePreviousFileOnlyOnceTheCopyIsChecked(t *testing.T) {
	manager, _, defaultPath := newTestManager(t)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	ciphertext := []byte{0x41, 0x42}
	write(t, manager, ciphertext)

	target := local(filepath.Join(t.TempDir(), "moved.rpv"))
	if _, err := manager.Relocate(target, func(destination storage.Ciphertext) error {
		write(t, destination, ciphertext)
		written, err := destination.LoadCiphertext()
		if err != nil || !bytes.Equal(written, ciphertext) {
			t.Fatalf("copy read back = %x, %v", written, err)
		}
		if previous, err := os.ReadFile(defaultPath); err != nil || !bytes.Equal(previous, ciphertext) {
			t.Fatalf("previous file while the copy is checked = %x, %v", previous, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(defaultPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("previous file after a checked copy: %v", err)
	}
}

func TestManagerRecordsWhichVaultALocationHolds(t *testing.T) {
	manager, selectionPath, defaultPath := newTestManager(t)
	if err := manager.Identify("0123"); !errors.Is(err, storage.ErrUnavailable) {
		t.Fatalf("identifying an unbound location: got %v, want ErrUnavailable", err)
	}
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	write(t, manager, []byte{0x51})
	if err := manager.Identify("0123"); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); status.Current.Vault != "0123" || status.Vaults[0].Vault != "0123" {
		t.Fatalf("status after identifying = %+v", status)
	}

	relabelled := local(defaultPath)
	relabelled.Label = storage.Label{Name: "vault.rpv", Place: "Ravenpass"}
	if err := manager.Bind(relabelled); err != nil {
		t.Fatal(err)
	}
	target := local(filepath.Join(t.TempDir(), "moved.rpv"))
	if _, err := manager.Relocate(target, func(destination storage.Ciphertext) error {
		write(t, destination, []byte{0x51})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	reopened := newManager(t, selectionPath, defaultPath)
	if err := reopened.Open(); err != nil {
		t.Fatal(err)
	}
	if status := reopened.Status(); !status.Current.Same(target) || status.Current.Vault != "0123" {
		t.Fatalf("reopened status = %+v", status)
	}
}
