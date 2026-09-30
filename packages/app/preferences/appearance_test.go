package preferences

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAppearanceFollowsTheSystemUntilChosen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if appearance := newStore(t, path).Appearance(); appearance != AppearanceSystem {
		t.Fatalf("appearance without a record = %q", appearance)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"appearance":"sepia"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if appearance := newStore(t, path).Appearance(); appearance != AppearanceSystem {
		t.Fatalf("appearance from a record naming no offered one = %q", appearance)
	}
}

func TestAppearanceSurvivesARestartAndRefusesOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	heard := 0
	store.OnAppearanceChange(func() {
		if store.Appearance() == AppearanceLight {
			heard++
		}
	})
	if err := store.SetAppearance(AppearanceLight); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAppearance("sepia"); !errors.Is(err, ErrUnsupportedAppearance) {
		t.Fatalf("an unoffered appearance = %v", err)
	}
	if heard != 1 {
		t.Fatalf("listener heard %d light choices, want 1", heard)
	}
	if appearance := newStore(t, path).Appearance(); appearance != AppearanceLight {
		t.Fatalf("appearance after a restart = %q", appearance)
	}
}
