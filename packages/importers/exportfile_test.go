package importers

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

type fakeReadable struct {
	label string
	wiped bool
}

func (r *fakeReadable) Read(Labels) (Export, error) {
	return Export{Items: []Item{{Content: vault.NewItem{Note: &vault.NoteInput{Label: r.label}}}}}, nil
}

func (r *fakeReadable) Wipe() { r.wiped = true }

// fakeSealed unseals with password into unsealed.
type fakeSealed struct {
	password string
	unsealed *ExportFile
}

func (s fakeSealed) Unseal(password string) (*ExportFile, error) {
	if password != s.password {
		return nil, ErrWrongPassword
	}
	return s.unsealed, nil
}

func TestALockedFileReadsOnlyOnceTheRightPasswordUnsealsIt(t *testing.T) {
	inside := &fakeReadable{label: "Inside"}
	file := NewLockedExportFile(FormatEncryptedZIP, fakeSealed{password: "right", unsealed: NewExportFile(FormatZIP, inside, 2)}, 1)
	if !file.Locked() || file.Format() != FormatEncryptedZIP {
		t.Fatalf("locked = %v, format = %s", file.Locked(), file.Format())
	}
	if _, err := file.Read(Labels{}); !errors.Is(err, ErrLocked) {
		t.Fatalf("Read while locked = %v, want ErrLocked", err)
	}
	if err := file.Unlock("wrong"); !errors.Is(err, ErrWrongPassword) || !file.Locked() {
		t.Fatalf("a wrong password: %v, locked = %v", err, file.Locked())
	}
	if err := file.Unlock("right"); err != nil || file.Locked() {
		t.Fatalf("the right password: %v, locked = %v", err, file.Locked())
	}
	if err := file.Unlock("anything"); err != nil {
		t.Fatalf("Unlock of an unlocked file = %v", err)
	}
	export, err := file.Read(Labels{})
	if err != nil {
		t.Fatal(err)
	}
	if export.Format != FormatEncryptedZIP || export.Attachments != 3 || len(export.Items) != 1 || export.Items[0].Content.Note.Label != "Inside" {
		t.Fatalf("export = %+v", export)
	}
}

func TestSealedContentThatIsItselfLockedIsUnrecognized(t *testing.T) {
	nested := NewLockedExportFile(FormatEncryptedJSON, fakeSealed{password: "right"}, 0)
	file := NewLockedExportFile(FormatEncryptedJSON, fakeSealed{password: "right", unsealed: nested}, 0)
	if err := file.Unlock("right"); !errors.Is(err, ErrUnrecognized) || !file.Locked() {
		t.Fatalf("nested sealing: %v, locked = %v", err, file.Locked())
	}
}

func TestCloseWipesTheTextAndEndsTheFile(t *testing.T) {
	plain := &fakeReadable{}
	unsealed := &fakeReadable{}
	unlocked := NewLockedExportFile(FormatEncryptedJSON, fakeSealed{password: "right", unsealed: NewExportFile(FormatJSON, unsealed, 0)}, 0)
	if err := unlocked.Unlock("right"); err != nil {
		t.Fatal(err)
	}
	files := map[string]struct {
		file *ExportFile
		text *fakeReadable
	}{
		"plain":    {NewExportFile(FormatCSV, plain, 0), plain},
		"unlocked": {unlocked, unsealed},
		"locked":   {NewLockedExportFile(FormatEncryptedZIP, fakeSealed{password: "right"}, 0), nil},
	}
	for name, test := range files {
		test.file.Close()
		if test.text != nil && !test.text.wiped {
			t.Errorf("%s: the text survives Close", name)
		}
		if _, err := test.file.Read(Labels{}); !errors.Is(err, fs.ErrClosed) {
			t.Errorf("%s: Read after Close = %v", name, err)
		}
		if err := test.file.Unlock("right"); !errors.Is(err, fs.ErrClosed) {
			t.Errorf("%s: Unlock after Close = %v", name, err)
		}
	}
}
