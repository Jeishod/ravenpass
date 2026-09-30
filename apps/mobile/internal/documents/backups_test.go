package documents

import (
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/storage"
)

const backupName = "Personal 2026-09-25 10-30-00.rpv"

func TestBackupFoldersChooseWithTheFolderPicker(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	provider.label = storage.Label{Name: "Backups", Place: "Drive"}
	chosen, label, picked, err := BackupFolders{Provider: provider}.Choose("Choose a folder for automatic backups")
	if err != nil || !picked || chosen != folder || label != provider.label {
		t.Fatalf("choosing a folder: %q, %+v, %t, %v", chosen, label, picked, err)
	}
	if want := []pick{{folder: true}}; !slices.Equal(provider.picks, want) {
		t.Fatalf("pickers shown: %+v, want %+v", provider.picks, want)
	}
}

func TestBackupFoldersReportACanceledOrFailedChoice(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	provider.canceled = true
	if chosen, _, picked, err := (BackupFolders{Provider: provider}).Choose(""); err != nil || picked || chosen != "" {
		t.Fatalf("a canceled choice: %q, %t, %v", chosen, picked, err)
	}
	provider.canceled, provider.pickErr = false, errGone
	if chosen, _, picked, err := (BackupFolders{Provider: provider}).Choose(""); !errors.Is(err, errGone) || picked || chosen != "" {
		t.Fatalf("a failed choice: %q, %t, %v", chosen, picked, err)
	}
}

func TestABackupIsSavedInANewDocumentOfTheFolder(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	folders := BackupFolders{Provider: provider}
	first, err := folders.Save(folder, backupName, []byte("vault"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := folders.Save(folder, backupName, []byte("next"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second || string(provider.documents[first]) != "vault" || string(provider.documents[second]) != "next" {
		t.Fatalf("documents %v after two backups at %q and %q", provider.documents, first, second)
	}
	want := creation{folder: folder, name: backupName, mediaType: "application/octet-stream"}
	if !slices.Equal(provider.creations, []creation{want, want}) || len(provider.deleted) != 0 {
		t.Fatalf("created %+v, deleted %v", provider.creations, provider.deleted)
	}
}

func TestAFailedBackupLeavesNoDocument(t *testing.T) {
	for name, test := range map[string]struct {
		createErr error
		failWrite error
		kept      int
		altered   bool
		want      error
		deleted   int
	}{
		"not created":           {createErr: errGone, want: errGone},
		"write untouched":       {failWrite: errUpload, kept: untouched, want: errUpload, deleted: 1},
		"write partial":         {failWrite: errUpload, kept: 2, want: errUpload, deleted: 1},
		"read back other bytes": {altered: true, want: api.ErrFileUnverified, deleted: 1},
	} {
		t.Run(name, func(t *testing.T) {
			provider := newDrive(map[string][]byte{})
			provider.createErr, provider.failWrite, provider.kept, provider.altered =
				test.createErr, test.failWrite, test.kept, test.altered
			saved, err := BackupFolders{Provider: provider}.Save(folder, backupName, []byte("vault"))
			if !errors.Is(err, test.want) || saved != "" {
				t.Fatalf("a failed backup: %q, %v, want %v", saved, err, test.want)
			}
			if len(provider.documents) != 0 || len(provider.deleted) != test.deleted {
				t.Fatalf("documents %v after a failed backup, deleted %v", provider.documents, provider.deleted)
			}
		})
	}
}

func TestRemoveDeletesABackup(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	folders := BackupFolders{Provider: provider}
	saved, err := folders.Save(folder, backupName, []byte("vault"))
	if err != nil {
		t.Fatal(err)
	}
	if err := folders.Remove(saved); err != nil || len(provider.documents) != 0 || !slices.Equal(provider.deleted, []string{saved}) {
		t.Fatalf("removing a backup: %v, documents %v, deleted %v", err, provider.documents, provider.deleted)
	}
	if err := folders.Remove(saved); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removing a backup that is gone: got %v, want fs.ErrNotExist", err)
	}
}
