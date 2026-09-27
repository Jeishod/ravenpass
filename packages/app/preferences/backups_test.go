package preferences

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

var testBackupFolder = BackupFolder{Address: "/Volumes/Backup/Ravenpass", Name: "Ravenpass", Place: "Backup"}

func TestAutoBackupDefaultsToOffDailyKeepingFiveWithoutAFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	want := AutoBackup{Enabled: false, Interval: BackupDaily, Keep: 5}
	if got := newStore(t, path).AutoBackup(); got != want {
		t.Fatalf("automatic backups without a record = %+v", got)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"ru","clipboardKept":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := newStore(t, path).AutoBackup(); got != want {
		t.Fatalf("automatic backups of a record that does not name them = %+v", got)
	}
}

func TestBackupChoicesAreOfferedInOrder(t *testing.T) {
	if got := BackupIntervals(); !slices.Equal(got, []BackupInterval{BackupDaily, BackupWeekly, BackupMonthly}) {
		t.Fatalf("offered intervals = %v", got)
	}
	if got := BackupKeeps(); !slices.Equal(got, []int{1, 3, 5, 10, 30}) {
		t.Fatalf("offered counts = %v", got)
	}
	for interval, every := range map[BackupInterval]time.Duration{
		BackupDaily:   24 * time.Hour,
		BackupWeekly:  7 * 24 * time.Hour,
		BackupMonthly: 30 * 24 * time.Hour,
	} {
		if got := interval.Every(); got != every {
			t.Fatalf("%s runs every %v, want %v", interval, got, every)
		}
	}
}

func TestAutoBackupSurvivesOtherChangesAndAReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetBackupFolder(testBackupFolder); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoBackup(true, BackupWeekly, 10); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoLock(AutoLock{Enabled: false, After: 5 * time.Minute}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLanguage(Russian); err != nil {
		t.Fatal(err)
	}
	want := AutoBackup{Enabled: true, Interval: BackupWeekly, Keep: 10, Folder: testBackupFolder}
	reopened := newStore(t, path)
	if got := reopened.AutoBackup(); got != want {
		t.Fatalf("automatic backups after a restart = %+v, want %+v", got, want)
	}
	if got := reopened.AutoLock(); got.Enabled || got.After != 5*time.Minute {
		t.Fatalf("recording automatic backups lost the automatic lock: %+v", got)
	}

	if err := reopened.SetAutoBackup(false, BackupMonthly, 1); err != nil {
		t.Fatal(err)
	}
	other := BackupFolder{Address: "/Users/owner/Backups", Name: "Backups", Place: "owner"}
	if err := reopened.SetBackupFolder(other); err != nil {
		t.Fatal(err)
	}
	want = AutoBackup{Enabled: false, Interval: BackupMonthly, Keep: 1, Folder: other}
	if got := newStore(t, path).AutoBackup(); got != want {
		t.Fatalf("automatic backups turned off in another folder = %+v, want %+v", got, want)
	}
}

func TestChoosingAFolderKeepsBackupsOnAsTheyWere(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "preferences.json"))
	if err := store.SetBackupFolder(testBackupFolder); err != nil {
		t.Fatal(err)
	}
	if store.AutoBackup().Enabled {
		t.Fatal("choosing a folder turned backups on")
	}
	if err := store.SetAutoBackup(true, BackupDaily, 5); err != nil {
		t.Fatal(err)
	}
	other := BackupFolder{Address: "/Users/owner/Backups", Name: "Backups", Place: "owner"}
	if err := store.SetBackupFolder(other); err != nil {
		t.Fatal(err)
	}
	if got := store.AutoBackup(); !got.Enabled || got.Folder != other {
		t.Fatalf("automatic backups after choosing another folder = %+v", got)
	}
}

func TestUnofferedBackupChoicesAreRefusedWithoutAWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	for _, choice := range []struct {
		interval BackupInterval
		keep     int
	}{{"hourly", 5}, {"", 5}, {BackupDaily, 0}, {BackupDaily, 2}, {BackupDaily, -1}, {BackupWeekly, 100}} {
		if err := store.SetAutoBackup(false, choice.interval, choice.keep); !errors.Is(err, ErrUnsupportedBackupChoice) {
			t.Fatalf("%q keeping %d: got %v, want ErrUnsupportedBackupChoice", choice.interval, choice.keep, err)
		}
	}
	if err := store.SetAutoBackup(true, BackupDaily, 5); !errors.Is(err, ErrBackupFolderMissing) {
		t.Fatalf("turning backups on without a folder: got %v, want ErrBackupFolderMissing", err)
	}
	if err := store.SetBackupFolder(BackupFolder{Name: "Backups", Place: "owner"}); !errors.Is(err, ErrBackupFolderMissing) {
		t.Fatalf("a folder without an address: got %v, want ErrBackupFolderMissing", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused choice wrote a record: %v", err)
	}
}

func TestUnofferedBackupChoicesAreForgottenOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	for name, test := range map[string]struct {
		record string
		want   AutoBackup
	}{
		"unoffered interval and count": {
			`{"version":1,"autoBackupOn":true,"autoBackupInterval":"hourly","autoBackupKeep":7,"autoBackupFolder":{"address":"/Volumes/Backup/Ravenpass","name":"Ravenpass","place":"Backup"}}`,
			AutoBackup{Enabled: true, Interval: BackupDaily, Keep: 5, Folder: testBackupFolder},
		},
		"on without a folder": {
			`{"version":1,"autoBackupOn":true,"autoBackupInterval":"weekly","autoBackupKeep":3}`,
			AutoBackup{Enabled: false, Interval: BackupWeekly, Keep: 3},
		},
		"folder without an address": {
			`{"version":1,"autoBackupOn":true,"autoBackupFolder":{"address":"","name":"Ravenpass","place":"Backup"}}`,
			AutoBackup{Enabled: false, Interval: BackupDaily, Keep: 5},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(test.record), 0600); err != nil {
				t.Fatal(err)
			}
			if got := newStore(t, path).AutoBackup(); got != test.want {
				t.Fatalf("automatic backups = %+v, want %+v", got, test.want)
			}
		})
	}
}
