package extensionaccess

import (
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

func newSettings(t *testing.T) (*Settings, *preferences.Store) {
	t.Helper()
	recorded, err := preferences.New(filepath.Join(t.TempDir(), "preferences.json"), func() []string {
		return []string{"ru-RU"}
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := NewSettings(recorded)
	if err != nil {
		t.Fatal(err)
	}
	return settings, recorded
}

func TestSettingsReportTheLanguageInUse(t *testing.T) {
	settings, recorded := newSettings(t)
	if language := settings.Language(); language != "ru" {
		t.Fatalf("language before a choice on a Russian device = %q, want ru", language)
	}
	if err := recorded.SetLanguage(preferences.English); err != nil {
		t.Fatal(err)
	}
	if language := settings.Language(); language != "en" {
		t.Fatalf("language after choosing English = %q, want en", language)
	}
}

func TestSettingsReportTheRecordedSignInStyle(t *testing.T) {
	settings, recorded := newSettings(t)
	if style := settings.SignInStyle(); style != "card" {
		t.Fatalf("style before a choice = %q, want card", style)
	}
	if err := recorded.SetSignInStyle(preferences.SignInField); err != nil {
		t.Fatal(err)
	}
	if style := settings.SignInStyle(); style != "field" {
		t.Fatalf("style after choosing the menu under the field = %q, want field", style)
	}
}
