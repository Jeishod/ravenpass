package documents

import (
	"errors"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

var chosen = storage.Label{Name: "vault.rpv", Place: "Drive"}

func TestFilesPickDocuments(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	provider.label = chosen
	files := Files{Provider: provider}
	if kinds := files.Kinds(); !slices.Equal(kinds, []storage.Kind{storage.Document}) {
		t.Fatalf("picked kinds: %v", kinds)
	}
	want := storage.Target{Kind: storage.Document, Path: address, Label: chosen}
	created, picked, err := files.Create(storage.Document, "vault.rpv")
	if err != nil || !picked || created != want {
		t.Fatalf("a created document: %+v, %t, %v", created, picked, err)
	}
	existing, picked, err := files.Existing()
	if err != nil || !picked || existing != want {
		t.Fatalf("an existing document: %+v, %t, %v", existing, picked, err)
	}
	if want := []pick{{create: true, name: "vault.rpv"}, {create: false}}; !slices.Equal(provider.picks, want) {
		t.Fatalf("pickers shown: %+v, want %+v", provider.picks, want)
	}
}

func TestFilesReportACanceledOrFailedPick(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	provider.canceled = true
	files := Files{Provider: provider}
	if target, picked, err := files.Existing(); err != nil || picked || target != (storage.Target{}) {
		t.Fatalf("a canceled pick: %+v, %t, %v", target, picked, err)
	}
	provider.canceled, provider.pickErr = false, errGone
	if target, picked, err := files.Create(storage.Document, "vault.rpv"); !errors.Is(err, errGone) || picked || target != (storage.Target{}) {
		t.Fatalf("a failed pick: %+v, %t, %v", target, picked, err)
	}
}

func TestFilesCreateOnlyDocuments(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	_, picked, err := Files{Provider: provider}.Create(storage.LocalFile, "vault.rpv")
	if !errors.Is(err, storage.ErrUnsupportedKind) || picked || len(provider.picks) != 0 {
		t.Fatalf("a local file: picked %t, %v after %d pickers", picked, err, len(provider.picks))
	}
}
