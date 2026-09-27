package linkstore

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/linkproto"
)

type recordingBackups struct {
	excluded []string
}

func (r *recordingBackups) Exclude(path string) error {
	r.excluded = append(r.excluded, path)
	return nil
}

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	store, path, _ := newTestStoreWithBackups(t)
	return store, path
}

func newTestStoreWithBackups(t *testing.T) (*Store, string, *recordingBackups) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "application", "extensions.json")
	backups := &recordingBackups{}
	store, err := New(path, backups)
	if err != nil {
		t.Fatal(err)
	}
	return store, path, backups
}

func reopen(t *testing.T, path string) *Store {
	t.Helper()
	store, err := New(path, &recordingBackups{})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestEveryWriteKeepsTheRecordsOutOfBackups(t *testing.T) {
	store, path, backups := newTestStoreWithBackups(t)
	if _, err := store.DesktopKey(); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPort(53117); err != nil {
		t.Fatal(err)
	}
	if len(backups.excluded) != 2 {
		t.Fatalf("two writes marked %d files", len(backups.excluded))
	}
	for _, marked := range backups.excluded {
		if marked == path || filepath.Dir(marked) != filepath.Dir(path) {
			t.Fatalf("marked %q, want a file beside %q that is renamed onto it", marked, path)
		}
	}
}

func TestAStoreNeedsAPathAndABackupExclusion(t *testing.T) {
	if _, err := New("", &recordingBackups{}); err == nil {
		t.Error("a store without a path was created")
	}
	if _, err := New(filepath.Join(t.TempDir(), "extensions.json"), nil); err == nil {
		t.Error("a store without a backup exclusion was created")
	}
}

func extensionKey(t *testing.T) []byte {
	t.Helper()
	key, err := linkproto.GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key.Public
}

func TestAFreshStoreHoldsNothingAndWritesNothing(t *testing.T) {
	store, path := newTestStore(t)
	port, err := store.Port()
	if err != nil || port != 0 {
		t.Fatalf("port = %d, error = %v", port, err)
	}
	extensions, err := store.Extensions()
	if err != nil || len(extensions) != 0 {
		t.Fatalf("extensions = %+v, error = %v", extensions, err)
	}
	if found, ok, err := store.Find(extensionKey(t)); err != nil || ok {
		t.Fatalf("found %+v in a fresh store, error = %v", found, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading a fresh store wrote the file: %v", err)
	}
}

func TestTheDesktopKeyIsGeneratedOnceAndKept(t *testing.T) {
	store, path := newTestStore(t)
	first, err := store.DesktopKey()
	if err != nil {
		t.Fatal(err)
	}
	if !first.Valid() {
		t.Fatal("the generated desktop key is invalid")
	}
	again, err := store.DesktopKey()
	if err != nil || !bytes.Equal(again.Private, first.Private) || !bytes.Equal(again.Public, first.Public) {
		t.Fatalf("the desktop key changed within a run, error = %v", err)
	}
	kept, err := reopen(t, path).DesktopKey()
	if err != nil || !bytes.Equal(kept.Private, first.Private) || !bytes.Equal(kept.Public, first.Public) {
		t.Fatalf("the desktop key changed across runs, error = %v", err)
	}
}

func TestRecordsRoundTripThroughAnOwnerOnlyFile(t *testing.T) {
	store, path := newTestStore(t)
	linkedAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return linkedAt.Add(400 * time.Millisecond) }
	if err := store.SetPort(53117); err != nil {
		t.Fatal(err)
	}
	desktop, err := store.DesktopKey()
	if err != nil {
		t.Fatal(err)
	}
	key := extensionKey(t)
	added, err := store.Add("Chrome · macOS", key)
	if err != nil {
		t.Fatal(err)
	}
	if len(added.ID) != 22 || added.Name != "Chrome · macOS" || !bytes.Equal(added.PublicKey, key) || !added.LinkedAt.Equal(linkedAt) {
		t.Fatalf("added = %+v", added)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %04o, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d files, want no leftover temporary file", len(entries))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var written map[string]any
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatal(err)
	}
	if written["version"] != float64(1) || written["port"] != float64(53117) {
		t.Fatalf("file = %s", data)
	}
	if !strings.Contains(string(data), `"linkedAt":"2026-09-22T12:00:00Z"`) {
		t.Fatalf("linkedAt is not written in whole seconds: %s", data)
	}

	reopened := reopen(t, path)
	port, err := reopened.Port()
	if err != nil || port != 53117 {
		t.Fatalf("port = %d, error = %v", port, err)
	}
	kept, err := reopened.DesktopKey()
	if err != nil || !bytes.Equal(kept.Public, desktop.Public) {
		t.Fatalf("desktop key lost, error = %v", err)
	}
	found, ok, err := reopened.Find(key)
	if err != nil || !ok || found.ID != added.ID || found.Name != added.Name || !found.LinkedAt.Equal(linkedAt) {
		t.Fatalf("found = %+v, %v, error = %v", found, ok, err)
	}
}

func TestExtensionsAreAddedAndRemoved(t *testing.T) {
	store, path := newTestStore(t)
	firstKey, secondKey := extensionKey(t), extensionKey(t)
	first, err := store.Add("Chrome · macOS", firstKey)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Add("Chrome · Windows", secondKey)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("two extensions share an identifier")
	}
	relinked, err := store.Add("Chrome · Linux", secondKey)
	if err != nil {
		t.Fatal(err)
	}
	extensions, err := store.Extensions()
	if err != nil {
		t.Fatal(err)
	}
	if len(extensions) != 2 || extensions[0].ID != first.ID || extensions[1].ID != relinked.ID || extensions[1].Name != "Chrome · Linux" {
		t.Fatalf("a relinked key left a second record: %+v", extensions)
	}

	if err := store.Remove(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove(first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removing twice: got %v, want ErrNotFound", err)
	}
	reopened := reopen(t, path)
	extensions, err = reopened.Extensions()
	if err != nil || len(extensions) != 1 || extensions[0].ID != relinked.ID {
		t.Fatalf("after removal: %+v, error = %v", extensions, err)
	}
	if _, ok, _ := reopened.Find(firstKey); ok {
		t.Fatal("a removed extension is still found")
	}
}

func TestAnExtensionIsRenamedAcrossARestart(t *testing.T) {
	store, path := newTestStore(t)
	key := extensionKey(t)
	linked, err := store.Add("Chromium · macOS", key)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.Add("Chrome · Windows", extensionKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Rename(linked.ID, "  Work laptop \n"); err != nil {
		t.Fatal(err)
	}
	longest := strings.Repeat("é", linkproto.MaxNameLength)
	for _, name := range []string{"", " \t ", longest + "é"} {
		if err := store.Rename(linked.ID, name); !errors.Is(err, ErrInvalidName) {
			t.Fatalf("renaming to %d characters: got %v, want ErrInvalidName", len([]rune(name)), err)
		}
	}
	if err := store.Rename("missing", "Work laptop"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("renaming an unknown extension: got %v, want ErrNotFound", err)
	}
	reopened := reopen(t, path)
	found, ok, err := reopened.Find(key)
	if err != nil || !ok || found.ID != linked.ID || found.Name != "Work laptop" || !found.LinkedAt.Equal(linked.LinkedAt) {
		t.Fatalf("after a restart = %+v, %v, error = %v", found, ok, err)
	}
	extensions, err := reopened.Extensions()
	if err != nil || len(extensions) != 2 || extensions[1].ID != other.ID || extensions[1].Name != "Chrome · Windows" {
		t.Fatalf("renaming one extension changed the others: %+v, error = %v", extensions, err)
	}
	if err := reopened.Rename(linked.ID, longest); err != nil {
		t.Fatalf("a name of %d characters was refused: %v", linkproto.MaxNameLength, err)
	}
}

func TestInvalidInputIsRefused(t *testing.T) {
	store, path := newTestStore(t)
	for _, name := range []string{"", strings.Repeat("a", linkproto.MaxNameLength+1)} {
		if _, err := store.Add(name, extensionKey(t)); err == nil {
			t.Errorf("name of %d characters was accepted", len(name))
		}
	}
	if _, err := store.Add("Chrome · macOS", []byte("short")); err == nil {
		t.Error("a short public key was accepted")
	}
	for _, port := range []int{0, 1023, 65536} {
		if err := store.SetPort(port); err == nil {
			t.Errorf("port %d was accepted", port)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused input wrote the file: %v", err)
	}
}

func TestAMalformedFileIsRejectedAndKept(t *testing.T) {
	valid := func(t *testing.T) map[string]any {
		key, err := linkproto.GenerateKeyPair(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{
			"version":    1,
			"port":       53117,
			"desktopKey": map[string]any{"private": key.Private, "public": key.Public},
			"extensions": []any{map[string]any{
				"id": "AAAAAAAAAAAAAAAAAAAAAA", "name": "Chrome · macOS", "publicKey": key.Public, "linkedAt": "2026-09-22T12:00:00Z",
			}},
		}
	}
	extensionField := func(record map[string]any, field string, value any) {
		record["extensions"].([]any)[0].(map[string]any)[field] = value
	}
	cases := map[string]func(map[string]any){
		"another version":        func(r map[string]any) { r["version"] = 2 },
		"a port below 1024":      func(r map[string]any) { r["port"] = 80 },
		"a port above 65535":     func(r map[string]any) { r["port"] = 70000 },
		"mismatched desktop key": func(r map[string]any) { r["desktopKey"].(map[string]any)["public"] = make([]byte, 32) },
		"a short desktop key":    func(r map[string]any) { r["desktopKey"].(map[string]any)["private"] = []byte("short") },
		"an identifier too long": func(r map[string]any) { extensionField(r, "id", "AAAAAAAAAAAAAAAAAAAAAAAA") },
		"an empty name":          func(r map[string]any) { extensionField(r, "name", "") },
		"a short public key":     func(r map[string]any) { extensionField(r, "publicKey", []byte("short")) },
		"no link date":           func(r map[string]any) { extensionField(r, "linkedAt", "0001-01-01T00:00:00Z") },
		"a repeated extension": func(r map[string]any) {
			r["extensions"] = append(r["extensions"].([]any), r["extensions"].([]any)[0])
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			record := valid(t)
			change(record)
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			assertRejected(t, data)
		})
	}
	t.Run("not JSON", func(t *testing.T) { assertRejected(t, []byte("{")) })
	t.Run("empty", func(t *testing.T) { assertRejected(t, nil) })
	t.Run("the valid record", func(t *testing.T) {
		data, err := json.Marshal(valid(t))
		if err != nil {
			t.Fatal(err)
		}
		store, path := newTestStore(t)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if extensions, err := store.Extensions(); err != nil || len(extensions) != 1 {
			t.Fatalf("extensions = %+v, error = %v", extensions, err)
		}
	})
}

// assertRejected checks that every call reports data malformed and none replaces the file.
func assertRejected(t *testing.T, data []byte) {
	t.Helper()
	store, path := newTestStore(t)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Extensions(); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Extensions: got %v, want ErrMalformed", err)
	}
	if _, err := store.Port(); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Port: got %v, want ErrMalformed", err)
	}
	if _, err := store.DesktopKey(); !errors.Is(err, ErrMalformed) {
		t.Fatalf("DesktopKey: got %v, want ErrMalformed", err)
	}
	if _, err := store.Add("Chrome · macOS", extensionKey(t)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Add: got %v, want ErrMalformed", err)
	}
	kept, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(kept, data) {
		t.Fatalf("the malformed file was replaced, error = %v", err)
	}
}
