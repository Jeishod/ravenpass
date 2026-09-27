package documents

import (
	"errors"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/api"
)

var scanFile = api.SavedFile{Prompt: "Save a copy of the scan", Name: "passport.pdf", MediaType: "application/pdf"}

func TestSavesWriteANewDocumentOfTheFilesType(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	target, chosen, err := Saves{Provider: provider}.Choose(scanFile)
	if err != nil || !chosen {
		t.Fatalf("choosing where to save: %t, %v", chosen, err)
	}
	if want := []pick{{create: true, name: "passport.pdf", mediaType: "application/pdf"}}; !slices.Equal(provider.picks, want) {
		t.Fatalf("pickers shown: %+v, want %+v", provider.picks, want)
	}
	if err := target.Write([]byte("scan")); err != nil {
		t.Fatal(err)
	}
	if string(provider.documents[address]) != "scan" || provider.writes != 1 || len(provider.deleted) != 0 {
		t.Fatalf("document %q after %d writes, deleted %v", provider.documents[address], provider.writes, provider.deleted)
	}
}

func TestSavesReportACanceledOrFailedChoice(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	provider.canceled = true
	target, chosen, err := Saves{Provider: provider}.Choose(scanFile)
	if err != nil || chosen || target != nil || len(provider.documents) != 0 {
		t.Fatalf("a canceled choice: %v, %t, %v, documents %v", target, chosen, err, provider.documents)
	}
	provider.canceled, provider.pickErr = false, errGone
	if target, chosen, err := (Saves{Provider: provider}).Choose(scanFile); !errors.Is(err, errGone) || chosen || target != nil {
		t.Fatalf("a failed choice: %v, %t, %v", target, chosen, err)
	}
}

func TestAFailedSaveIsDiscardedWithWhatItWrote(t *testing.T) {
	for name, test := range map[string]struct {
		failWrite error
		kept      int
		altered   bool
		want      error
	}{
		"untouched":             {failWrite: errUpload, kept: untouched, want: errUpload},
		"partial":               {failWrite: errUpload, kept: 2, want: errUpload},
		"read back other bytes": {altered: true, want: api.ErrFileUnverified},
	} {
		t.Run(name, func(t *testing.T) {
			provider := newDrive(map[string][]byte{})
			target, _, err := Saves{Provider: provider}.Choose(scanFile)
			if err != nil {
				t.Fatal(err)
			}
			provider.failWrite, provider.kept, provider.altered = test.failWrite, test.kept, test.altered
			if err := target.Write([]byte("scan")); !errors.Is(err, test.want) {
				t.Fatalf("a failed write: got %v, want %v", err, test.want)
			}
			target.Discard()
			if _, found := provider.documents[address]; found || !slices.Equal(provider.deleted, []string{address}) {
				t.Fatalf("documents %v after discarding, deleted %v", provider.documents, provider.deleted)
			}
		})
	}
}
