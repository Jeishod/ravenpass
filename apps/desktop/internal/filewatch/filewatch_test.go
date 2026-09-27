package filewatch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

type stubVault struct {
	unlocked bool
	target   storage.Target
}

func (v *stubVault) Unlocked() bool { return v.unlocked }

func (v *stubVault) Storage() storage.Status { return storage.Status{Current: v.target} }

type watched struct {
	vault   *stubVault
	watcher *Watcher
	path    string
	follows int
}

func newWatched(t *testing.T) *watched {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Ravenpass.rpv")
	write(t, path, "first")
	w := &watched{vault: &stubVault{unlocked: true, target: storage.Target{Kind: storage.LocalFile, Path: path}}, path: path}
	w.watcher = New(w.vault, func() { w.follows++ })
	return w
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// replace puts content in the file's place by rename, as Ravenpass and synced folders write it.
func replace(t *testing.T, path, content string) {
	t.Helper()
	next := path + ".next"
	write(t, next, content)
	if err := os.Rename(next, path); err != nil {
		t.Fatal(err)
	}
}

func (w *watched) expect(t *testing.T, follows int, after string) {
	t.Helper()
	w.watcher.check()
	if w.follows != follows {
		t.Fatalf("%s: followed %d times, want %d", after, w.follows, follows)
	}
}

func TestFollowsEveryWayTheFileChanges(t *testing.T) {
	w := newWatched(t)
	w.expect(t, 1, "the file the vault opened with")
	w.expect(t, 1, "an unchanged file")
	replace(t, w.path, "second")
	w.expect(t, 2, "a replacement by rename")
	replace(t, w.path, "third!")
	w.expect(t, 3, "a second replacement by rename")
	write(t, w.path, "rewritten in place")
	w.expect(t, 4, "a rewrite in place")
	if err := os.Remove(w.path); err != nil {
		t.Fatal(err)
	}
	w.expect(t, 5, "a file that went")
	w.expect(t, 5, "a file still missing")
	write(t, w.path, "created again")
	w.expect(t, 6, "a file created again")
}

func TestAFileMissingWhenTheVaultOpensIsNotFollowed(t *testing.T) {
	w := newWatched(t)
	if err := os.Remove(w.path); err != nil {
		t.Fatal(err)
	}
	w.expect(t, 0, "a file missing from the start")
}

func TestFollowsOnlyAnOpenVaultInALocalFile(t *testing.T) {
	w := newWatched(t)
	w.vault.unlocked = false
	w.expect(t, 0, "a locked vault")
	w.vault.unlocked = true
	w.expect(t, 1, "the vault opening")
	w.vault.unlocked = false
	w.expect(t, 1, "the vault locking")
	w.vault.unlocked = true
	w.expect(t, 2, "the vault opening again")
	w.vault.target = storage.Target{Kind: storage.Document, Path: "content://provider/document/1"}
	w.expect(t, 2, "a document")
}
