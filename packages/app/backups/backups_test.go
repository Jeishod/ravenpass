package backups

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	folderA = "/Volumes/Backup/Ravenpass"
	folderB = "/Users/owner/Backups"
)

// fakeVault is an open or locked vault whose head changes when the test says so.
type fakeVault struct {
	mu          sync.Mutex
	unlocked    bool
	head        vault.Head
	target      storage.Target
	exportErr   error
	afterExport func(*fakeVault)
	lockedReads int
	exports     int
	recorded    []vault.Head
	digests     [][32]byte
}

func (f *fakeVault) Unlocked() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.unlocked
}

func (f *fakeVault) State() (vaultservice.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.unlocked {
		f.lockedReads++
		return vaultservice.State{Phase: vaultservice.PhaseLocked, VaultExists: true}, nil
	}
	return vaultservice.State{Phase: vaultservice.PhaseReady, VaultExists: true, Head: f.head}, nil
}

func (f *fakeVault) Snapshot() ([]byte, vault.Head, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exports++
	if f.exportErr != nil {
		return nil, vault.Head{}, f.exportErr
	}
	head := f.head
	if f.afterExport != nil {
		f.afterExport(f)
	}
	return containerOf(head), head, nil
}

func (f *fakeVault) RecordExport(head vault.Head, digest [32]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recorded = append(f.recorded, head)
	f.digests = append(f.digests, digest)
	return nil
}

func (f *fakeVault) Storage() storage.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return storage.Status{Current: f.target, Available: true}
}

// change makes the vault's next revision.
func (f *fakeVault) change() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.head.Revision++
	f.head.Hash = sha256.Sum256(containerOf(f.head))
}

func containerOf(head vault.Head) []byte {
	return fmt.Appendf(nil, "container of %s at %d", head.VaultID, head.Revision)
}

// fakeFolders holds files by address, a folder's path joined with the file's name.
type fakeFolders struct {
	mu        sync.Mutex
	files     map[string][]byte
	saveErr   error
	removeErr map[string]error
	saved     chan string
}

func (f *fakeFolders) Save(folder, name string, data []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return "", f.saveErr
	}
	address := path.Join(folder, name)
	if _, exists := f.files[address]; exists {
		return "", fs.ErrExist
	}
	f.files[address] = bytes.Clone(data)
	if f.saved != nil {
		f.saved <- address
	}
	return address, nil
}

func (f *fakeFolders) Remove(address string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.removeErr[address]; err != nil {
		return err
	}
	if _, exists := f.files[address]; !exists {
		return fs.ErrNotExist
	}
	delete(f.files, address)
	return nil
}

func (f *fakeFolders) addresses() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Sorted(maps.Keys(f.files))
}

// harness is a keeper over a fake vault and fake folders, with a clock the test moves.
type harness struct {
	t        *testing.T
	vault    *fakeVault
	folders  *fakeFolders
	settings preferences.AutoBackup
	clock    time.Time
	journal  string
	keeper   *Keeper
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		t: t,
		vault: &fakeVault{
			unlocked: true,
			head:     vault.Head{VaultID: vault.ID{0xa1}, Revision: 1, Hash: [32]byte{1}},
			target:   storage.Target{Kind: storage.LocalFile, Path: "/Users/owner/Ravenpass/Personal.rpv"},
		},
		folders:  &fakeFolders{files: map[string][]byte{}, removeErr: map[string]error{}},
		settings: preferences.AutoBackup{Enabled: true, Interval: preferences.BackupDaily, Keep: 5, Folder: preferences.BackupFolder{Address: folderA, Name: "Ravenpass", Place: "Backup"}},
		clock:    time.Date(2026, 9, 25, 10, 30, 0, 0, time.Local),
		journal:  filepath.Join(t.TempDir(), "backups.json"),
	}
	h.keeper = h.newKeeper()
	return h
}

func (h *harness) newKeeper() *Keeper {
	h.t.Helper()
	keeper, err := New(h.journal, h.vault, func() preferences.AutoBackup { return h.settings }, h.folders)
	if err != nil {
		h.t.Fatal(err)
	}
	keeper.now = func() time.Time { return h.clock }
	return keeper
}

func (h *harness) advance(d time.Duration) { h.clock = h.clock.Add(d) }

// backupName is the name of a backup of the Personal vault written at clock.
func backupName(at time.Time) string {
	return "Personal " + at.Format("2006-01-02 15-04-05") + ".rpv"
}

func (h *harness) assertFiles(want ...string) {
	h.t.Helper()
	slices.Sort(want)
	if got := h.folders.addresses(); !slices.Equal(got, want) {
		h.t.Fatalf("folders hold %q, want %q", got, want)
	}
}

func (h *harness) assertExports(want int) {
	h.t.Helper()
	if h.vault.exports != want {
		h.t.Fatalf("the vault was exported %d times, want %d", h.vault.exports, want)
	}
}

func TestNewNeedsEveryDependency(t *testing.T) {
	settings := func() preferences.AutoBackup { return preferences.AutoBackup{} }
	journal := filepath.Join(t.TempDir(), "backups.json")
	for name, build := range map[string]func() (*Keeper, error){
		"journal":     func() (*Keeper, error) { return New("", &fakeVault{}, settings, Directory{}) },
		"vault":       func() (*Keeper, error) { return New(journal, nil, settings, Directory{}) },
		"settings":    func() (*Keeper, error) { return New(journal, &fakeVault{}, nil, Directory{}) },
		"destination": func() (*Keeper, error) { return New(journal, &fakeVault{}, settings, nil) },
	} {
		if _, err := build(); err == nil {
			t.Fatalf("a keeper without its %s was made", name)
		}
	}
}

func TestTheFirstBackupIsWrittenRecordedAndReported(t *testing.T) {
	h := newHarness(t)
	h.keeper.Check()
	address := path.Join(folderA, backupName(h.clock))
	h.assertFiles(address)
	head := h.vault.head
	if !bytes.Equal(h.folders.files[address], containerOf(head)) {
		t.Fatal("the backup is not the exported container")
	}
	if !slices.Equal(h.vault.recorded, []vault.Head{head}) || h.vault.digests[0] != sha256.Sum256(containerOf(head)) {
		t.Fatalf("recorded exports = %v", h.vault.recorded)
	}
	if status := h.keeper.Status(); !status.Last.Equal(h.clock) || status.Failed {
		t.Fatalf("status after the first backup = %+v", status)
	}
	info, err := os.Stat(h.journal)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("journal permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestNothingIsBackedUpWhileOffWithoutAFolderOrLocked(t *testing.T) {
	for name, prepare := range map[string]func(*harness){
		"off":         func(h *harness) { h.settings.Enabled = false },
		"no folder":   func(h *harness) { h.settings.Folder = preferences.BackupFolder{} },
		"locked":      func(h *harness) { h.vault.unlocked = false },
		"locked, off": func(h *harness) { h.vault.unlocked, h.settings.Enabled = false, false },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			prepare(h)
			h.keeper.Check()
			h.keeper.check(false)
			h.assertExports(0)
			h.assertFiles()
			if h.vault.lockedReads != 0 {
				t.Fatalf("a locked vault's file was read %d times", h.vault.lockedReads)
			}
			if status := h.keeper.Status(); status != (Status{}) {
				t.Fatalf("status = %+v, want none", status)
			}
		})
	}
}

func TestABackupWaitsForTheIntervalAndAChange(t *testing.T) {
	h := newHarness(t)
	h.keeper.Check()
	first := h.clock

	h.advance(48 * time.Hour)
	h.keeper.check(false)
	h.assertExports(1)

	h.vault.change()
	h.advance(-25 * time.Hour)
	h.keeper.check(false)
	h.assertExports(1)

	h.advance(time.Hour)
	h.keeper.check(false)
	h.assertFiles(path.Join(folderA, backupName(first)), path.Join(folderA, backupName(h.clock)))

	h.settings.Interval = preferences.BackupWeekly
	h.vault.change()
	h.advance(6 * 24 * time.Hour)
	h.keeper.Check()
	h.assertExports(2)
	h.advance(24 * time.Hour)
	h.keeper.Check()
	h.assertExports(3)
}

func TestAKeyChangeBacksUpAtOnceWhateverTheInterval(t *testing.T) {
	h := newHarness(t)
	h.keeper.Check()
	h.vault.change()
	h.advance(time.Minute)
	h.keeper.check(false)
	h.assertExports(1)

	h.keeper.KeyChanged(vault.ID{0xb2})
	h.keeper.check(false)
	h.assertExports(1)

	h.keeper.KeyChanged(h.vault.head.VaultID)
	h.keeper.check(false)
	h.assertExports(2)
	h.vault.change()
	h.advance(time.Minute)
	h.keeper.check(false)
	h.assertExports(2)
}

func TestRetentionRemovesTheOldestJournaledBackupsOnly(t *testing.T) {
	h := newHarness(t)
	h.settings.Keep = 3
	foreign := []string{path.Join(folderA, "Personal export.rpv"), path.Join(folderA, backupName(h.clock.Add(-24*time.Hour)))}
	for _, address := range foreign {
		h.folders.files[address] = []byte("the owner's file")
	}
	var written []string
	for range 5 {
		h.keeper.Check()
		written = append(written, path.Join(folderA, backupName(h.clock)))
		h.vault.change()
		h.advance(24 * time.Hour)
	}
	h.assertFiles(append(slices.Clone(foreign), written[2:]...)...)

	h.settings.Keep = 1
	h.keeper.Check()
	h.assertFiles(append(slices.Clone(foreign), path.Join(folderA, backupName(h.clock)))...)
}

func TestAFileThatResistsRemovalIsTriedAfterTheNextBackup(t *testing.T) {
	h := newHarness(t)
	h.settings.Keep = 1
	h.keeper.Check()
	stuck := path.Join(folderA, backupName(h.clock))
	h.folders.removeErr[stuck] = fs.ErrPermission
	h.vault.change()
	h.advance(24 * time.Hour)
	h.keeper.Check()
	second := path.Join(folderA, backupName(h.clock))
	h.assertFiles(stuck, second)
	if status := h.keeper.Status(); status.Failed || !status.Last.Equal(h.clock) {
		t.Fatalf("a backup whose predecessor stayed reports %+v", status)
	}

	delete(h.folders.removeErr, stuck)
	h.vault.change()
	h.advance(24 * time.Hour)
	h.keeper.Check()
	h.assertFiles(path.Join(folderA, backupName(h.clock)))
}

func TestABackupRemovedByTheOwnerLeavesTheJournal(t *testing.T) {
	h := newHarness(t)
	h.settings.Keep = 1
	h.keeper.Check()
	delete(h.folders.files, path.Join(folderA, backupName(h.clock)))
	h.vault.change()
	h.advance(24 * time.Hour)
	h.keeper.Check()
	h.assertFiles(path.Join(folderA, backupName(h.clock)))
	if listed := h.keeper.journal.backups(h.vault.head.VaultID.String()); len(listed) != 1 {
		t.Fatalf("the journal lists %d backups, want 1", len(listed))
	}
}

func TestAnotherFolderStartsItsOwnBackupsAndLeavesTheOldOnes(t *testing.T) {
	h := newHarness(t)
	h.settings.Keep = 1
	h.keeper.Check()
	inA := path.Join(folderA, backupName(h.clock))

	h.advance(time.Hour)
	h.settings.Folder = preferences.BackupFolder{Address: folderB, Name: "Backups", Place: "owner"}
	if status := h.keeper.Status(); !status.Last.IsZero() {
		t.Fatalf("a new folder reports a backup at %v", status.Last)
	}
	h.keeper.Check()
	inB := path.Join(folderB, backupName(h.clock))
	h.assertFiles(inA, inB)

	h.advance(time.Hour)
	h.settings.Folder.Address = folderA
	h.keeper.Check()
	h.assertFiles(inA, inB, path.Join(folderA, backupName(h.clock)))
}

func TestAFailedBackupWaitsAnHourUnlessAsked(t *testing.T) {
	h := newHarness(t)
	h.folders.saveErr = errors.New("the drive is not connected")
	h.keeper.Check()
	if status := h.keeper.Status(); !status.Failed || !status.Last.IsZero() {
		t.Fatalf("status after a failed backup = %+v", status)
	}
	h.folders.saveErr = nil

	h.advance(59 * time.Minute)
	h.keeper.check(false)
	h.assertExports(1)
	h.advance(time.Minute)
	h.keeper.check(false)
	h.assertFiles(path.Join(folderA, backupName(h.clock)))
	if status := h.keeper.Status(); status.Failed {
		t.Fatal("a backup after a failure still reports the failure")
	}

	h.vault.exportErr = vaultservice.ErrStaleExport
	h.vault.change()
	h.advance(24 * time.Hour)
	h.keeper.check(false)
	if !h.keeper.Status().Failed {
		t.Fatal("a failed export is not reported")
	}
	h.vault.exportErr = nil
	h.keeper.Check()
	h.assertExports(4)
	if h.keeper.Status().Failed {
		t.Fatal("a check the owner asked for did not try again")
	}
}

func TestABackupOfAChangingVaultIsNotRecordedAsTheLatestCopy(t *testing.T) {
	h := newHarness(t)
	h.vault.afterExport = func(f *fakeVault) {
		f.head.Revision++
		f.head.Hash = [32]byte{0xee}
	}
	h.keeper.Check()
	h.assertExports(1)
	if len(h.folders.files) != 1 || len(h.vault.recorded) != 0 {
		t.Fatalf("backups %d, recorded %d; want one unrecorded backup", len(h.folders.files), len(h.vault.recorded))
	}
}

func TestTheJournalOutlivesTheKeeper(t *testing.T) {
	h := newHarness(t)
	h.settings.Keep = 1
	h.keeper.Check()
	h.keeper = h.newKeeper()
	if status := h.keeper.Status(); !status.Last.Equal(h.clock) {
		t.Fatalf("status after a restart = %+v", status)
	}
	h.advance(time.Hour)
	h.keeper.Check()
	h.assertExports(1)

	h.vault.change()
	h.advance(24 * time.Hour)
	h.keeper.Check()
	h.assertFiles(path.Join(folderA, backupName(h.clock)))
}

func TestAnUnreadableJournalRemovesNothing(t *testing.T) {
	h := newHarness(t)
	h.settings.Keep = 1
	h.keeper.Check()
	first := path.Join(folderA, backupName(h.clock))
	for _, content := range []string{"{", `{"version":2,"vaults":{}}`, `{"version":1}`} {
		if err := os.WriteFile(h.journal, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		h.keeper = h.newKeeper()
		h.advance(time.Hour)
		h.keeper.Check()
	}
	if got := len(h.folders.files); got != 4 {
		t.Fatalf("folders hold %d files, want every backup kept", got)
	}
	if _, kept := h.folders.files[first]; !kept {
		t.Fatal("a backup the journal no longer lists was removed")
	}
}

func TestPokeAsksRunForACheck(t *testing.T) {
	h := newHarness(t)
	h.folders.saved = make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		h.keeper.Run(ctx)
		close(stopped)
	}()
	h.keeper.Poke()
	h.keeper.Poke()
	select {
	case address := <-h.folders.saved:
		if address != path.Join(folderA, backupName(h.clock)) {
			t.Fatalf("a poke backed up to %q", address)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a poke did not back up the open vault")
	}
	cancel()
	<-stopped
}

func TestABackupIsNamedAfterItsVaultAndLocalTime(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	for name, test := range map[string]struct {
		target storage.Target
		want   string
	}{
		"path":           {storage.Target{Path: "/Users/owner/Personal.rpv"}, "Personal 2026-01-02 03-04-05.rpv"},
		"label":          {storage.Target{Path: "content://drive/document/7", Label: storage.Label{Name: "Family.rpv", Place: "Drive"}}, "Family 2026-01-02 03-04-05.rpv"},
		"separators":     {storage.Target{Label: storage.Label{Name: `Work: 2026\Q3` + "\t.rpv"}}, "Work- 2026-Q3- 2026-01-02 03-04-05.rpv"},
		"no extension":   {storage.Target{Path: "/Users/owner/vault"}, "vault 2026-01-02 03-04-05.rpv"},
		"extension only": {storage.Target{Path: "/Users/owner/.rpv"}, "2026-01-02 03-04-05.rpv"},
	} {
		if got := fileName(test.target, at); got != test.want {
			t.Errorf("%s: name = %q, want %q", name, got, test.want)
		}
	}
}

func TestTheDirectoryWritesNewFilesAndRemovesThem(t *testing.T) {
	folder := t.TempDir()
	data := []byte("encrypted copy")
	address, err := Directory{}.Save(folder, "Personal 2026-09-25 10-30-00.rpv", data)
	if err != nil {
		t.Fatal(err)
	}
	if address != filepath.Join(folder, "Personal 2026-09-25 10-30-00.rpv") {
		t.Fatalf("address = %q", address)
	}
	if saved, err := os.ReadFile(address); err != nil || !bytes.Equal(saved, data) {
		t.Fatalf("saved %q, %v", saved, err)
	}
	if _, err := (Directory{}).Save(folder, filepath.Base(address), []byte("other")); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("saving over a file = %v, want fs.ErrExist", err)
	}
	if err := (Directory{}).Remove(address); err != nil {
		t.Fatal(err)
	}
	if err := (Directory{}).Remove(address); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removing a file already gone = %v, want fs.ErrNotExist", err)
	}
	if _, err := (Directory{}).Save(filepath.Join(folder, "unplugged"), "Personal.rpv", data); err == nil {
		t.Fatal("a missing folder took a backup")
	}
}
