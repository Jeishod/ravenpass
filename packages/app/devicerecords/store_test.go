package devicerecords

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testVaultID = "0123456789abcdef0123456789abcdef"

// recordingBackups remembers every path it was asked to exclude and answers with err.
type recordingBackups struct {
	err      error
	excluded []string
}

func (r *recordingBackups) Exclude(path string) error {
	r.excluded = append(r.excluded, path)
	return r.err
}

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	store, path, _ := newTestStoreWithBackups(t, &recordingBackups{})
	return store, path
}

func newTestStoreWithBackups(t *testing.T, backups *recordingBackups) (*Store, string, *recordingBackups) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "records", "device.json")
	store, err := New(path, backups)
	if err != nil {
		t.Fatal(err)
	}
	return store, path, backups
}

func reopen(t *testing.T, path string) *Store {
	t.Helper()
	store, err := New(path, &recordingBackups{})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestEveryWriteMarksTheRecordsBeforeTheyReachTheirPath(t *testing.T) {
	store, path, backups := newTestStoreWithBackups(t, &recordingBackups{})
	if err := store.SaveHeadWitness(testVaultID, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveHeadWitness(testVaultID, []byte("second")); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteHeadWitness(testVaultID); err != nil {
		t.Fatal(err)
	}
	if len(backups.excluded) != 3 {
		t.Fatalf("three writes marked %d files", len(backups.excluded))
	}
	for _, marked := range backups.excluded {
		if marked == path || filepath.Dir(marked) != filepath.Dir(path) {
			t.Fatalf("marked %q, want a file beside %q that is renamed onto it", marked, path)
		}
	}
}

func TestAFailedMarkDoesNotFailTheWrite(t *testing.T) {
	store, path, _ := newTestStoreWithBackups(t, &recordingBackups{err: errors.New("no backup mark")})
	if err := store.SaveHeadWitness(testVaultID, []byte("witness")); err != nil {
		t.Fatalf("a failed mark failed the write: %v", err)
	}
	if witness, err := reopen(t, path).LoadHeadWitness(testVaultID); err != nil || string(witness) != "witness" {
		t.Fatalf("the record came back as %q, error = %v", witness, err)
	}
}

func TestNewRequiresABackupExclusion(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "device.json"), nil); err == nil {
		t.Fatal("a store without a backup exclusion was accepted")
	}
}

func TestEveryRecordSurvivesAReopen(t *testing.T) {
	store, path := newTestStore(t)
	written := map[string][]byte{
		"witness": []byte("revision and hash"),
		"usage":   []byte("last used"),
		"export":  []byte("export note"),
		"policy":  []byte(`{"version":2}`),
	}
	if err := store.SaveHeadWitness(testVaultID, written["witness"]); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUsageRecord(testVaultID, written["usage"]); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveExportRecord(testVaultID, written["export"]); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUnlockPolicy(testVaultID, written["policy"]); err != nil {
		t.Fatal(err)
	}
	reopened := reopen(t, path)
	for name, read := range map[string]func(string) ([]byte, error){
		"witness": reopened.LoadHeadWitness,
		"usage":   reopened.LoadUsageRecord,
		"export":  reopened.LoadExportRecord,
		"policy":  reopened.LoadUnlockPolicy,
	} {
		value, err := read(testVaultID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(value, written[name]) {
			t.Fatalf("%s came back as %q", name, value)
		}
	}
}

func TestAMissingRecordIsReportedAsMissing(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.LoadHeadWitness(testVaultID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a vault with no witness: got %v, want ErrNotFound", err)
	}
	if err := store.DeleteHeadWitness(testVaultID); err != nil {
		t.Fatalf("deleting a record that is not there: %v", err)
	}
}

func TestRecordsAreRefusedForAnInvalidVaultAndKeptPrivate(t *testing.T) {
	store, path := newTestStore(t)
	for _, invalid := range []string{"not a vault", strings.ToUpper(testVaultID), testVaultID[:30]} {
		if err := store.SaveHeadWitness(invalid, []byte("x")); !errors.Is(err, ErrInvalidIdentifier) {
			t.Fatalf("vault identifier %q: got %v", invalid, err)
		}
	}
	if err := store.SaveHeadWitness(testVaultID, nil); !errors.Is(err, ErrInvalidData) {
		t.Fatalf("an empty record: got %v", err)
	}
	if err := store.SaveHeadWitness(testVaultID, []byte("x")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("the records file is readable as %v", info.Mode().Perm())
	}
}

func TestADeletedRecordLeavesTheOthers(t *testing.T) {
	store, path := newTestStore(t)
	if err := store.SaveHeadWitness(testVaultID, []byte("witness")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUsageRecord(testVaultID, []byte("usage")); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteUsageRecord(testVaultID); err != nil {
		t.Fatal(err)
	}
	reopened := reopen(t, path)
	if _, err := reopened.LoadUsageRecord(testVaultID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the deleted record: got %v, want ErrNotFound", err)
	}
	witness, err := reopened.LoadHeadWitness(testVaultID)
	if err != nil || string(witness) != "witness" {
		t.Fatalf("the remaining record came back as %q, error = %v", witness, err)
	}
}
