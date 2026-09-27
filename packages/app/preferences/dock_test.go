package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDockStaysUntilTheUserHidesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if newStore(t, path).DockHiddenWithWindow() {
		t.Fatal("the Dock icon hides without a record")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"ru","siteIconsOff":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).DockHiddenWithWindow() {
		t.Fatal("the Dock icon hides for a record that does not name the choice")
	}
}

func TestDockChoiceSurvivesOtherChangesAndAReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetDockHiddenWithWindow(true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSiteIcons(false); err != nil {
		t.Fatal(err)
	}
	reopened := newStore(t, path)
	if !reopened.DockHiddenWithWindow() {
		t.Fatal("the Dock choice was lost after a restart")
	}
	if reopened.SiteIcons() {
		t.Fatal("recording the Dock choice lost the site icons choice")
	}
	if err := reopened.SetDockHiddenWithWindow(false); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).DockHiddenWithWindow() {
		t.Fatal("turning the Dock choice off did not survive a restart")
	}
}
