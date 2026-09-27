package api

import (
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

// namedFolders is a host that names the folders it holds.
type namedFolders map[string]string

func (folders namedFolders) Place(folder string) (string, bool) {
	place, named := folders[folder]
	return place, named
}

func TestALocalFileIsPlacedByItsFoldersName(t *testing.T) {
	service := newServiceOnHost(t, Host{})
	for path, want := range map[string]storage.Label{
		"/Users/me/Documents/vault.rpv":                                    {Name: "vault.rpv", Place: "Documents"},
		"/Users/me/Library/CloudStorage/GoogleDrive-me/My Drive/vault.rpv": {Name: "vault.rpv", Place: "My Drive"},
		"/vault.rpv": {Name: "vault.rpv", Place: "/"},
	} {
		if got := service.label(storage.Target{Kind: storage.LocalFile, Path: path}); got != want {
			t.Errorf("label(%q) = %+v, want %+v", path, got, want)
		}
	}
}

func TestTheHostNamesTheFoldersItKnows(t *testing.T) {
	service := newServiceOnHost(t, Host{Places: namedFolders{"/phone/files": "On this device"}})
	for path, want := range map[string]storage.Label{
		"/phone/files/vault 2.rpv":  {Name: "vault 2.rpv", Place: "On this device"},
		"/phone/Download/vault.rpv": {Name: "vault.rpv", Place: "Download"},
	} {
		if got := service.label(storage.Target{Kind: storage.LocalFile, Path: path}); got != want {
			t.Errorf("label(%q) = %+v, want %+v", path, got, want)
		}
	}
}

func TestADocumentKeepsItsProvidersLabel(t *testing.T) {
	chosen := storage.Label{Name: "Family.rpv", Place: "Drive"}
	target := storage.Target{Kind: storage.Document, Path: "content://drive/document/1", Label: chosen}
	service := newServiceOnHost(t, Host{Places: namedFolders{filepath.Dir(target.Path): "On this device"}})
	if got := service.label(target); got != chosen {
		t.Fatalf("a document's label = %+v, want %+v", got, chosen)
	}
}
