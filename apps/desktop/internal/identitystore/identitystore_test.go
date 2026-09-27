package identitystore

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/changecount"
	"github.com/dortanes/ravenpass/packages/vault"
)

var (
	personal = vault.ID{1}
	work     = vault.ID{2}
	mail     = autofill.CredentialIdentity{Kind: autofill.IdentityPassword, Site: "mail.example", User: "alex", Record: "a"}
	bank     = autofill.CredentialIdentity{Kind: autofill.IdentityPassword, Site: "bank.example", User: "sam", Record: "b"}
)

type fakeVault struct {
	mu         sync.Mutex
	current    vault.ID
	found      bool
	open       bool
	identities []autofill.CredentialIdentity
	unreadable bool
	states     changecount.Counter
}

func (v *fakeVault) AwaitVaultState(ctx context.Context, seen uint64) (uint64, error) {
	return v.states.Await(ctx, seen)
}

func (v *fakeVault) CurrentVault() (vault.ID, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.current, v.found
}

func (v *fakeVault) CredentialIdentities() ([]autofill.CredentialIdentity, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	switch {
	case v.unreadable:
		return nil, errors.New("the index could not be read")
	case !v.open:
		return nil, autofill.ErrLocked
	default:
		return slices.Clone(v.identities), nil
	}
}

// set changes the vault under the list and records a change of its state.
func (v *fakeVault) set(change func(*fakeVault)) {
	v.mu.Lock()
	change(v)
	v.mu.Unlock()
	v.states.Record()
}

// opened is the vault id open with identities.
func opened(id vault.ID, identities ...autofill.CredentialIdentity) func(*fakeVault) {
	return func(v *fakeVault) {
		v.current, v.found, v.open, v.identities = id, true, true, identities
	}
}

func locked(v *fakeVault) { v.open = false }

// fakeSystem records the lists written to it; each write is nil for a removal.
type fakeSystem struct {
	mu      sync.Mutex
	enabled bool
	refuse  bool
	writes  [][]autofill.CredentialIdentity
	written chan struct{}
}

func newSystem() *fakeSystem {
	return &fakeSystem{enabled: true, written: make(chan struct{}, 16)}
}

func (s *fakeSystem) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

func (s *fakeSystem) Replace(identities []autofill.CredentialIdentity) error {
	return s.write(append([]autofill.CredentialIdentity{}, identities...))
}

func (s *fakeSystem) RemoveAll() error {
	return s.write(nil)
}

func (s *fakeSystem) write(identities []autofill.CredentialIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refuse {
		return errors.New("the store is busy")
	}
	s.writes = append(s.writes, identities)
	s.written <- struct{}{}
	return nil
}

// taken returns the writes since the last call.
func (s *fakeSystem) taken() [][]autofill.CredentialIdentity {
	s.mu.Lock()
	defer s.mu.Unlock()
	writes := s.writes
	s.writes = nil
	return writes
}

func newList(t *testing.T, v *fakeVault, on *bool, system *fakeSystem) *List {
	t.Helper()
	list, err := New(v, v, func() bool { return *on }, system)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// expect fails unless the writes since the last call are want; a nil entry is a removal.
func expect(t *testing.T, system *fakeSystem, step string, want ...[]autofill.CredentialIdentity) {
	t.Helper()
	if got := system.taken(); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: writes = %+v, want %+v", step, got, want)
	}
}

func TestNewRequiresEverything(t *testing.T) {
	v, on := &fakeVault{}, true
	system := newSystem()
	for name, build := range map[string]func() (*List, error){
		"vault":      func() (*List, error) { return New(nil, v, func() bool { return on }, system) },
		"identities": func() (*List, error) { return New(v, nil, func() bool { return on }, system) },
		"choice":     func() (*List, error) { return New(v, v, nil, system) },
		"system":     func() (*List, error) { return New(v, v, func() bool { return on }, nil) },
	} {
		if _, err := build(); err == nil {
			t.Fatalf("a list without its %s was composed", name)
		}
	}
}

func TestTheListIsWrittenOnceForEachChangeAndKeptAcrossALock(t *testing.T) {
	v, on, system := &fakeVault{}, true, newSystem()
	list := newList(t, v, &on, system)
	opened(personal, mail)(v)
	list.sync()
	expect(t, system, "opening", []autofill.CredentialIdentity{mail})
	list.sync()
	expect(t, system, "an unchanged vault")
	opened(personal, mail, bank)(v)
	list.sync()
	expect(t, system, "a save", []autofill.CredentialIdentity{mail, bank})
	locked(v)
	list.sync()
	expect(t, system, "a lock")
	opened(personal, mail, bank)(v)
	list.sync()
	expect(t, system, "unlocking the same vault")
}

func TestTheListIsClearedWhenAnotherVaultBecomesCurrentOrNoneIs(t *testing.T) {
	v, on, system := &fakeVault{}, true, newSystem()
	list := newList(t, v, &on, system)
	opened(personal, mail)(v)
	list.sync()
	expect(t, system, "opening", []autofill.CredentialIdentity{mail})
	v.current, v.open = work, false
	list.sync()
	expect(t, system, "a switch", nil)
	list.sync()
	expect(t, system, "the switched vault still locked")
	opened(work, bank)(v)
	list.sync()
	expect(t, system, "opening the other vault", []autofill.CredentialIdentity{bank})
	v.found, v.open = false, false
	list.sync()
	expect(t, system, "removing the current vault", nil)
	opened(work)(v)
	list.sync()
	expect(t, system, "an empty vault", []autofill.CredentialIdentity{})
	opened(personal)(v)
	list.sync()
	expect(t, system, "another empty vault", []autofill.CredentialIdentity{})
}

func TestTheOwnersChoiceClearsAndRestoresTheList(t *testing.T) {
	v, on, system := &fakeVault{}, false, newSystem()
	list := newList(t, v, &on, system)
	opened(personal, mail)(v)
	list.sync()
	expect(t, system, "a first sync with the list off", nil)
	list.sync()
	expect(t, system, "the list still off")
	on = true
	list.sync()
	expect(t, system, "turning the list on", []autofill.CredentialIdentity{mail})
	on = false
	list.sync()
	expect(t, system, "turning the list off", nil)
	locked(v)
	on = true
	list.sync()
	expect(t, system, "turning the list on while locked")
}

func TestNothingIsWrittenWhileRavenpassIsNotTheProvider(t *testing.T) {
	v, on, system := &fakeVault{}, true, newSystem()
	list := newList(t, v, &on, system)
	opened(personal, mail)(v)
	list.sync()
	expect(t, system, "opening", []autofill.CredentialIdentity{mail})
	system.enabled = false
	list.sync()
	on = false
	list.sync()
	expect(t, system, "a provider turned off")
	on = true
	system.enabled = true
	list.sync()
	expect(t, system, "the provider turned on again", []autofill.CredentialIdentity{mail})
}

func TestAListFromAnEarlierRunIsKeptForTheCurrentVaultUntilItChanges(t *testing.T) {
	v, on, system := &fakeVault{current: personal, found: true}, true, newSystem()
	list := newList(t, v, &on, system)
	list.sync()
	expect(t, system, "a locked vault at launch")
	opened(personal, mail)(v)
	list.sync()
	expect(t, system, "the first unlock", []autofill.CredentialIdentity{mail})

	launched := newList(t, v, &on, system)
	v.current, v.open = work, false
	launched.sync()
	expect(t, system, "another vault current at launch")
	v.current = personal
	launched.sync()
	expect(t, system, "a switch after launch", nil)

	v.found = false
	newList(t, v, &on, system).sync()
	expect(t, system, "no vault at launch", nil)
}

func TestAFailedWriteIsTriedAgainAndAnUnreadableIndexWritesNothing(t *testing.T) {
	v, on, system := &fakeVault{}, true, newSystem()
	list := newList(t, v, &on, system)
	opened(personal, mail)(v)
	system.refuse = true
	list.sync()
	expect(t, system, "a refused write")
	system.refuse = false
	list.sync()
	expect(t, system, "the next sync", []autofill.CredentialIdentity{mail})
	v.unreadable = true
	list.sync()
	expect(t, system, "an unreadable index")
}

func TestRunFollowsTheVaultAndTheOwnersChoice(t *testing.T) {
	v, system := &fakeVault{}, newSystem()
	var mu sync.Mutex
	on := false
	list, err := New(v, v, func() bool { mu.Lock(); defer mu.Unlock(); return on }, system)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		list.Run(ctx)
	}()
	await := func(step string) {
		t.Helper()
		select {
		case <-system.written:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: nothing was written", step)
		}
	}
	await("starting with the list off")
	v.set(opened(personal, mail))
	mu.Lock()
	on = true
	mu.Unlock()
	list.Changed()
	await("turning the list on")
	v.set(opened(personal, mail, bank))
	await("a save")
	cancel()
	<-stopped
	expect(t, system, "running", nil, []autofill.CredentialIdentity{mail}, []autofill.CredentialIdentity{mail, bank})
}
