package api

import (
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

func TestTheHostIsHeldWhileEveryPickerShows(t *testing.T) {
	held, released := 0, 0
	service := newServiceOnHost(t, Host{
		Files:  &pickedFiles{kinds: []storage.Kind{storage.Document}},
		Photos: &fakePhotoPicker{},
		Saver:  &fakeSaver{target: &fakeTarget{}},
		Hold: func() (release func()) {
			held++
			return func() { released++ }
		},
	})
	if _, _, err := service.files.Existing(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.files.Create(storage.Document, "Ravenpass.rpv"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.photoPicker.Pick(1); err != nil {
		t.Fatal(err)
	}
	saved, err := service.saveFile(SavedFile{Name: "scan.pdf"}, func(target SaveTarget) error {
		if held-released != 1 {
			t.Error("a save wrote its file with the host released")
		}
		return target.Write([]byte("scan"))
	})
	if err != nil || !saved {
		t.Fatalf("saving a file = %t, error = %v", saved, err)
	}
	if held != 4 || released != 4 {
		t.Fatalf("held %d times and released %d, want 4 each", held, released)
	}
}
