package vaultservice

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestExportStateMovesFromUnknownToCurrentThenStale(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVault(t, service)
	id, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.ExportState()
	if err != nil || state != ExportUnknown {
		t.Fatalf("state without a record = %v, error = %v", state, err)
	}
	data, exported, err := service.Export()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordExport(exported, sha256.Sum256(data)); err != nil {
		t.Fatal(err)
	}
	state, err = service.ExportState()
	if err != nil || state != ExportCurrent {
		t.Fatalf("state after export = %v, error = %v", state, err)
	}
	label := "Mail and calendar"
	if err := service.EditCredential(id, vault.CredentialPatch{Label: &label}); err != nil {
		t.Fatal(err)
	}
	state, err = service.ExportState()
	if err != nil || state != ExportStale {
		t.Fatalf("state after an edit = %v, error = %v", state, err)
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	state, err = service.ExportState()
	if err != nil || state != ExportStale {
		t.Fatalf("state after reopening = %v, error = %v", state, err)
	}
	if err := service.RecordExport(exported, sha256.Sum256(data)); !errors.Is(err, ErrStaleExport) {
		t.Fatalf("record of a superseded head = %v", err)
	}
	service.Lock()
	if _, err := service.ExportState(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("locked export state error = %v", err)
	}
}

func TestExportStateReportsMalformedRecordAsUnknown(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)
	data, exported, err := service.Export()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordExport(exported, sha256.Sum256(data)); err != nil {
		t.Fatal(err)
	}
	name := head.VaultID.String()
	valid := bytes.Clone(keys.export[name])
	if len(valid) != exportRecordSize {
		t.Fatalf("stored export record size = %d bytes", len(valid))
	}
	unknownVersion := bytes.Clone(valid)
	unknownVersion[0] = exportRecordVersion + 1
	otherVault := bytes.Clone(valid)
	otherVault[1] ^= 1
	zeroRevision := bytes.Clone(valid)
	clear(zeroRevision[17:25])
	corrupt := map[string][]byte{
		"empty":            nil,
		"unknown version":  unknownVersion,
		"truncated":        valid[:len(valid)-1],
		"oversized":        append(bytes.Clone(valid), 0),
		"another vault":    otherVault,
		"missing revision": zeroRevision,
	}
	for description, record := range corrupt {
		keys.export[name] = record
		reopenTestVault(t, service)
		state, err := service.ExportState()
		if err != nil || state != ExportUnknown {
			t.Fatalf("%s state = %v, error = %v", description, state, err)
		}
	}
	mismatchedDigest := bytes.Clone(valid)
	mismatchedDigest[len(mismatchedDigest)-1] ^= 1
	keys.export[name] = mismatchedDigest
	reopenTestVault(t, service)
	state, err := service.ExportState()
	if err != nil || state != ExportStale {
		t.Fatalf("state for a record of other file bytes = %v, error = %v", state, err)
	}
	keys.export[name] = valid
	reopenTestVault(t, service)
	state, err = service.ExportState()
	if err != nil || state != ExportCurrent {
		t.Fatalf("state for the stored record = %v, error = %v", state, err)
	}
}
