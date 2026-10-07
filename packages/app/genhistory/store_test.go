package genhistory

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

type sessionVault struct {
	mu      sync.Mutex
	session *vault.Session
	id      vault.ID
	locked  bool
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
	defer v.mu.Unlock()
	if v.locked || id != v.id {
		return nil, vault.ErrLocked
	}
	return v.session.OpenDeviceData(name, sealed)
}

func newStore(t *testing.T, v Vault) (*Store, string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "generator")
	store, err := New(directory, v, vault.SeedWordlist)
	if err != nil {
		t.Fatal(err)
	}
	return store, directory
}

func TestANewVaultStartsWithTheDefaultOptionsAndNoHistory(t *testing.T) {
	store, _ := newStore(t, newSessionVault(t))
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.Options != DefaultOptions() || len(state.History) != 0 {
		t.Fatalf("state = %+v", state)
	}
}

func TestEveryGeneratedValueIsKeptNewestFirstWithItsOptions(t *testing.T) {
	store, _ := newStore(t, newSessionVault(t))
	first, err := store.Generate(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	phrase := DefaultOptions()
	phrase.Kind = KindPassphrase
	second, err := store.Generate(phrase)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.History) != 2 || state.History[0] != second || state.History[1] != first {
		t.Fatalf("history = %+v", state.History)
	}
	if state.Options != phrase {
		t.Fatalf("options = %+v, want the last used", state.Options)
	}
}

func TestTheHistoryFileHoldsNoPlaintext(t *testing.T) {
	v := newSessionVault(t)
	store, directory := newStore(t, v)
	entry, err := store.Generate(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(directory, v.id.String()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(contents, []byte(entry.Value)) || bytes.Contains(contents, []byte("history")) {
		t.Fatal("the history file holds plaintext")
	}
}

func TestClearForgetsTheHistoryAndKeepsTheOptions(t *testing.T) {
	store, _ := newStore(t, newSessionVault(t))
	options := DefaultOptions()
	options.Length = 30
	if _, err := store.Generate(options); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.History) != 0 || state.Options != options {
		t.Fatalf("state = %+v", state)
	}
}

func TestALockedVaultKeepsItsHistoryClosed(t *testing.T) {
	v := newSessionVault(t)
	store, _ := newStore(t, v)
	if _, err := store.Generate(DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	v.mu.Lock()
	v.locked = true
	v.mu.Unlock()
	if _, err := store.State(); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("State err = %v, want ErrLocked", err)
	}
	if _, err := store.Generate(DefaultOptions()); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("Generate err = %v, want ErrLocked", err)
	}
}

func TestAHistoryNoKeyOpensIsSetAsideAndTheGeneratorStartsAgain(t *testing.T) {
	first := newSessionVault(t)
	store, directory := newStore(t, first)
	if _, err := store.Generate(DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	second := newSessionVault(t)
	if err := os.Rename(filepath.Join(directory, first.id.String()), filepath.Join(directory, second.id.String())); err != nil {
		t.Fatal(err)
	}
	other, err := New(directory, second, vault.SeedWordlist)
	if err != nil {
		t.Fatal(err)
	}
	state, err := other.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.History) != 0 {
		t.Fatalf("another vault read the history: %+v", state.History)
	}
	aside, err := filepath.Glob(filepath.Join(directory, second.id.String()+".unreadable-*"))
	if err != nil || len(aside) != 1 {
		t.Fatalf("set aside %v, %v", aside, err)
	}
	if _, err := other.Generate(DefaultOptions()); err != nil {
		t.Fatalf("the generator did not start again: %v", err)
	}
}

func TestAKeyChangeSealsTheHistoryAgain(t *testing.T) {
	v := newSessionVault(t)
	store, _ := newStore(t, v)
	entry, err := store.Generate(DefaultOptions())
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
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.History) != 1 || state.History[0] != entry {
		t.Fatalf("history after the key change = %+v", state.History)
	}
}

func TestAVaultWithoutHistoryHasNothingToSealAgain(t *testing.T) {
	v := newSessionVault(t)
	store, _ := newStore(t, v)
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

func TestInvalidOptionsRecordNothing(t *testing.T) {
	store, _ := newStore(t, newSessionVault(t))
	options := DefaultOptions()
	options.Length = MaxLength + 1
	if _, err := store.Generate(options); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("err = %v, want ErrInvalidOptions", err)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.History) != 0 {
		t.Fatalf("history = %+v", state.History)
	}
}

func TestTheHistoryDropsTheOldestPastItsLimit(t *testing.T) {
	v := newSessionVault(t)
	store, _ := newStore(t, v)
	id, data, err := store.read()
	if err != nil {
		t.Fatal(err)
	}
	for range MaxEntries {
		data.History = append(data.History, Entry{Value: "old", Kind: KindPassword})
	}
	data.History[0].Value = "oldest"
	if err := store.write(id, data); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Generate(DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.History) != MaxEntries || state.History[len(state.History)-1].Value == "oldest" {
		t.Fatalf("kept %d entries, oldest %q", len(state.History), state.History[len(state.History)-1].Value)
	}
	if strings.Contains(state.History[0].Value, "old") {
		t.Fatal("the newest entry is not first")
	}
}
