package api

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/dortanes/ravenpass/packages/importers/bitwarden"
)

func TestOnlyATemporaryPickIsDiscarded(t *testing.T) {
	kept := writeExport(t, "kept")
	pickedFile{path: kept}.discard()
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("the owner's own file was removed: %v", err)
	}
	copied := writeExport(t, "copy")
	pickedFile{path: copied, temporary: true}.discard()
	if _, err := os.Stat(copied); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the dialog's copy stayed: %v", err)
	}
	pickedFile{path: copied, temporary: true}.discard()
}

func TestATemporaryImportPickIsDeletedOnceReadAndNotOfferedToTheTrash(t *testing.T) {
	service := newReadyService(t)
	copied := writeExport(t, bitwardenExport)
	choice, err := service.stageImport(pickedFile{path: copied, temporary: true}, bitwarden.Open)
	if err != nil || !choice.Chosen || choice.Preview.Items == 0 {
		t.Fatalf("choice = %+v, error = %v", choice, err)
	}
	if _, err := os.Stat(copied); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the dialog's copy of the export stayed: %v", err)
	}
	result, err := service.ImportItems(ImportOptions{})
	if err != nil || result.Located {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	assertFailure(t, service.TrashImportFile(), failureImportNotActive)
}

func TestATemporaryScanPickIsDeletedEvenWhenRefused(t *testing.T) {
	copied := writeExport(t, "not a picture")
	picked := pickedFile{path: copied, temporary: true}
	if _, err := readChosenFile(picked.path, 1); err == nil {
		t.Fatal("an oversized file was read")
	}
	picked.discard()
	if _, err := os.Stat(copied); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the dialog's copy stayed: %v", err)
	}
}
