package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheIdentityListStaysOffUntilTheUserTurnsItOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if newStore(t, path).IdentityList() {
		t.Fatal("the identity list is on without a record")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"ru","hideDockWithWindow":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).IdentityList() {
		t.Fatal("the identity list is on for a record that does not name the choice")
	}
}

func TestTheIdentityListChoiceSurvivesOtherChangesAndAReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetIdentityList(true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDockHiddenWithWindow(true); err != nil {
		t.Fatal(err)
	}
	reopened := newStore(t, path)
	if !reopened.IdentityList() {
		t.Fatal("turning the identity list on was lost after a restart")
	}
	if !reopened.DockHiddenWithWindow() {
		t.Fatal("recording the identity list lost the Dock choice")
	}
	if err := reopened.SetIdentityList(false); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).IdentityList() {
		t.Fatal("turning the identity list off did not survive a restart")
	}
}
