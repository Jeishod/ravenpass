package api

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/dortanes/ravenpass/packages/app/backups"
	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/dortanes/ravenpass/packages/app/storage"
)

// fakeBackupFolders answers with its address, err or a cancel when address is empty, recording prompts.
type fakeBackupFolders struct {
	address  string
	label    storage.Label
	err      error
	prompts  []string
	choosing func()
}

func (f *fakeBackupFolders) Choose(prompt string) (string, storage.Label, bool, error) {
	f.prompts = append(f.prompts, prompt)
	if f.choosing != nil {
		f.choosing()
	}
	if f.err != nil {
		return "", storage.Label{}, false, f.err
	}
	return f.address, f.label, f.address != "", nil
}

// backingUp wires a keeper that writes plain paths into service, with folders as its picker.
func backingUp(t *testing.T, service *Service, folders BackupFolders) {
	t.Helper()
	keeper, err := backups.New(filepath.Join(t.TempDir(), "backups.json"), service.vault, service.preferences.AutoBackup, backups.Directory{})
	if err != nil {
		t.Fatal(err)
	}
	service.backups = keeper
	service.folders = folders
}

func autoBackup(t *testing.T, service *Service) AutoBackup {
	t.Helper()
	backup, err := service.GetAutoBackup()
	if err != nil {
		t.Fatal(err)
	}
	return backup
}

// backupFiles lists the names of the files in folder.
func backupFiles(t *testing.T, folder string) []string {
	t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}

func TestAHostWithoutBackupsOffersNone(t *testing.T) {
	service := newServiceOnHost(t, Host{})
	if service.Capabilities().AutoBackups {
		t.Fatal("a host without a keeper offers automatic backups")
	}
	_, err := service.GetAutoBackup()
	assertFailure(t, err, failureGeneral)
	assertFailure(t, service.SetAutoBackup(false, "daily", 5), failureGeneral)
	_, err = service.ChooseBackupFolder()
	assertFailure(t, err, failureGeneral)
}

func TestAHostWithAKeeperOffersBackups(t *testing.T) {
	ready := newReadyService(t)
	keeper, err := backups.New(filepath.Join(t.TempDir(), "backups.json"), ready.vault, ready.preferences.AutoBackup, backups.Directory{})
	if err != nil {
		t.Fatal(err)
	}
	if !newServiceOnHost(t, Host{Backups: keeper}).Capabilities().AutoBackups {
		t.Fatal("a host with a keeper does not offer automatic backups")
	}
}

func TestAutoBackupStartsOffDailyKeepingFiveWithoutAFolder(t *testing.T) {
	service := newReadyService(t)
	backingUp(t, service, &fakeBackupFolders{})
	got := autoBackup(t, service)
	want := AutoBackup{
		Intervals: []string{"daily", "weekly", "monthly"}, Interval: "daily",
		Keeps: []int{1, 3, 5, 10, 30}, Keep: 5,
	}
	if got.Enabled || got.Interval != want.Interval || !slices.Equal(got.Intervals, want.Intervals) ||
		got.Keep != want.Keep || !slices.Equal(got.Keeps, want.Keeps) || got.Folder != nil ||
		got.LastBackupAt != 0 || got.Failed {
		t.Fatalf("automatic backups on a new device = %+v", got)
	}
}

func TestUnofferedOrFolderlessChoicesAreRefused(t *testing.T) {
	service := newReadyService(t)
	backingUp(t, service, &fakeBackupFolders{})
	assertFailure(t, service.SetAutoBackup(true, "daily", 5), failureBackupFolderMissing)
	assertFailure(t, service.SetAutoBackup(false, "hourly", 5), failureBackupChoiceUnsupported)
	assertFailure(t, service.SetAutoBackup(false, "weekly", 4), failureBackupChoiceUnsupported)
	if got := autoBackup(t, service); got.Enabled || got.Interval != "daily" || got.Keep != 5 {
		t.Fatalf("a refused choice changed automatic backups to %+v", got)
	}
}

func TestTurningBackupsOnWritesTheFirstBackupBeforeAnswering(t *testing.T) {
	service := newReadyService(t)
	home := t.TempDir()
	folder := filepath.Join(home, "Backups")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	folders := &fakeBackupFolders{address: folder}
	backingUp(t, service, folders)

	chosen, err := service.ChooseBackupFolder()
	if err != nil || !chosen {
		t.Fatalf("choosing a folder = %t, %v", chosen, err)
	}
	if !slices.Equal(folders.prompts, []string{service.preferences.Dialogs().ChooseBackupFolder}) {
		t.Fatalf("the picker asked %q", folders.prompts)
	}
	got := autoBackup(t, service)
	if got.Enabled || got.Folder == nil || *got.Folder != (BackupFolder{Name: "Backups", Place: filepath.Base(home)}) {
		t.Fatalf("automatic backups after choosing a folder = %+v", got)
	}
	if files := backupFiles(t, folder); len(files) != 0 {
		t.Fatalf("choosing a folder with backups off wrote %q", files)
	}

	if err := service.SetAutoBackup(true, "weekly", 3); err != nil {
		t.Fatal(err)
	}
	files := backupFiles(t, folder)
	if len(files) != 1 {
		t.Fatalf("turning backups on wrote %q, want one backup", files)
	}
	written, err := os.ReadFile(filepath.Join(folder, files[0]))
	if err != nil {
		t.Fatal(err)
	}
	exported, _, err := service.vault.Export()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, exported) {
		t.Fatal("the backup is not the vault")
	}
	got = autoBackup(t, service)
	if !got.Enabled || got.Interval != "weekly" || got.Keep != 3 || got.LastBackupAt == 0 || got.Failed {
		t.Fatalf("automatic backups after the first backup = %+v", got)
	}
	if state := exportState(t, service); state != "current" {
		t.Fatalf("export state after the first backup = %q", state)
	}
}

func TestAnotherFolderIsBackedUpToAsSoonAsItIsChosen(t *testing.T) {
	service := newReadyService(t)
	first, second := t.TempDir(), t.TempDir()
	folders := &fakeBackupFolders{address: first}
	backingUp(t, service, folders)
	if _, err := service.ChooseBackupFolder(); err != nil {
		t.Fatal(err)
	}
	if err := service.SetAutoBackup(true, "daily", 5); err != nil {
		t.Fatal(err)
	}
	folders.address, folders.label = second, storage.Label{Name: "Ravenpass", Place: "Backup drive"}
	if _, err := service.ChooseBackupFolder(); err != nil {
		t.Fatal(err)
	}
	if len(backupFiles(t, first)) != 1 || len(backupFiles(t, second)) != 1 {
		t.Fatal("the new folder did not get its own backup beside the first folder's")
	}
	if got := autoBackup(t, service).Folder; got == nil || *got != (BackupFolder{Name: "Ravenpass", Place: "Backup drive"}) {
		t.Fatalf("the folder shows as %+v, want the picker's label", got)
	}
}

func TestACanceledOrFailedPickerRecordsNoFolder(t *testing.T) {
	service := newReadyService(t)
	folders := &fakeBackupFolders{}
	backingUp(t, service, folders)
	chosen, err := service.ChooseBackupFolder()
	if err != nil || chosen {
		t.Fatalf("a canceled picker = %t, %v", chosen, err)
	}
	folders.err = fs.ErrPermission
	_, err = service.ChooseBackupFolder()
	assertFailure(t, err, failurePermissionDenied)
	if got := autoBackup(t, service).Folder; got != nil {
		t.Fatalf("a canceled or failed picker recorded %+v", got)
	}
}

func TestTheHostIsHeldWhileTheFolderPickerShows(t *testing.T) {
	service := newReadyService(t)
	held, released := 0, 0
	service.hold = func() (release func()) {
		held++
		return func() { released++ }
	}
	backingUp(t, service, &fakeBackupFolders{address: t.TempDir(), choosing: func() {
		if held-released != 1 {
			t.Error("the folder picker showed with the host released")
		}
	}})
	if _, err := service.ChooseBackupFolder(); err != nil {
		t.Fatal(err)
	}
	if held != 1 || released != 1 {
		t.Fatalf("held %d times and released %d, want once each", held, released)
	}
}

func TestAFolderThatCannotBeWrittenReportsTheFailure(t *testing.T) {
	service := newReadyService(t)
	backingUp(t, service, &fakeBackupFolders{address: filepath.Join(t.TempDir(), "unplugged")})
	if _, err := service.ChooseBackupFolder(); err != nil {
		t.Fatal(err)
	}
	if err := service.SetAutoBackup(true, "daily", 5); err != nil {
		t.Fatal(err)
	}
	if got := autoBackup(t, service); !got.Failed || got.LastBackupAt != 0 {
		t.Fatalf("automatic backups into a missing folder = %+v", got)
	}
}

func TestUnlockingAsksForADueBackupWithoutWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := newReadyServiceOnDevice(t)
		folder := t.TempDir()
		backingUp(t, service, &fakeBackupFolders{})
		if err := service.Lock(); err != nil {
			t.Fatal(err)
		}
		if err := service.preferences.SetBackupFolder(preferences.BackupFolder{Address: folder, Name: "Backups", Place: "owner"}); err != nil {
			t.Fatal(err)
		}
		if err := service.preferences.SetAutoBackup(true, preferences.BackupDaily, 5); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan struct{})
		go func() {
			service.backups.Run(ctx)
			close(stopped)
		}()
		defer func() {
			cancel()
			<-stopped
		}()
		if err := service.Unlock(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if autoBackup(t, service).LastBackupAt == 0 {
			t.Fatal("unlocking did not back up the vault")
		}
		if files := backupFiles(t, folder); len(files) != 1 {
			t.Fatalf("unlocking wrote %q, want one backup", files)
		}
	})
}
