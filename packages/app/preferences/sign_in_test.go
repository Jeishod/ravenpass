package preferences

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSignInStyleIsTheCardUntilChosen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if style := newStore(t, path).SignInStyle(); style != SignInCard {
		t.Fatalf("style without a record = %q, want card", style)
	}
	for _, content := range []string{
		`{"version":1,"language":"ru","siteIconsOff":true}`,
		`{"version":1,"signInStyle":"popup"}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if style := newStore(t, path).SignInStyle(); style != SignInCard {
			t.Fatalf("style for record %s = %q, want card", content, style)
		}
	}
}

func TestSignInStyleSurvivesOtherChangesAndARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetSignInStyle(SignInField); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSiteIcons(false); err != nil {
		t.Fatal(err)
	}
	reopened := newStore(t, path)
	if style := reopened.SignInStyle(); style != SignInField {
		t.Fatalf("style after a restart = %q, want field", style)
	}
	if reopened.SiteIcons() {
		t.Fatal("recording the sign-in style lost the site icons choice")
	}
	if err := reopened.SetSignInStyle(SignInCard); err != nil {
		t.Fatal(err)
	}
	if style := newStore(t, path).SignInStyle(); style != SignInCard {
		t.Fatalf("style after choosing the card again = %q", style)
	}
}

func TestAnUnofferedSignInStyleIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetSignInStyle("popup"); !errors.Is(err, ErrUnsupportedSignInStyle) {
		t.Fatalf("unoffered style: got %v, want ErrUnsupportedSignInStyle", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused style wrote a record: %v", err)
	}
}
