package vaultservice

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func TestUsageKeepsOnlyTheMostRecentCredentials(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)
	ids := make([]vault.ID, 0, maxUsageEntries+2)
	for i := range maxUsageEntries + 2 {
		id, err := service.CreateCredential(vault.CredentialInput{Label: fmt.Sprintf("entry %d", i), Password: "secret"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	usage, err := service.Usage()
	if err != nil || len(usage) != 0 {
		t.Fatalf("usage before any use = %v, error = %v", usage, err)
	}
	for _, id := range ids {
		if err := service.MarkUsed(id); err != nil {
			t.Fatal(err)
		}
	}
	usage, err = service.Usage()
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != maxUsageEntries {
		t.Fatalf("recorded credentials = %d, want %d", len(usage), maxUsageEntries)
	}
	for _, id := range ids[:2] {
		if _, recorded := usage[id]; recorded {
			t.Fatal("the least recently used credentials were not evicted")
		}
	}
	for _, id := range ids[2:] {
		if usage[id] <= 0 {
			t.Fatalf("credential %s has no usage timestamp", id)
		}
	}
	if err := service.MarkUsed(ids[0]); err != nil {
		t.Fatal(err)
	}
	usage, err = service.Usage()
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != maxUsageEntries || usage[ids[0]] <= 0 {
		t.Fatalf("re-used credential was not recorded in %d entries", len(usage))
	}
	if _, recorded := usage[ids[2]]; recorded {
		t.Fatal("re-use did not evict the oldest recorded credential")
	}
	stored := keys.usage[head.VaultID.String()]
	if len(stored) != 1+maxUsageEntries*usageRecordSize {
		t.Fatalf("stored usage size = %d bytes", len(stored))
	}
	if bytes.Contains(stored, []byte("entry")) {
		t.Fatal("credential labels reached the device usage record")
	}
	if err := service.MarkUsed(vault.ID{}); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("unknown credential usage error = %v", err)
	}
	service.Lock()
	if _, err := service.Usage(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("locked usage error = %v", err)
	}
}

func TestUsageRejectsMalformedDeviceRecord(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)
	id, err := service.CreateCredential(vault.CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.MarkUsed(id); err != nil {
		t.Fatal(err)
	}
	name := head.VaultID.String()
	valid := bytes.Clone(keys.usage[name])
	if len(valid) != 1+usageRecordSize {
		t.Fatalf("stored usage size = %d bytes", len(valid))
	}
	unknownVersion := bytes.Clone(valid)
	unknownVersion[0] = usageVersion + 1
	duplicated := append(bytes.Clone(valid), valid[1:]...)
	zeroTimestamp := bytes.Clone(valid)
	clear(zeroTimestamp[17:25])
	oversized := make([]byte, 1+(maxUsageEntries+1)*usageRecordSize)
	oversized[0] = usageVersion
	corrupt := map[string][]byte{
		"empty":           nil,
		"unknown version": unknownVersion,
		"truncated":       valid[:len(valid)-1],
		"duplicate id":    duplicated,
		"zero timestamp":  zeroTimestamp,
		"over the limit":  oversized,
	}
	for description, data := range corrupt {
		if _, err := decodeUsage(data); !errors.Is(err, ErrUsageCorrupt) {
			t.Fatalf("%s decode error = %v", description, err)
		}
		keys.usage[name] = data
		reopenTestVault(t, service)
		usage, err := service.Usage()
		if err != nil || len(usage) != 0 {
			t.Fatalf("%s usage = %v, error = %v", description, usage, err)
		}
		if err := service.MarkUsed(id); err != nil {
			t.Fatalf("%s mark used error = %v", description, err)
		}
		replaced, err := service.Usage()
		if err != nil || replaced[id] <= 0 {
			t.Fatalf("%s replaced usage = %v, error = %v", description, replaced, err)
		}
	}
	keys.usage[name] = valid
	reopenTestVault(t, service)
	usage, err := service.Usage()
	if err != nil || usage[id] <= 0 {
		t.Fatalf("valid usage record = %v, error = %v", usage, err)
	}
}
