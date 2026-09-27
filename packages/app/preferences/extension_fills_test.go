package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtensionFillsGoUnconfirmedUntilTheOwnerAsks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if newStore(t, path).ConfirmExtensionFills() {
		t.Fatal("fills wait for confirmation without a record")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"signInStyle":"field"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).ConfirmExtensionFills() {
		t.Fatal("fills wait for confirmation by a record that does not name the choice")
	}
}

func TestTheExtensionFillChoiceSurvivesOtherChangesAndARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetConfirmExtensionFills(true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSignInStyle(SignInField); err != nil {
		t.Fatal(err)
	}
	reopened := newStore(t, path)
	if !reopened.ConfirmExtensionFills() {
		t.Fatal("confirming fills was lost after a restart")
	}
	if reopened.SignInStyle() != SignInField {
		t.Fatal("recording the fill choice lost the sign-in style")
	}
	if err := reopened.SetConfirmExtensionFills(false); err != nil {
		t.Fatal(err)
	}
	if newStore(t, path).ConfirmExtensionFills() {
		t.Fatal("turning confirmation off did not survive a restart")
	}
}
