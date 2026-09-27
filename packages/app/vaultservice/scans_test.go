package vaultservice

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

var testPDF = []byte("%PDF-1.4\n1 0 obj\n<< >>\nendobj\n%%EOF\n")

func identityWithScan(t *testing.T) vault.IdentityInput {
	t.Helper()
	scan, err := vault.PrepareScan("passport.pdf", testPDF)
	if err != nil {
		t.Fatal(err)
	}
	input := testIdentity()
	input.Documents = append([]vault.Document(nil), input.Documents...)
	input.Documents[0].Attach = []vault.PreparedScan{scan}
	return input
}

func TestScansRoundTripAcrossLockAndGoWithTheirIdentity(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	owner, err := service.CreateIdentity(identityWithScan(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(files.data, []byte("endobj")) {
		t.Fatal("the scan appeared in the vault file")
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	scans, err := service.ScansOf(owner)
	if err != nil || len(scans) != 1 {
		t.Fatalf("scans = %+v, error = %v", scans, err)
	}
	read, err := service.ReadScan(scans[0].ID)
	if err != nil || !bytes.Equal(read.Content, testPDF) || read.Name != "passport.pdf" {
		t.Fatalf("scan after a lock = %+v, error = %v", read.ScanSummary, err)
	}
	if err := service.DeleteItem(scans[0].ID); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("deleting a scan as an item = %v", err)
	}
	if err := service.DeleteItem(owner); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadScan(scans[0].ID); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("a deleted identity's scan = %v", err)
	}
}

func TestOversizedSaveIsRefusedWithoutLocking(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	owner, err := service.CreateIdentity(testIdentity(), nil)
	if err != nil {
		t.Fatal(err)
	}
	saved := bytes.Clone(files.data)
	files.maxBytes = len(saved) + len(testPDF)/2
	if err := service.EditIdentity(owner, identityWithScan(t), nil); !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("an oversized save = %v", err)
	}
	if !bytes.Equal(files.data, saved) {
		t.Fatal("the refused save reached the vault file")
	}
	files.maxBytes = 0
	if err := service.EditIdentity(owner, identityWithScan(t), nil); err != nil {
		t.Fatalf("the vault was left unusable after a refused save: %v", err)
	}
}

func TestScanMethodsNeedAnOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	cases := map[string]func() error{
		"ScansOf":  func() error { _, err := service.ScansOf(vault.ID{1}); return err },
		"ReadScan": func() error { _, err := service.ReadScan(vault.ID{1}); return err },
	}
	for name, call := range cases {
		if err := call(); !errors.Is(err, ErrNotReady) {
			t.Fatalf("%s before a vault is open = %v", name, err)
		}
	}
}
