package vaultservice

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestAddItemsWritesOneSaveThatSurvivesALock(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	credential := vault.CredentialInput{Label: "Mail", Login: "alex", Password: "imported-password"}
	note := vault.NoteInput{Label: "Router", Body: "imported-note-body"}
	result, err := service.AddItems([]vault.NewItem{
		{Credential: &credential, Pinned: true, Groups: []string{"Imported"}},
		{Note: &note, Groups: []string{"imported"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.Groups != 1 {
		t.Fatalf("result = %+v", result)
	}
	for _, value := range []string{credential.Password, note.Body, "Imported"} {
		if bytes.Contains(files.data, []byte(value)) {
			t.Fatalf("%q appeared in the vault file", value)
		}
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	entries, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	listed := 0
	for _, entry := range entries {
		if entry.ID == result.Items[0] && entry.Pinned && len(entry.Groups) == 1 || entry.ID == result.Items[1] && len(entry.Groups) == 1 {
			listed++
		}
	}
	if listed != 2 {
		t.Fatalf("after a lock the vault lists %+v", entries)
	}
}

func TestAddItemsRefusedLeavesTheFileUnchanged(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	before := bytes.Clone(files.data)
	valid := vault.NoteInput{Label: "Fine"}
	long := vault.NoteInput{Label: strings.Repeat("x", vault.MaxLabelLength+1)}
	if _, err := service.AddItems([]vault.NewItem{{Note: &valid}, {Note: &long}}); !errors.Is(err, vault.ErrInvalidInput) {
		t.Fatalf("AddItems = %v", err)
	}
	if !bytes.Equal(files.data, before) {
		t.Fatal("a refused batch changed the vault file")
	}
	if _, err := service.AddItems([]vault.NewItem{{Note: &valid}}); err != nil {
		t.Fatalf("the service did not recover from a refused batch: %v", err)
	}
}

func TestAddItemsNeedsAnOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	note := vault.NoteInput{Label: "Note"}
	if _, err := service.AddItems([]vault.NewItem{{Note: &note}}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("AddItems without a vault = %v", err)
	}
}
