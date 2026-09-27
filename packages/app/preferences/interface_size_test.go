package preferences

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInterfaceSizeIsTheDefaultUntilChosen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if size := newStore(t, path).InterfaceSize(); size != defaultInterfaceSize {
		t.Fatalf("size without a record = %d", size)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"interfaceSize":117}`), 0600); err != nil {
		t.Fatal(err)
	}
	if size := newStore(t, path).InterfaceSize(); size != defaultInterfaceSize {
		t.Fatalf("size from a record naming no offered size = %d", size)
	}
}

func TestInterfaceSizeSurvivesAReloadAndRefusesOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetInterfaceSize(130); err != nil {
		t.Fatal(err)
	}
	if err := store.SetInterfaceSize(120); !errors.Is(err, ErrUnsupportedInterfaceSize) {
		t.Fatalf("an unoffered size = %v", err)
	}
	if size := newStore(t, path).InterfaceSize(); size != 130 {
		t.Fatalf("size after a restart = %d", size)
	}
}
