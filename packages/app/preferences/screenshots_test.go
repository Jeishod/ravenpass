package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScreenshotsStayBlockedUntilTheUserAllowsThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if newStore(t, path).ScreenshotsAllowed() {
		t.Fatal("screenshots are allowed without a record")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"ru","siteIconsOff":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).ScreenshotsAllowed() {
		t.Fatal("screenshots are allowed by a record that does not name the choice")
	}
}

func TestTheScreenshotChoiceSurvivesOtherChangesAndAReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetScreenshotsAllowed(true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSiteIcons(false); err != nil {
		t.Fatal(err)
	}
	reopened := newStore(t, path)
	if !reopened.ScreenshotsAllowed() {
		t.Fatal("allowing screenshots was lost after a restart")
	}
	if reopened.SiteIcons() {
		t.Fatal("recording the screenshot choice lost the website icons choice")
	}
	if err := reopened.SetScreenshotsAllowed(false); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).ScreenshotsAllowed() {
		t.Fatal("blocking screenshots again did not survive a restart")
	}
}
