package vaultservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// stateAfter returns the state count once it passes seen, failing after five seconds.
func stateAfter(t *testing.T, service *Service, seen uint64) uint64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, err := service.AwaitVaultState(ctx, seen)
	if err != nil {
		t.Fatalf("the state count did not pass %d: %v", seen, err)
	}
	return count
}

// stateQuiet reports whether the state count stays at seen for a short while.
func stateQuiet(t *testing.T, service *Service, seen uint64) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := service.AwaitVaultState(ctx, seen)
	return errors.Is(err, context.DeadlineExceeded)
}

func TestTheVaultStateMovesForEverySaveOpeningLockAndSwitch(t *testing.T) {
	personal, work := target("/vaults/personal.rpv"), target("/vaults/work.rpv")
	service := newTestService(t, &memoryFiles{target: personal}, newMemoryKeys())
	createTestVault(t, service)
	seen := stateAfter(t, service, 0)
	if !stateQuiet(t, service, seen) {
		t.Fatal("the state moved without a change")
	}
	steps := []struct {
		name string
		step func() error
	}{
		{"a save from the workspace", func() error {
			_, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
			return err
		}},
		{"a lock", func() error { service.Lock(); return nil }},
		{"an unlock", func() error { _, err := service.Unlock(testReason); return err }},
		{"a switch", func() error { return service.SwitchVault(work) }},
		{"binding a location", func() error { return service.BindStorage(personal) }},
	}
	for _, s := range steps {
		if err := s.step(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		next := stateAfter(t, service, seen)
		if next <= seen {
			t.Fatalf("%s: state %d after %d", s.name, next, seen)
		}
		seen = next
	}
	if _, err := service.State(); err != nil {
		t.Fatal(err)
	}
	if !stateQuiet(t, service, seen) {
		t.Fatal("the state moved for a read")
	}
}

func TestCurrentVaultIsTheOpenOneOrTheOneTheLocationHolds(t *testing.T) {
	personal, work := target("/vaults/personal.rpv"), target("/vaults/work.rpv")
	service := newTestService(t, &memoryFiles{target: personal}, newMemoryKeys())
	_, head := createTestVault(t, service)
	if id, ok := service.CurrentVault(); !ok || id != head.VaultID {
		t.Fatalf("current vault while open = %v, %v; want %v", id, ok, head.VaultID)
	}
	service.Lock()
	if id, ok := service.CurrentVault(); !ok || id != head.VaultID {
		t.Fatalf("current vault while locked = %v, %v; want %v", id, ok, head.VaultID)
	}
	if err := service.SwitchVault(work); err != nil {
		t.Fatal(err)
	}
	if id, ok := service.CurrentVault(); ok {
		t.Fatalf("current vault at an empty location = %v", id)
	}
	_, other := createTestVault(t, service)
	service.Lock()
	if id, ok := service.CurrentVault(); !ok || id != other.VaultID || id == head.VaultID {
		t.Fatalf("current vault after switching = %v, %v; want %v", id, ok, other.VaultID)
	}
	if _, err := service.DeleteVault(work); err != nil {
		t.Fatal(err)
	}
	if id, ok := service.CurrentVault(); !ok || id != head.VaultID {
		t.Fatalf("current vault after deleting the other = %v, %v; want %v", id, ok, head.VaultID)
	}
}
