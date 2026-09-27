package api

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

// fakeSaver answers with its target, a cancel or err, recording the files it was asked to save.
type fakeSaver struct {
	target *fakeTarget
	err    error
	files  []SavedFile
}

func (f *fakeSaver) Choose(file SavedFile) (SaveTarget, bool, error) {
	f.files = append(f.files, file)
	if f.err != nil || f.target == nil {
		return nil, false, f.err
	}
	return f.target, true, nil
}

// fakeTarget keeps what is written to it, or fails every write with its error.
type fakeTarget struct {
	err       error
	written   []byte
	discarded bool
}

func (f *fakeTarget) Write(data []byte) error {
	if f.err != nil {
		return f.err
	}
	f.written = bytes.Clone(data)
	return nil
}

func (f *fakeTarget) Discard() { f.discarded = true }

func exportState(t *testing.T, service *Service) string {
	t.Helper()
	status, err := service.ExportStatus()
	if err != nil {
		t.Fatal(err)
	}
	return status.State
}

func TestAHostSaverIsOffered(t *testing.T) {
	if newServiceOnHost(t, Host{}).Capabilities().SaveFiles {
		t.Fatal("a host that did not offer its save dialog offers saving files")
	}
	if !newServiceOnHost(t, Host{Saver: &fakeSaver{}}).Capabilities().SaveFiles {
		t.Fatal("a host with a saver does not offer saving files")
	}
}

func TestTheDesktopSavesWithItsSaveDialog(t *testing.T) {
	service, _, pdf := storedScanIdentity(t)
	_, err := service.SaveScan(pdf)
	assertFailure(t, err, failureWindowUnavailable)
	assertFailure(t, service.ExportEncryptedCopy(), failureWindowUnavailable)
}

func TestAScanIsSavedAsItsMediaTypeUnderItsName(t *testing.T) {
	service, _, pdf := storedScanIdentity(t)
	target := &fakeTarget{}
	saver := &fakeSaver{target: target}
	service.saver = saver
	saved, err := service.SaveScan(pdf)
	if err != nil || !saved.Saved {
		t.Fatalf("saving a scan = %+v, error = %v", saved, err)
	}
	if !bytes.Equal(target.written, photoFixture(t, "scan.pdf")) || target.discarded {
		t.Fatalf("the saved copy holds %d bytes, discarded %t", len(target.written), target.discarded)
	}
	want := SavedFile{Prompt: service.preferences.Dialogs().SaveScan, Name: "scan.pdf", MediaType: vault.MediaPDF}
	if !slices.Equal(saver.files, []SavedFile{want}) {
		t.Fatalf("files asked for = %+v, want %+v", saver.files, want)
	}
}

func TestTheEncryptedCopyIsSavedAsAVaultAndRecorded(t *testing.T) {
	service := newReadyService(t)
	target := &fakeTarget{}
	saver := &fakeSaver{target: target}
	service.saver = saver
	if err := service.ExportEncryptedCopy(); err != nil {
		t.Fatal(err)
	}
	exported, _, err := service.vault.Export()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(target.written, exported) || target.discarded {
		t.Fatalf("the saved copy is not the vault, discarded %t", target.discarded)
	}
	dialogs := service.preferences.Dialogs()
	want := SavedFile{
		Prompt: dialogs.SaveExport, Name: dialogs.ExportFileName, MediaType: "application/octet-stream",
		Filter: dialogs.VaultFilter, Pattern: "*.rpv",
	}
	if !slices.Equal(saver.files, []SavedFile{want}) {
		t.Fatalf("files asked for = %+v, want %+v", saver.files, want)
	}
	if state := exportState(t, service); state != "current" {
		t.Fatalf("export state after a saved copy = %q", state)
	}
}

func TestTheRecoveryKeyIsSavedAsText(t *testing.T) {
	home := t.TempDir()
	service, files := newServiceWithStorage(t, filepath.Join(home, "storage.json"), filepath.Join(home, "vault.rpv"))
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	phrase, err := service.BeginCreation()
	if err != nil {
		t.Fatal(err)
	}
	target := &fakeTarget{}
	saver := &fakeSaver{target: target}
	service.saver = saver
	if err := service.SaveRecoveryKey(phrase); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(target.written), phrase) || target.discarded {
		t.Fatalf("the saved file does not hold the recovery key, discarded %t", target.discarded)
	}
	dialogs := service.preferences.Dialogs()
	want := SavedFile{
		Prompt: dialogs.SaveRecoveryKey, Name: dialogs.RecoveryFileName, MediaType: "text/plain",
		Filter: dialogs.TextFilter, Pattern: "*.txt",
	}
	if !slices.Equal(saver.files, []SavedFile{want}) {
		t.Fatalf("files asked for = %+v, want %+v", saver.files, want)
	}

	refused := &fakeTarget{}
	service.saver = &fakeSaver{target: refused}
	assertFailure(t, service.SaveRecoveryKey("other words"), failureRecoveryKeyMismatch)
	if refused.written != nil || refused.discarded {
		t.Fatal("a recovery key that does not match asked where to save it")
	}
	service.saver = &fakeSaver{}
	assertFailure(t, service.SaveRecoveryKey(phrase), failureRecoveryKeyNotSaved)
	failed := &fakeTarget{err: ErrFileUnverified}
	service.saver = &fakeSaver{target: failed}
	assertFailure(t, service.SaveRecoveryKey(phrase), failureFileUnverified)
	if !failed.discarded {
		t.Fatal("a recovery key file that failed to write was kept")
	}
}

func TestACanceledSaveWritesNothing(t *testing.T) {
	service, _, pdf := storedScanIdentity(t)
	service.saver = &fakeSaver{}
	saved, err := service.SaveScan(pdf)
	if err != nil || saved.Saved {
		t.Fatalf("a canceled scan save = %+v, error = %v", saved, err)
	}
	assertFailure(t, service.ExportEncryptedCopy(), failureExportCanceled)
	if state := exportState(t, service); state != "unknown" {
		t.Fatalf("export state after a canceled copy = %q", state)
	}
}

func TestAFailedSaveIsDiscardedAndUnrecorded(t *testing.T) {
	for name, test := range map[string]struct {
		err  error
		want failure
	}{
		"read back other content": {ErrFileUnverified, failureFileUnverified},
		"refused":                 {fs.ErrPermission, failurePermissionDenied},
		"failed":                  {errors.New("the provider did not answer"), failureGeneral},
	} {
		t.Run(name, func(t *testing.T) {
			service, _, pdf := storedScanIdentity(t)
			scan := &fakeTarget{err: test.err}
			service.saver = &fakeSaver{target: scan}
			_, err := service.SaveScan(pdf)
			assertFailure(t, err, test.want)
			copied := &fakeTarget{err: test.err}
			service.saver = &fakeSaver{target: copied}
			assertFailure(t, service.ExportEncryptedCopy(), test.want)
			if !scan.discarded || !copied.discarded {
				t.Fatalf("discarded the scan %t and the copy %t, want both", scan.discarded, copied.discarded)
			}
			if state := exportState(t, service); state != "unknown" {
				t.Fatalf("export state after a failed copy = %q", state)
			}
		})
	}
}

func TestAFailedChoiceSavesNothing(t *testing.T) {
	service, _, pdf := storedScanIdentity(t)
	service.saver = &fakeSaver{err: errors.New("the picker cannot be shown now")}
	_, err := service.SaveScan(pdf)
	assertFailure(t, err, failureGeneral)
	assertFailure(t, service.ExportEncryptedCopy(), failureGeneral)
}

func TestALockedVaultAsksNowhereToSaveItsCopy(t *testing.T) {
	service := newReadyService(t)
	saver := &fakeSaver{target: &fakeTarget{}}
	service.saver = saver
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	assertFailure(t, service.ExportEncryptedCopy(), failureVaultLocked)
	if len(saver.files) != 0 {
		t.Fatal("a locked vault asked where to save its copy")
	}
}
