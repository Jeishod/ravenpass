//go:build darwin

package localfile

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

func vaultPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), DefaultVaultName)
}

func newStore(t *testing.T, path string, maxBytes int64) *Store {
	t.Helper()
	store, err := New(path, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func commit(t *testing.T, store *Store, expected *[sha256.Size]byte, ciphertext []byte) [sha256.Size]byte {
	t.Helper()
	finalized := false
	if err := store.CommitCiphertext(expected, ciphertext, func() error {
		finalized = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !finalized {
		t.Fatal("successful commit did not invoke finalizer")
	}
	return sha256.Sum256(ciphertext)
}

func TestStoreRoundTripAndPermissions(t *testing.T) {
	path := vaultPath(t)
	store := newStore(t, path, 1024)
	if !store.Restricted() {
		t.Fatal("temporary directory reported as unable to restrict access")
	}
	if _, err := store.LoadCiphertext(); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("empty vault: got %v, want ErrNotFound", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadCiphertext(); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("a zero-byte file: got %v, want ErrNotFound", err)
	}

	ciphertext := []byte{0x00, 0xff, 0x42, 0x00, 0x81}
	head := commit(t, store, nil, ciphertext)
	loaded, err := store.LoadCiphertext()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, ciphertext) {
		t.Fatalf("loaded ciphertext differs: got %x, want %x", loaded, ciphertext)
	}
	replacement := []byte{0x91, 0x92, 0x93}
	commit(t, store, &head, replacement)
	loaded, err = store.LoadCiphertext()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, replacement) {
		t.Fatalf("replaced ciphertext differs: got %x, want %x", loaded, replacement)
	}

	for _, name := range []string{store.name, store.lockName} {
		info, err := os.Stat(filepath.Join(store.directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0600 {
			t.Errorf("%s mode: got %04o, want 0600", name, got)
		}
	}
}

func TestStoreKeepsUnrelatedFilesInSharedDirectory(t *testing.T) {
	directory := t.TempDir()
	neighbor := filepath.Join(directory, "notes.txt")
	if err := os.WriteFile(neighbor, []byte("kept"), 0644); err != nil {
		t.Fatal(err)
	}
	store := newStore(t, filepath.Join(directory, "personal.rpv"), 1024)
	commit(t, store, nil, []byte{0x01, 0x02})
	if _, err := New(filepath.Join(directory, "personal.rpv"), 1024); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(neighbor)
	if err != nil || !bytes.Equal(data, []byte("kept")) {
		t.Fatalf("neighboring file changed: %x, %v", data, err)
	}
	info, err := os.Stat(neighbor)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0644 {
		t.Fatalf("neighboring file mode: got %04o, want 0644", got)
	}
}

func TestInvalidSaveKeepsPreviousCiphertext(t *testing.T) {
	store := newStore(t, vaultPath(t), 4)
	old := []byte{0x10, 0x20, 0x30}
	head := commit(t, store, nil, old)
	if err := store.CommitCiphertext(&head, nil, func() error { return nil }); !errors.Is(err, storage.ErrEmptyCiphertext) {
		t.Fatalf("empty save: got %v, want ErrEmptyCiphertext", err)
	}
	if err := store.CommitCiphertext(&head, []byte{1, 2, 3, 4, 5}, func() error { return nil }); !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("oversize save: got %v, want ErrTooLarge", err)
	}
	loaded, err := store.LoadCiphertext()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, old) {
		t.Fatalf("invalid save changed vault: got %x, want %x", loaded, old)
	}
}

func TestLoadRestoresOwnerOnlyAccessAndRejectsOversizeData(t *testing.T) {
	path := vaultPath(t)
	store := newStore(t, path, 4)
	commit(t, store, nil, []byte{1, 2, 3})
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, []byte{1, 2, 3}) {
		t.Fatalf("widened vault file: got %x, %v", loaded, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("restored mode: got %04o, want 0600", got)
	}

	if err := os.WriteFile(path, []byte{1, 2, 3, 4, 5}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadCiphertext(); !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("oversize file: got %v, want ErrTooLarge", err)
	}
}

func TestLoadRejectsVaultSymlink(t *testing.T) {
	path := vaultPath(t)
	store := newStore(t, path, 1024)
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte{0x99}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadCiphertext(); err == nil {
		t.Fatal("symlinked vault was read")
	}
}

func TestNewRejectsUnsupportedPaths(t *testing.T) {
	directory := t.TempDir()
	occupied := filepath.Join(directory, "file")
	if err := os.WriteFile(occupied, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"relative":       "vault.rpv",
		"directory":      directory,
		"reserved name":  filepath.Join(directory, ".vault.rpv"),
		"parent is file": filepath.Join(occupied, DefaultVaultName),
	} {
		if _, err := New(path, 1024); err == nil {
			t.Errorf("%s path was accepted", name)
		}
	}
	if _, err := New(filepath.Join(directory, DefaultVaultName), 0); err == nil {
		t.Error("invalid size limit was accepted")
	}
}

func TestRemoveClearsVaultAndItsSiblings(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, DefaultVaultName)
	store := newStore(t, path, 1024)
	commit(t, store, nil, []byte{0x01, 0x02})
	neighbor := filepath.Join(directory, "other.rpv")
	if err := os.WriteFile(neighbor, []byte("kept"), 0600); err != nil {
		t.Fatal(err)
	}
	interrupted := filepath.Join(directory, store.tempBase+strings.Repeat("a", tempSuffixLength))
	if err := os.WriteFile(interrupted, []byte{0xde}, 0600); err != nil {
		t.Fatal(err)
	}

	if err := store.Remove(); err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{path, interrupted, filepath.Join(directory, store.lockName)} {
		if _, err := os.Stat(removed); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s remains: %v", removed, err)
		}
	}
	if _, err := os.Stat(neighbor); err != nil {
		t.Errorf("neighboring vault was removed: %v", err)
	}
	if err := store.Remove(); err != nil {
		t.Fatalf("removing an absent vault: %v", err)
	}
}

func TestInterruptedTemporaryWriteCannotReplaceVault(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, DefaultVaultName)
	store := newStore(t, path, 1024)
	old := []byte{0xa0, 0xb0, 0xc0}
	commit(t, store, nil, old)
	interrupted := filepath.Join(directory, store.tempBase+strings.Repeat("a", tempSuffixLength))
	if err := os.WriteFile(interrupted, []byte{0xde}, 0600); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(directory, store.tempBase+"unrelated")
	if err := os.WriteFile(unrelated, []byte{0xbe}, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte{0xad}, 0600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(directory, store.tempBase+strings.Repeat("b", tempSuffixLength))
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path, 1024); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(interrupted); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted temporary file remains: %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated file was removed: %v", err)
	}
	if _, err := os.Lstat(symlink); err != nil {
		t.Fatalf("symlink was removed: %v", err)
	}
	if targetData, err := os.ReadFile(target); err != nil || !bytes.Equal(targetData, []byte{0xad}) {
		t.Fatalf("symlink target changed: %x, %v", targetData, err)
	}
	loaded, err := store.LoadCiphertext()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, old) {
		t.Fatalf("interrupted write changed vault: got %x, want %x", loaded, old)
	}
}

func TestTwoStoresRejectStaleConcurrentWriter(t *testing.T) {
	path := vaultPath(t)
	left := newStore(t, path, 1024)
	right := newStore(t, path, 1024)
	head := commit(t, left, nil, []byte{0x01})

	type result struct {
		ciphertext []byte
		err        error
		finalized  bool
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for _, attempt := range []struct {
		store      *Store
		ciphertext []byte
	}{
		{store: left, ciphertext: []byte{0x02}},
		{store: right, ciphertext: []byte{0x03}},
	} {
		go func() {
			<-start
			finalized := false
			err := attempt.store.CommitCiphertext(&head, attempt.ciphertext, func() error {
				finalized = true
				return nil
			})
			results <- result{ciphertext: attempt.ciphertext, err: err, finalized: finalized}
		}()
	}
	close(start)

	var winner []byte
	var committed, stale int
	for range 2 {
		attempt := <-results
		switch {
		case attempt.err == nil && attempt.finalized:
			committed++
			winner = attempt.ciphertext
		case errors.Is(attempt.err, storage.ErrStaleHead) && !attempt.finalized:
			stale++
		default:
			t.Fatalf("unexpected commit result: err=%v finalized=%t", attempt.err, attempt.finalized)
		}
	}
	if committed != 1 || stale != 1 {
		t.Fatalf("got %d commits and %d stale writes, want one each", committed, stale)
	}
	loaded, err := right.LoadCiphertext()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, winner) {
		t.Fatalf("stored ciphertext %x differs from committed ciphertext %x", loaded, winner)
	}
}

func TestFinalizerFailureIsNotAcknowledged(t *testing.T) {
	store := newStore(t, vaultPath(t), 1024)
	head := commit(t, store, nil, []byte{0x01})
	candidate := []byte{0x02}
	finalizerError := errors.New("witness write failed")
	called := false
	err := store.CommitCiphertext(&head, candidate, func() error {
		called = true
		loaded, loadErr := store.LoadCiphertext()
		if loadErr != nil || !bytes.Equal(loaded, candidate) {
			t.Errorf("finalizer ran before replacement: got %x, %v", loaded, loadErr)
		}
		return finalizerError
	})
	if !called || !errors.Is(err, storage.ErrFinalizerFailed) || !errors.Is(err, finalizerError) {
		t.Fatalf("finalizer failure: called=%t err=%v", called, err)
	}
	loaded, err := store.LoadCiphertext()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, candidate) {
		t.Fatalf("failed finalizer unexpectedly rolled back file: got %x", loaded)
	}
	if err := store.CommitCiphertext(&head, []byte{0x03}, func() error { return nil }); !errors.Is(err, storage.ErrStaleHead) {
		t.Fatalf("old head was accepted after finalizer failure: %v", err)
	}
}

func TestCommitRequiresFinalizer(t *testing.T) {
	store := newStore(t, vaultPath(t), 1024)
	if err := store.CommitCiphertext(nil, []byte{0x01}, nil); !errors.Is(err, storage.ErrFinalizerRequired) {
		t.Fatalf("missing finalizer: got %v, want ErrFinalizerRequired", err)
	}
	if _, err := store.LoadCiphertext(); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing finalizer created a vault: %v", err)
	}
	commit(t, store, nil, []byte{0x01})
	called := false
	if err := store.CommitCiphertext(nil, []byte{0x02}, func() error {
		called = true
		return nil
	}); !errors.Is(err, storage.ErrStaleHead) || called {
		t.Fatalf("absent-head commit against existing vault: err=%v finalized=%t", err, called)
	}
}

func TestReconcileVerifiesHeadWithoutRewritingVault(t *testing.T) {
	path := vaultPath(t)
	store := newStore(t, path, 1024)
	ciphertext := []byte{0x01, 0x02, 0x03}
	head := commit(t, store, nil, ciphertext)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if err := store.ReconcileCiphertext(head, func() error {
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("reconcile: called=%t err=%v", called, err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("reconciliation replaced the vault file")
	}
	loaded, err := store.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, ciphertext) {
		t.Fatalf("reconciliation changed ciphertext: got %x, %v", loaded, err)
	}

	stale := sha256.Sum256([]byte{0xff})
	called = false
	if err := store.ReconcileCiphertext(stale, func() error {
		called = true
		return nil
	}); !errors.Is(err, storage.ErrStaleHead) || called {
		t.Fatalf("stale reconciliation: called=%t err=%v", called, err)
	}
	if err := store.ReconcileCiphertext(head, nil); !errors.Is(err, storage.ErrFinalizerRequired) {
		t.Fatalf("missing reconciliation finalizer: %v", err)
	}
	failure := errors.New("witness update failed")
	if err := store.ReconcileCiphertext(head, func() error { return failure }); !errors.Is(err, storage.ErrFinalizerFailed) || !errors.Is(err, failure) {
		t.Fatalf("failed reconciliation finalizer: %v", err)
	}
	final, err := os.Stat(path)
	if err != nil || !os.SameFile(before, final) {
		t.Fatalf("failed reconciliation replaced the vault: %v", err)
	}
}

func TestReconcileHoldsLockUntilFinalizerCompletes(t *testing.T) {
	path := vaultPath(t)
	reconciler := newStore(t, path, 1024)
	writer := newStore(t, path, 1024)
	head := commit(t, writer, nil, []byte{0x01})
	entered := make(chan struct{})
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	reconcileDone := make(chan error, 1)
	go func() {
		reconcileDone <- reconciler.ReconcileCiphertext(head, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	writerDone := make(chan error, 1)
	go func() {
		writerDone <- writer.CommitCiphertext(&head, []byte{0x02}, func() error { return nil })
	}()
	select {
	case err := <-writerDone:
		t.Fatalf("writer completed while reconciliation finalizer held lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	released = true
	if err := <-reconcileDone; err != nil {
		t.Fatal(err)
	}
	if err := <-writerDone; err != nil {
		t.Fatal(err)
	}
	loaded, err := writer.LoadCiphertext()
	if err != nil || !bytes.Equal(loaded, []byte{0x02}) {
		t.Fatalf("writer did not commit after reconciliation: got %x, %v", loaded, err)
	}
}
