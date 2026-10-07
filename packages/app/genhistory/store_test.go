package genhistory

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

type sessionVault struct {
	mu      sync.Mutex
	session *vault.Session
	id      vault.ID
	locked  bool
	// beforeOpen runs before each opening, as a key change sealing the file again would.
	beforeOpen func()
}

func newSessionVault(t *testing.T) *sessionVault {
	t.Helper()
	created, err := vault.Create()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(created.Session.Lock)
	head, err := created.Session.Head()
	if err != nil {
		t.Fatal(err)
	}
	return &sessionVault{session: created.Session, id: head.VaultID}
}

func (v *sessionVault) OpenVaultID() (vault.ID, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.locked {
		return vault.ID{}, vault.ErrLocked
	}
	return v.id, nil
}

func (v *sessionVault) SealDeviceData(id vault.ID, name string, plaintext []byte) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.locked || id != v.id {
		return nil, vault.ErrLocked
	}
	return v.session.SealDeviceData(name, plaintext)
}

func (v *sessionVault) OpenDeviceData(id vault.ID, name string, sealed []byte) ([]byte, error) {
	v.mu.Lock()
	before := v.beforeOpen
	v.beforeOpen = nil
	v.mu.Unlock()
	if before != nil {
		before()
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.locked || id != v.id {
		return nil, vault.ErrLocked
	}
	return v.session.OpenDeviceData(name, sealed)
}

type clock struct{ now time.Time }

func (c *clock) read() time.Time { return c.now }

func newStore(t *testing.T, v Vault, days int) (*Store, *clock, string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "generator")
	store, err := New(directory, v, func() int { return days })
	if err != nil {
		t.Fatal(err)
	}
	at := &clock{now: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)}
	store.now = at.read
	return store, at, directory
}

func values(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.Value
	}
	return out
}

func TestANewVaultHasNoHistory(t *testing.T) {
	store, _, _ := newStore(t, newSessionVault(t), 30)
	history, err := store.History()
	if err != nil || len(history) != 0 {
		t.Fatalf("history = %+v, %v", history, err)
	}
}

func TestRecordedPasswordsAreReadNewestFirst(t *testing.T) {
	store, at, _ := newStore(t, newSessionVault(t), 30)
	if _, err := store.Record("first", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	at.now = at.now.Add(time.Minute)
	second, err := store.Record("Orbit-Velvet7-Canyon", ModeWords)
	if err != nil {
		t.Fatal(err)
	}
	history, err := store.History()
	if err != nil {
		t.Fatal(err)
	}
	if got := values(history); len(got) != 2 || got[0] != "Orbit-Velvet7-Canyon" || got[1] != "first" {
		t.Fatalf("history = %v", got)
	}
	if history[0] != second || !second.At.Equal(at.now) {
		t.Fatalf("newest = %+v, want %+v at %v", history[0], second, at.now)
	}
}

func TestAnInvalidPasswordOrModeIsNotRecorded(t *testing.T) {
	store, _, _ := newStore(t, newSessionVault(t), 30)
	for name, entry := range map[string]Entry{
		"empty":          {Value: "", Mode: ModeWords},
		"unknown mode":   {Value: "secret", Mode: "pin"},
		"too long":       {Value: strings.Repeat("a", MaxValueBytes+1), Mode: ModeCharacters},
		"invalid UTF-8":  {Value: "\xff", Mode: ModeCharacters},
		"no mode at all": {Value: "secret"},
	} {
		if _, err := store.Record(entry.Value, entry.Mode); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("%s: err = %v, want ErrInvalidEntry", name, err)
		}
	}
	history, err := store.History()
	if err != nil || len(history) != 0 {
		t.Fatalf("history = %+v, %v", history, err)
	}
}

func TestTheHistoryFileHoldsNoPlaintext(t *testing.T) {
	v := newSessionVault(t)
	store, _, directory := newStore(t, v, 30)
	if _, err := store.Record("hunter2-generated", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(directory, v.id.String()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(contents, []byte("hunter2-generated")) || bytes.Contains(contents, []byte("history")) {
		t.Fatal("the history file holds plaintext")
	}
}

func TestClearRemovesTheHistoryFile(t *testing.T) {
	v := newSessionVault(t)
	store, _, directory := newStore(t, v, 30)
	if _, err := store.Record("secret", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, v.id.String())); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the history file is still there: %v", err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("clearing an empty history: %v", err)
	}
}

func TestALockedVaultKeepsItsHistoryClosed(t *testing.T) {
	v := newSessionVault(t)
	store, _, _ := newStore(t, v, 30)
	if _, err := store.Record("secret", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	v.mu.Lock()
	v.locked = true
	v.mu.Unlock()
	if _, err := store.History(); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("History err = %v, want ErrLocked", err)
	}
	if _, err := store.Record("secret", ModeCharacters); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("Record err = %v, want ErrLocked", err)
	}
}

func TestPasswordsKeptPastThePeriodAreRemoved(t *testing.T) {
	v := newSessionVault(t)
	store, at, _ := newStore(t, v, 7)
	if _, err := store.Record("old", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	at.now = at.now.Add(5 * day)
	if _, err := store.Record("recent", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	if count, err := store.Past(3); err != nil || count != 1 {
		t.Fatalf("Past(3) = %d, %v, want 1", count, err)
	}
	at.now = at.now.Add(2 * day)
	history, err := store.History()
	if err != nil {
		t.Fatal(err)
	}
	if got := values(history); len(got) != 1 || got[0] != "recent" {
		t.Fatalf("history after seven days = %v", got)
	}
	plaintext, err := store.open(v.id)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plaintext, []byte(`"old"`)) {
		t.Fatal("the removed password is still in the file")
	}
}

func TestAHistoryNoKeyOpensIsSetAsideAndTheHistoryStartsAgain(t *testing.T) {
	first := newSessionVault(t)
	store, _, directory := newStore(t, first, 30)
	if _, err := store.Record("secret", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	second := newSessionVault(t)
	if err := os.Rename(filepath.Join(directory, first.id.String()), filepath.Join(directory, second.id.String())); err != nil {
		t.Fatal(err)
	}
	other, err := New(directory, second, func() int { return 30 })
	if err != nil {
		t.Fatal(err)
	}
	history, err := other.History()
	if err != nil || len(history) != 0 {
		t.Fatalf("another vault read the history: %+v, %v", history, err)
	}
	aside, err := filepath.Glob(filepath.Join(directory, second.id.String()+".unreadable-*"))
	if err != nil || len(aside) != 1 {
		t.Fatalf("set aside %v, %v", aside, err)
	}
	if _, err := other.Record("next", ModeCharacters); err != nil {
		t.Fatalf("the history did not start again: %v", err)
	}
}

func TestAFileSealedAgainWhileItIsReadStillOpens(t *testing.T) {
	v := newSessionVault(t)
	store, _, directory := newStore(t, v, 30)
	if _, err := store.Record("secret", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, v.id.String())
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The read finds bytes no key opens; the file then holds a fresh seal, as after a key change.
	if err := os.WriteFile(path, append([]byte(nil), current[:len(current)-1]...), 0o600); err != nil {
		t.Fatal(err)
	}
	v.beforeOpen = func() {
		if err := os.WriteFile(path, current, 0o600); err != nil {
			t.Error(err)
		}
	}
	history, err := store.History()
	if err != nil {
		t.Fatal(err)
	}
	if got := values(history); len(got) != 1 || got[0] != "secret" {
		t.Fatalf("history = %v", got)
	}
	if aside, _ := filepath.Glob(path + ".unreadable-*"); len(aside) != 0 {
		t.Fatalf("a readable history was set aside: %v", aside)
	}
}

func TestAKeyChangeSealsTheHistoryAgain(t *testing.T) {
	v := newSessionVault(t)
	store, _, _ := newStore(t, v, 30)
	entry, err := store.Record("secret", ModeCharacters)
	if err != nil {
		t.Fatal(err)
	}
	rekey, err := v.session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	write, err := store.ResealDeviceData(v.id, v.session.OpenDeviceData, rekey.SealDeviceData)
	if err != nil || write == nil {
		t.Fatalf("reseal: %v", err)
	}
	pending, err := v.session.PrepareRekey(rekey)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if err := write(); err != nil {
		t.Fatal(err)
	}
	history, err := store.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0] != entry {
		t.Fatalf("history after the key change = %+v", history)
	}
}

func TestAVaultWithoutHistoryHasNothingToSealAgain(t *testing.T) {
	v := newSessionVault(t)
	store, _, _ := newStore(t, v, 30)
	rekey, err := v.session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	defer rekey.Discard()
	write, err := store.ResealDeviceData(v.id, v.session.OpenDeviceData, rekey.SealDeviceData)
	if err != nil || write != nil {
		t.Fatalf("write %v, err %v", write != nil, err)
	}
}

func TestTheHistoryDropsTheOldestPastItsLimit(t *testing.T) {
	v := newSessionVault(t)
	store, at, _ := newStore(t, v, 30)
	full := make([]Entry, MaxEntries)
	for i := range full {
		full[i] = Entry{Value: "old", Mode: ModeCharacters, At: at.now}
	}
	full[0].Value = "oldest"
	if err := store.write(v.id, full); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Record("newest", ModeCharacters); err != nil {
		t.Fatal(err)
	}
	history, err := store.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != MaxEntries || history[0].Value != "newest" || history[len(history)-1].Value == "oldest" {
		t.Fatalf("kept %d entries, newest %q, oldest %q", len(history), history[0].Value, history[len(history)-1].Value)
	}
}
