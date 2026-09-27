package vaultservice

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func testCard() vault.CardInput {
	return vault.CardInput{
		Label:        "Everyday",
		Holder:       "Alex Example",
		Number:       "4111111111111111",
		Expiry:       "2029-08",
		SecurityCode: "7391",
		PIN:          "482913",
		Network:      vault.NetworkVisa,
		BankName:     "Jyske Bank",
		BankSite:     "jyskebank.dk",
		Color:        "#00a0e1",
		Billing:      &vault.Address{Street: "Vestergade 8-16", City: "Silkeborg"},
		Notes:        "kept in the wallet",
	}
}

func readTestCard(t *testing.T, service *Service, id vault.ID) vault.Card {
	t.Helper()
	selection, err := service.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	card, err := service.ReadSelectedCard(selection)
	if err != nil {
		t.Fatal(err)
	}
	return card
}

func TestCardWritesRoundTripAcrossLock(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	input := testCard()
	id, err := service.CreateCard(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{input.Number, input.Holder, input.PIN, input.Billing.Street, input.Notes} {
		if bytes.Contains(files.data, []byte(value)) {
			t.Fatalf("%q appeared in the vault file", value)
		}
	}
	if card := readTestCard(t, service, id); !reflect.DeepEqual(card.CardInput, input) {
		t.Fatalf("card = %+v", card.CardInput)
	}
	owner, err := service.CreateIdentity(vault.IdentityInput{Label: "Alex", Addresses: []vault.Address{{City: "Aarhus"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := service.IdentityAddresses()
	if err != nil || len(addresses) != 1 || addresses[0].Identity != owner {
		t.Fatalf("identity addresses = %+v, error = %v", addresses, err)
	}
	replacement := vault.CardInput{Label: "Travel", Number: "2200123412341234", Network: vault.NetworkMir,
		BillingLink: &vault.AddressLink{Identity: owner, Address: addresses[0].Addresses[0].ID}}
	if err := service.EditCard(id, replacement, nil); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	if _, err := service.Unlock(testReason); err != nil {
		t.Fatal(err)
	}
	card := readTestCard(t, service, id)
	if !reflect.DeepEqual(card.CardInput, replacement) || card.Linked == nil || card.Linked.City != "Aarhus" {
		t.Fatalf("card after a lock = %+v, linked = %+v", card.CardInput, card.Linked)
	}
	if err := service.DeleteItem(owner); err != nil {
		t.Fatal(err)
	}
	if card := readTestCard(t, service, id); card.BillingLink != nil || card.Linked != nil {
		t.Fatalf("card after its identity was deleted = %+v", card.CardInput)
	}
}

func TestCardWritesRefuseAnotherKind(t *testing.T) {
	files := &memoryFiles{}
	service := newTestService(t, files, newMemoryKeys())
	createTestVault(t, service)
	identity, err := service.CreateIdentity(testIdentity(), nil)
	if err != nil {
		t.Fatal(err)
	}
	saved := bytes.Clone(files.data)
	if err := service.EditCard(identity, testCard(), nil); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("card edit of an identity = %v", err)
	}
	selection, err := service.Select(identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadSelectedCard(selection); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("card read of an identity = %v", err)
	}
	if !bytes.Equal(files.data, saved) {
		t.Fatal("a refused card write reached the vault file")
	}
}

func TestCardMethodsNeedAnOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	cases := []struct {
		name string
		call func() error
	}{
		{name: "ReadSelectedCard", call: func() error { _, err := service.ReadSelectedCard(vault.Selection{}); return err }},
		{name: "CreateCard", call: func() error { _, err := service.CreateCard(testCard(), nil); return err }},
		{name: "EditCard", call: func() error { return service.EditCard(vault.ID{1}, testCard(), nil) }},
		{name: "IdentityAddresses", call: func() error { _, err := service.IdentityAddresses(); return err }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); !errors.Is(err, ErrNotReady) {
				t.Fatalf("error before a vault is open = %v, want ErrNotReady", err)
			}
		})
	}
}
