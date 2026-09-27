package vaultservice

import (
	"crypto/sha256"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestDeviceRecordsAreReadOncePerSession(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVault(t, service)
	id, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, exported, err := service.Export()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordExport(exported, sha256.Sum256(data)); err != nil {
		t.Fatal(err)
	}

	keys.usageLoads = 0
	keys.exportLoads = 0
	for range 5 {
		if _, err := service.Usage(); err != nil {
			t.Fatal(err)
		}
		if _, err := service.ExportState(); err != nil {
			t.Fatal(err)
		}
		if err := service.MarkUsed(id); err != nil {
			t.Fatal(err)
		}
	}
	if keys.usageLoads != 1 || keys.exportLoads != 0 {
		t.Fatalf("record reads: usage = %d, export = %d, want one usage read and none for the export the session already holds", keys.usageLoads, keys.exportLoads)
	}

	reopenTestVault(t, service)
	if _, err := service.Usage(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExportState(); err != nil {
		t.Fatal(err)
	}
	if keys.usageLoads != 2 || keys.exportLoads != 1 {
		t.Fatalf("record reads after unlock: usage = %d, export = %d, want one more of each", keys.usageLoads, keys.exportLoads)
	}
}
