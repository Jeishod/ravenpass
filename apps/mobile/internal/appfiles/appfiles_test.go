package appfiles

import (
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

const files = "/data/user/0/com.dortanes.ravenpass/files"

func newPlaces(t *testing.T) (Places, *preferences.Store) {
	t.Helper()
	settings, err := preferences.New(filepath.Join(t.TempDir(), "preferences.json"), func() []string { return []string{"en-US"} })
	if err != nil {
		t.Fatal(err)
	}
	return Places{Directory: files, Dialogs: settings.Dialogs}, settings
}

func TestTheAppsFilesDirectoryIsThisDeviceInTheOwnersLanguage(t *testing.T) {
	places, settings := newPlaces(t)
	if got, named := places.Place(files); !named || got != "On this device" {
		t.Fatalf("the files directory in English = %q, %v", got, named)
	}
	if err := settings.SetLanguage(preferences.Russian); err != nil {
		t.Fatal(err)
	}
	if got, named := places.Place(files); !named || got != "На этом устройстве" {
		t.Fatalf("the files directory in Russian = %q, %v", got, named)
	}
}

func TestAnotherFolderIsNotNamed(t *testing.T) {
	places, _ := newPlaces(t)
	for _, folder := range []string{
		"/data/user/0/com.dortanes.ravenpass",
		files + "/vaults",
		"/storage/emulated/0/Download",
		"/data/local/tmp",
	} {
		if got, named := places.Place(folder); named {
			t.Errorf("Place(%q) = %q, want no name", folder, got)
		}
	}
}
