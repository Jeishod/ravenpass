package vaultservice

import (
	"bytes"
	"errors"
	"testing"
)

// keyOf is the key the open vault of service is sealed under.
func keyOf(t *testing.T, service *Service) [32]byte {
	t.Helper()
	_, identity, err := sessionKey(service.session)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func TestRecoveryOntoAKeyThisDeviceReplacedNeedsConfirming(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	oldPhrase, head := createTestVault(t, service)
	oldKey := keyOf(t, service)
	oldFile := bytes.Clone(files.data)
	oldWitness := bytes.Clone(keys.witness[head.VaultID.String()])
	phrase, err := service.BeginRekey(openingOf(service), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmRekey(phrase); err != nil {
		t.Fatal(err)
	}
	newKey := keyOf(t, service)
	service.Lock()

	// Someone holding the old recovery key writes a file under it that the witness accepts.
	files.data = oldFile
	keys.witness[head.VaultID.String()] = oldWitness
	preview, err := service.BeginRecovery(oldPhrase)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.KeyReplaced || preview.MayLoseNewerCredentials {
		t.Fatalf("a file under the replaced key previews as %+v", preview)
	}
	if _, err := service.ConfirmRecovery(false, MethodChoice{}); !errors.Is(err, ErrReplacedKeyNeedsConfirmation) {
		t.Fatalf("an unconfirmed recovery onto the replaced key: got %v, want ErrReplacedKeyNeedsConfirmation", err)
	}
	if _, err := service.ConfirmRecovery(true, MethodChoice{Biometry: true}); err != nil {
		t.Fatalf("a confirmed recovery onto the replaced key: %v", err)
	}
	history, err := service.loadKeyHistory(head.VaultID)
	if err != nil {
		t.Fatal(err)
	}
	if [32]byte(history.Current) != oldKey || !history.replaced(newKey) || history.replaced(oldKey) {
		t.Fatal("the key record did not move to the key the owner chose")
	}
}

func TestAKeyChangedOnAnotherDeviceRecoversWithoutWarningAndIsThenRemembered(t *testing.T) {
	devices := newSyncedDevices(t)
	oldFile := bytes.Clone(devices.macFiles.data)
	phrase, err := devices.mac.BeginRekey(openingOf(devices.mac), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := devices.mac.ConfirmRekey(phrase); err != nil {
		t.Fatal(err)
	}
	deliver(devices.macFiles, devices.phoneFiles)
	devices.phone.Lock()
	preview, err := devices.phone.BeginRecovery(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if preview.KeyReplaced {
		t.Fatal("a key changed on another device warned as a replaced one")
	}
	if _, err := devices.phone.ConfirmRecovery(false, MethodChoice{}); err != nil {
		t.Fatal(err)
	}
	devices.phone.Lock()
	devices.phoneFiles.data = oldFile
	preview, err = devices.phone.BeginRecovery(devices.phrase)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.KeyReplaced {
		t.Fatal("the phone forgot the key it recovered away from")
	}
}

func TestAVaultOpenedWithoutAKeyRecordStartsOne(t *testing.T) {
	service, keys := readyVault(t)
	head, err := service.session.Head()
	if err != nil {
		t.Fatal(err)
	}
	identity := keyOf(t, service)
	delete(keys.keys, head.VaultID.String())
	reopenTestVault(t, service)
	history, err := service.loadKeyHistory(head.VaultID)
	if err != nil || [32]byte(history.Current) != identity || len(history.Replaced) != 0 {
		t.Fatalf("an opened vault recorded %+v, error = %v", history, err)
	}
}

func TestAKeyChangeWithoutARecordStillMarksTheOldKeyReplaced(t *testing.T) {
	service, keys := readyVault(t)
	head, err := service.session.Head()
	if err != nil {
		t.Fatal(err)
	}
	old := keyOf(t, service)
	delete(keys.keys, head.VaultID.String())
	phrase, err := service.BeginRekey(openingOf(service), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmRekey(phrase); err != nil {
		t.Fatal(err)
	}
	history, err := service.loadKeyHistory(head.VaultID)
	if err != nil || !history.replaced(old) || [32]byte(history.Current) != keyOf(t, service) {
		t.Fatalf("after a key change without a record: %+v, error = %v", history, err)
	}
}

func TestADeviceFromAnEarlierReleaseLearnsTheKeyItsWaysInHeld(t *testing.T) {
	devices := newSyncedDevices(t)
	old := keyOf(t, devices.phone)
	head, err := devices.phone.session.Head()
	if err != nil {
		t.Fatal(err)
	}
	phoneKeys := devices.phone.keys.(*memoryKeys)
	delete(phoneKeys.keys, head.VaultID.String())
	phrase, err := devices.mac.BeginRekey(openingOf(devices.mac), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := devices.mac.ConfirmRekey(phrase); err != nil {
		t.Fatal(err)
	}
	deliver(devices.macFiles, devices.phoneFiles)
	devices.phone.Lock()
	if _, err := devices.phone.BeginRecovery(phrase); err != nil {
		t.Fatal(err)
	}
	if _, err := devices.phone.ConfirmRecovery(false, MethodChoice{}); err != nil {
		t.Fatal(err)
	}
	history, err := devices.phone.loadKeyHistory(head.VaultID)
	if err != nil || !history.replaced(old) {
		t.Fatalf("the phone's record after recovery: %+v, error = %v", history, err)
	}
}

func TestAMalformedKeyRecordReadsAsNone(t *testing.T) {
	service, keys := readyVault(t)
	head, err := service.session.Head()
	if err != nil {
		t.Fatal(err)
	}
	keys.keys[head.VaultID.String()] = []byte("not a record")
	history, err := service.loadKeyHistory(head.VaultID)
	if err != nil || len(history.Current) != 0 {
		t.Fatalf("a malformed record read as %+v, error = %v", history, err)
	}
}

func TestTheKeyHistoryKeepsTheNewestReplacedKeys(t *testing.T) {
	var history keyHistory
	for i := range maxReplacedKeys + 3 {
		history = history.movedTo([32]byte{byte(i + 1)})
	}
	if len(history.Replaced) != maxReplacedKeys {
		t.Fatalf("the history holds %d replaced keys, want %d", len(history.Replaced), maxReplacedKeys)
	}
	if !history.replaced([32]byte{maxReplacedKeys + 2}) || history.replaced([32]byte{1}) {
		t.Fatal("the history dropped a newer key before an older one")
	}
	back := history.movedTo([32]byte{maxReplacedKeys + 2})
	if back.replaced([32]byte{maxReplacedKeys + 2}) || !back.replaced([32]byte{maxReplacedKeys + 3}) {
		t.Fatal("returning to a replaced key kept it replaced")
	}
}

func TestDeletingAVaultRemovesItsKeyRecord(t *testing.T) {
	files := &memoryFiles{target: target("/vaults/personal.rpv")}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVault(t, service)
	if _, err := service.DeleteVault(files.target); err != nil {
		t.Fatal(err)
	}
	if _, kept := keys.keys[head.VaultID.String()]; kept {
		t.Fatal("deleting the vault kept its key record")
	}
}
