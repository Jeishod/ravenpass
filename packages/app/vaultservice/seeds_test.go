package vaultservice

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

const testPhrase = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

func testPhraseSeed() vault.SeedInput {
	return vault.SeedInput{
		Label:      "Cold wallet",
		Format:     vault.SeedPhrase,
		Words:      strings.Fields(testPhrase),
		Passphrase: "extra words",
		Path:       "m/84'/0'/0'",
		Wallet:     "Ledger",
		Addresses:  []vault.SeedAddress{{Label: "Savings", Value: "bc1qexample"}},
		Notes:      "paper copy in the safe",
	}
}

func testCodesSeed() vault.SeedInput {
	return vault.SeedInput{
		Label:  "Mail codes",
		Format: vault.SeedBackupCodes,
		Codes:  []vault.BackupCode{{Value: "1111-2222"}, {Value: "3333-4444"}},
	}
}

func readTestSeed(t *testing.T, service *Service, id vault.ID) vault.Seed {
	t.Helper()
	selection, err := service.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := service.ReadSelectedSeed(selection)
	if err != nil {
		t.Fatal(err)
	}
	return seed
}

func TestSeedWritesRoundTripAcrossLock(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	input := testPhraseSeed()
	id, err := service.CreateSeed(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"abandon", input.Passphrase, input.Notes} {
		if bytes.Contains(files.data, []byte(value)) {
			t.Fatalf("%q appeared in the vault file", value)
		}
	}
	seed := readTestSeed(t, service, id)
	if !reflect.DeepEqual(seed.SeedInput, input) || seed.Checksum != vault.SeedChecksumValid || seed.CheckedOn != "" {
		t.Fatalf("seed = %+v", seed)
	}
	if err := service.RecordSeedCheck(id, "2026-09-21"); err != nil {
		t.Fatal(err)
	}
	replacement := input
	replacement.Wallet = "Trezor"
	if err := service.EditSeed(id, replacement, nil); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	if seed := readTestSeed(t, service, id); !reflect.DeepEqual(seed.SeedInput, replacement) || seed.CheckedOn != "2026-09-21" {
		t.Fatalf("seed after a lock = %+v", seed)
	}
}

func TestSpendBackupCodeMarksOneCode(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	codes, err := service.CreateSeed(testCodesSeed(), nil)
	if err != nil {
		t.Fatal(err)
	}
	phrase, err := service.CreateSeed(testPhraseSeed(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SpendBackupCode(codes, 1); err != nil {
		t.Fatal(err)
	}
	if seed := readTestSeed(t, service, codes); seed.Codes[0].Used || !seed.Codes[1].Used {
		t.Fatalf("codes after a use = %+v", seed.Codes)
	}
	saved := bytes.Clone(files.data)
	for name, call := range map[string]func() error{
		"used code":          func() error { return service.SpendBackupCode(codes, 1) },
		"index out of range": func() error { return service.SpendBackupCode(codes, 2) },
		"phrase seed":        func() error { return service.SpendBackupCode(phrase, 0) },
		"check of codes":     func() error { return service.RecordSeedCheck(codes, "2026-09-21") },
		"malformed day":      func() error { return service.RecordSeedCheck(phrase, "21.09.2026") },
	} {
		if err := call(); !errors.Is(err, vault.ErrInvalidInput) {
			t.Fatalf("%s = %v, want ErrInvalidInput", name, err)
		}
	}
	if !bytes.Equal(files.data, saved) {
		t.Fatal("a refused seed write reached the vault file")
	}
}

func TestSeedMethodsNeedAnOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	cases := []struct {
		name string
		call func() error
	}{
		{name: "ReadSelectedSeed", call: func() error { _, err := service.ReadSelectedSeed(vault.Selection{}); return err }},
		{name: "CreateSeed", call: func() error { _, err := service.CreateSeed(testPhraseSeed(), nil); return err }},
		{name: "EditSeed", call: func() error { return service.EditSeed(vault.ID{1}, testPhraseSeed(), nil) }},
		{name: "SpendBackupCode", call: func() error { return service.SpendBackupCode(vault.ID{1}, 0) }},
		{name: "RecordSeedCheck", call: func() error { return service.RecordSeedCheck(vault.ID{1}, "2026-09-21") }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); !errors.Is(err, ErrNotReady) {
				t.Fatalf("error before a vault is open = %v, want ErrNotReady", err)
			}
		})
	}
}
