package preferences

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBankDetailsDefaultToOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if !newStore(t, path).BankDetails() {
		t.Fatal("the bank lookup without a record is off")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"ru","siteIconsOff":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if !newStore(t, path).BankDetails() {
		t.Fatal("the bank lookup of a record that does not name the choice is off")
	}
}

func TestBankDetailsSurviveOtherChangesAndAReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetBankDetails(false); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSiteIcons(false); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLanguage(Russian); err != nil {
		t.Fatal(err)
	}
	reopened := newStore(t, path)
	if reopened.BankDetails() {
		t.Fatal("the bank lookup turned off came back on after a restart")
	}
	if reopened.SiteIcons() {
		t.Fatal("recording the bank lookup lost the website icons choice")
	}
	if err := reopened.SetBankDetails(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "bankDetailsOff") {
		t.Fatalf("a record with the lookup on names the choice: %s", data)
	}
	if !newStore(t, path).BankDetails() {
		t.Fatal("the bank lookup turned back on stayed off after a restart")
	}
}
