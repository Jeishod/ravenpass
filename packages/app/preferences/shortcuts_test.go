package preferences

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShortcutsStartEmptyAndSurviveAReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if got := store.Shortcuts(); len(got) != 0 {
		t.Fatalf("shortcuts without a record = %v", got)
	}
	if err := store.SetShortcut("palette", "Mod+P"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSiteIcons(false); err != nil {
		t.Fatal(err)
	}
	if got := newStore(t, path).Shortcuts(); got["palette"] != "Mod+P" || len(got) != 1 {
		t.Fatalf("shortcuts after a restart = %v", got)
	}
}

func TestAnEmptyHotkeyRestoresTheDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetShortcut("search", "Mod+Shift+F"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetShortcut("search", ""); err != nil {
		t.Fatal(err)
	}
	if got := store.Shortcuts(); len(got) != 0 {
		t.Fatalf("shortcuts after a reset = %v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "shortcuts") {
		t.Fatalf("a record with every default names shortcuts: %s", data)
	}
}

func TestShortcutsReturnedAreACopy(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "preferences.json"))
	if err := store.SetShortcut("palette", "Mod+P"); err != nil {
		t.Fatal(err)
	}
	store.Shortcuts()["palette"] = "Mod+X"
	if got := store.Shortcuts()["palette"]; got != "Mod+P" {
		t.Fatalf("changing the returned map changed the store: %q", got)
	}
}

func TestSetShortcutRefusesWhatBreaksTheBounds(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "preferences.json"))
	for _, tc := range []struct{ action, hotkey string }{
		{"", "Mod+K"},
		{"Palette", "Mod+K"},
		{strings.Repeat("a", maxShortcutActionLen+1), "Mod+K"},
		{"palette", strings.Repeat("K", maxShortcutHotkeyLen+1)},
		{"palette", "Mod+\nK"},
	} {
		if err := store.SetShortcut(tc.action, tc.hotkey); !errors.Is(err, ErrInvalidShortcut) {
			t.Fatalf("SetShortcut(%q, %q) = %v", tc.action, tc.hotkey, err)
		}
	}
	for index := range maxShortcuts {
		if err := store.SetShortcut(fmt.Sprintf("action-%d", index), "Mod+K"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetShortcut("one-more", "Mod+K"); !errors.Is(err, ErrInvalidShortcut) {
		t.Fatalf("a shortcut past the limit = %v", err)
	}
	if err := store.SetShortcut("action-0", "Mod+J"); err != nil {
		t.Fatalf("changing a recorded shortcut at the limit = %v", err)
	}
}

func TestARecordWithBrokenShortcutsKeepsTheDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	record := `{"version":1,"language":"ru","shortcuts":{"palette":"Mod+P","Bad Action":"Mod+K"}}`
	if err := os.WriteFile(path, []byte(record), 0600); err != nil {
		t.Fatal(err)
	}
	store := newStore(t, path)
	if got := store.Shortcuts(); len(got) != 0 {
		t.Fatalf("shortcuts of a broken record = %v", got)
	}
	if language, chosen := store.Language(); language != Russian || !chosen {
		t.Fatal("broken shortcuts discarded the rest of the record")
	}
}
