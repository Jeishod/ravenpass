package vaultservice

import (
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestCreationPhraseActionsRequireMatchingPendingVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	called := false
	action := func() error {
		called = true
		return nil
	}
	if err := service.VerifyStagedPhrase("not a recovery key"); !errors.Is(err, ErrNoPendingSetup) {
		t.Fatalf("verification without setup: %v", err)
	}
	if err := service.WithVerifiedStagedPhrase("not a recovery key", action); !errors.Is(err, ErrNoPendingSetup) || called {
		t.Fatalf("action without setup: %v, called = %t", err, called)
	}
	phrase, err := service.BeginCreation()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyStagedPhrase("not a recovery key"); !errors.Is(err, vault.ErrInvalidPhrase) {
		t.Fatalf("wrong phrase verification: %v", err)
	}
	if err := service.WithVerifiedStagedPhrase("not a recovery key", action); !errors.Is(err, vault.ErrInvalidPhrase) || called {
		t.Fatalf("action with wrong phrase: %v, called = %t", err, called)
	}
	if err := service.VerifyStagedPhrase(phrase); err != nil {
		t.Fatal(err)
	}
	if err := service.WithVerifiedStagedPhrase(phrase, action); err != nil || !called {
		t.Fatalf("verified action: %v, called = %t", err, called)
	}
	service.Lock()
	called = false
	if err := service.WithVerifiedStagedPhrase(phrase, action); !errors.Is(err, ErrNoPendingSetup) || called {
		t.Fatalf("action after setup ended: %v, called = %t", err, called)
	}
}

func TestPhraseActionsFollowTheStagedNewPhrase(t *testing.T) {
	service, _ := readyVault(t)
	called := false
	action := func() error {
		called = true
		return nil
	}
	phrase, err := service.BeginRekey("")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.WithVerifiedStagedPhrase(phrase, action); err != nil || !called {
		t.Fatalf("an action with the new phrase: %v, called = %t", err, called)
	}
	service.DiscardRekey()
	called = false
	if err := service.WithVerifiedStagedPhrase(phrase, action); !errors.Is(err, ErrNoPendingSetup) || called {
		t.Fatalf("an action after the new phrase was dropped: %v, called = %t", err, called)
	}
}
