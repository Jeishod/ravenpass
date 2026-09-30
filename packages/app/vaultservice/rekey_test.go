package vaultservice

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/vault"
)

func TestANewRecoveryPhraseKeepsThisDevicesWaysIn(t *testing.T) {
	devices := newSyncedDevices(t)
	mac := devices.mac
	if err := mac.SetPIN(openingOf(mac), testPIN); err != nil {
		t.Fatal(err)
	}
	saveItems(t, mac, "GitHub")
	before := bytes.Clone(devices.macFiles.data)
	if _, err := mac.BeginRekey(openingOf(mac), ""); !errors.Is(err, vault.ErrInvalidPIN) {
		t.Fatalf("a new phrase without the PIN: got %v, want ErrInvalidPIN", err)
	}
	if _, err := mac.BeginRekey(openingOf(mac), "999999"); !errors.Is(err, unlock.ErrWrongPIN) {
		t.Fatalf("a new phrase with a wrong PIN: got %v, want ErrWrongPIN", err)
	}
	if methods, err := mac.UnlockMethods(); err != nil || methods.PINAttemptsLeft != unlock.MaxPINFailures-1 {
		t.Fatalf("the wrong PIN was not counted: %+v, error = %v", methods, err)
	}
	phrase, err := mac.BeginRekey(openingOf(mac), testPIN)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mac.ConfirmRekey("not the words"); !errors.Is(err, vault.ErrInvalidPhrase) {
		t.Fatalf("confirming other words: got %v, want ErrInvalidPhrase", err)
	}
	if !bytes.Equal(devices.macFiles.data, before) {
		t.Fatal("the vault file changed before the new phrase was confirmed")
	}
	head, err := mac.ConfirmRekey(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if witnessOf(t, mac, head.VaultID) != vault.WitnessFor(head) {
		t.Fatal("the new version is not acknowledged")
	}
	if labels := labelsOf(t, mac); len(labels) != 1 {
		t.Fatalf("the vault under the new key shows %v", labels)
	}
	if methods, err := mac.UnlockMethods(); err != nil || !methods.PINSet || !methods.BiometryEnabled {
		t.Fatalf("the new phrase changed the ways in: %+v, error = %v", methods, err)
	}
	mac.Lock()
	if _, err := mac.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("the PIN no longer opens the vault: %v", err)
	}
	mac.Lock()
	if _, err := mac.Unlock(testReason); err != nil {
		t.Fatalf("device authentication no longer opens the vault: %v", err)
	}
	if opened, err := vault.OpenWithRecovery(devices.macFiles.data, phrase); err != nil {
		t.Fatalf("the new phrase does not open the file: %v", err)
	} else {
		opened.Lock()
	}
	if opened, err := vault.OpenWithRecovery(before, devices.phrase); err != nil {
		t.Fatalf("an older copy no longer opens with the old phrase: %v", err)
	} else {
		opened.Lock()
	}
}

func TestTheWrongPINThatRemovesThePINForANewPhraseLocksTheVault(t *testing.T) {
	service, _ := readyVault(t)
	if err := service.SetPIN(openingOf(service), testPIN); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	service.pinThrottle = unlock.NewThrottle(func() time.Time { return now })
	for attempt := 1; attempt < unlock.MaxPINFailures; attempt++ {
		if _, err := service.BeginRekey(openingOf(service), "999999"); !errors.Is(err, unlock.ErrWrongPIN) {
			t.Fatalf("attempt %d: got %v, want ErrWrongPIN", attempt, err)
		}
		now = now.Add(unlock.AttemptDelay(attempt))
	}
	if _, err := service.BeginRekey(openingOf(service), "999999"); !errors.Is(err, unlock.ErrPINRemoved) {
		t.Fatalf("the last attempt: got %v, want ErrPINRemoved", err)
	}
	if service.Unlocked() {
		t.Fatal("the vault stayed open after its PIN was removed")
	}
	if _, err := service.BeginRekey(openingOf(service), ""); !errors.Is(err, ErrNotReady) {
		t.Fatalf("a new phrase after the PIN was removed: got %v, want ErrNotReady", err)
	}
}

func TestAnotherDeviceNeedsTheNewRecoveryPhrase(t *testing.T) {
	devices := newSyncedDevices(t)
	if err := devices.phone.SetPIN(openingOf(devices.phone), testPIN); err != nil {
		t.Fatal(err)
	}
	phrase, err := devices.mac.BeginRekey(openingOf(devices.mac), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := devices.mac.ConfirmRekey(phrase); err != nil {
		t.Fatal(err)
	}
	deliver(devices.macFiles, devices.phoneFiles)
	if change := devices.phone.Follow(); change != FileRefused {
		t.Fatalf("an open vault following a file under a new key: %v", change)
	}
	if _, err := devices.phone.Unlock(testReason); !errors.Is(err, vault.ErrKeyReplaced) {
		t.Fatalf("device authentication: got %v, want ErrKeyReplaced", err)
	}
	if _, err := devices.phone.UnlockWithPIN(testPIN); !errors.Is(err, vault.ErrKeyReplaced) {
		t.Fatalf("the PIN: got %v, want ErrKeyReplaced", err)
	}
	if methods, err := devices.phone.UnlockMethods(); err != nil || methods.PINAttemptsLeft != unlock.MaxPINFailures {
		t.Fatalf("a PIN for the replaced key was counted: %+v, error = %v", methods, err)
	}
	if _, err := devices.phone.BeginRecovery(devices.phrase); !errors.Is(err, vault.ErrAuthentication) {
		t.Fatalf("the old phrase: got %v, want ErrAuthentication", err)
	}
	preview, err := devices.phone.BeginRecovery(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if preview.MayLoseNewerCredentials || preview.NeedsWayIn {
		t.Fatalf("the new phrase on a device that followed the vault: %+v", preview)
	}
	if _, err := devices.phone.ConfirmRecovery(false, MethodChoice{}); err != nil {
		t.Fatal(err)
	}
	if methods, err := devices.phone.UnlockMethods(); err != nil || methods.PINSet || !methods.BiometryEnabled {
		t.Fatalf("after recovery the phone offers %+v, error = %v; want device authentication alone", methods, err)
	}
	devices.phone.Lock()
	if _, err := devices.phone.Unlock(testReason); err != nil {
		t.Fatalf("device authentication after recovery: %v", err)
	}
}

func TestAPINOnlyDeviceChoosesAWayInAfterANewPhrase(t *testing.T) {
	files := &memoryFiles{}
	other := &memoryFiles{}
	mac := newTestService(t, files, newMemoryKeys())
	oldPhrase, _ := createTestVault(t, mac)
	deliver(files, other)
	phone := newTestService(t, other, newMemoryKeys())
	if _, err := phone.BeginRecovery(oldPhrase); err != nil {
		t.Fatal(err)
	}
	if _, err := phone.ConfirmRecovery(true, MethodChoice{PIN: testPIN}); err != nil {
		t.Fatal(err)
	}
	phone.Lock()
	phrase, err := mac.BeginRekey(openingOf(mac), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mac.ConfirmRekey(phrase); err != nil {
		t.Fatal(err)
	}
	deliver(files, other)
	preview, err := phone.BeginRecovery(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.NeedsWayIn {
		t.Fatal("a PIN for the replaced key was offered as a way in")
	}
	if _, err := phone.ConfirmRecovery(false, MethodChoice{}); !errors.Is(err, unlock.ErrNoMethodLeft) {
		t.Fatalf("keeping a PIN for the replaced key: got %v, want ErrNoMethodLeft", err)
	}
	if _, err := phone.ConfirmRecovery(false, MethodChoice{PIN: testPIN}); err != nil {
		t.Fatal(err)
	}
	phone.Lock()
	if _, err := phone.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("the PIN chosen after recovery: %v", err)
	}
}

func TestANewPhraseInterruptedAfterTheFileIsReplacedKeepsTheWaysIn(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVaultWith(t, service, MethodChoice{PIN: testPIN})
	phrase, err := service.BeginRekey(openingOf(service), testPIN)
	if err != nil {
		t.Fatal(err)
	}
	files.failAfterReplace = true
	_, err = service.ConfirmRekey(phrase)
	if !errors.Is(err, ErrKeyChangeUnfinished) || !errors.Is(err, storage.ErrDurabilityUncertain) {
		t.Fatalf("an interrupted save: got %v, want ErrKeyChangeUnfinished", err)
	}
	if service.Unlocked() {
		t.Fatal("the vault stayed open after an interrupted save")
	}
	if _, err := service.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("the PIN after the file was replaced: %v", err)
	}
}

func TestANewPhraseWhoseRecordFailsAfterTheWriteStillMovesTheWaysIn(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVaultWith(t, service, MethodChoice{PIN: testPIN})
	phrase, err := service.BeginRekey(openingOf(service), testPIN)
	if err != nil {
		t.Fatal(err)
	}
	keys.policySaveFailures = []error{errors.New("the records file is busy")}
	if _, err := service.ConfirmRekey(phrase); !errors.Is(err, ErrKeyChangeUnfinished) {
		t.Fatalf("a record that failed after the write: got %v, want ErrKeyChangeUnfinished", err)
	}
	if _, err := service.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("the PIN after the ways in were moved again: %v", err)
	}
}

func TestANewPhraseWhoseFileCannotBeReadBackIsUncertain(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	_, head := createTestVaultWith(t, service, MethodChoice{PIN: testPIN})
	policy := bytes.Clone(keys.policy[head.VaultID.String()])
	phrase, err := service.BeginRekey(openingOf(service), testPIN)
	if err != nil {
		t.Fatal(err)
	}
	files.failAfterReplace = true
	files.unreadable = errors.New("the folder is out of reach")
	if _, err := service.ConfirmRekey(phrase); !errors.Is(err, ErrKeyChangeUncertain) {
		t.Fatalf("a save that cannot be read back: got %v, want ErrKeyChangeUncertain", err)
	}
	if !bytes.Equal(keys.policy[head.VaultID.String()], policy) {
		t.Fatal("an uncertain key change moved the ways in")
	}
}

func TestANewPhraseWhoseFileIsNotWrittenKeepsTheOldKeyAndThePhrase(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVaultWith(t, service, MethodChoice{PIN: testPIN})
	head, err := service.session.Head()
	if err != nil {
		t.Fatal(err)
	}
	policy := bytes.Clone(keys.policy[head.VaultID.String()])
	history := bytes.Clone(keys.keys[head.VaultID.String()])
	phrase, err := service.BeginRekey(openingOf(service), testPIN)
	if err != nil {
		t.Fatal(err)
	}
	files.maxBytes = 1
	if _, err := service.ConfirmRekey(phrase); !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("a refused save: got %v, want ErrTooLarge", err)
	}
	if len(history) == 0 || !bytes.Equal(keys.policy[head.VaultID.String()], policy) || !bytes.Equal(keys.keys[head.VaultID.String()], history) {
		t.Fatal("the ways in or the key record moved to a key that was never written")
	}
	files.maxBytes = 0
	if _, err := service.ConfirmRekey(phrase); err != nil {
		t.Fatalf("the staged phrase after a refused save: %v", err)
	}
	service.Lock()
	if _, err := service.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("the PIN after the key change: %v", err)
	}
}

func TestANewPhraseForAFileThatChangedLeavesTheVaultAsItWas(t *testing.T) {
	devices := newSyncedDevices(t)
	if err := devices.mac.SetPIN(openingOf(devices.mac), testPIN); err != nil {
		t.Fatal(err)
	}
	phrase, err := devices.mac.BeginRekey(openingOf(devices.mac), testPIN)
	if err != nil {
		t.Fatal(err)
	}
	saveItems(t, devices.phone, "saved on the phone")
	deliver(devices.phoneFiles, devices.macFiles)
	if _, err := devices.mac.ConfirmRekey(phrase); !errors.Is(err, storage.ErrStaleHead) {
		t.Fatalf("a new phrase over another device's save: got %v, want ErrStaleHead", err)
	}
	if _, err := vault.OpenWithRecovery(devices.macFiles.data, phrase); !errors.Is(err, vault.ErrAuthentication) {
		t.Fatalf("the refused phrase opens the file: %v", err)
	}
	if _, err := devices.mac.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("the PIN after a refused new phrase: %v", err)
	}
}

func TestAnUnreadableEnvelopeIsAWayInTheDeviceNoLongerHas(t *testing.T) {
	keys := newMemoryKeys()
	device := newTestDevice()
	service := newTestServiceOn(t, &memoryFiles{}, keys, device)
	createTestVaultWith(t, service, MethodChoice{Biometry: true, PIN: testPIN})
	name := service.device.policyVault
	service.Lock()
	policy, err := unlock.Decode(keys.policy[name])
	if err != nil {
		t.Fatal(err)
	}
	policy.PIN.Envelope = []byte{0x83, 0x01, 0x02, 0x03}
	policy.Platform.Envelope = []byte{0x83, 0x01, 0x02, 0x03}
	stored, err := policy.Encode()
	if err != nil {
		t.Fatal(err)
	}
	keys.policy[name] = stored
	if _, err := service.UnlockWithPIN(testPIN); !errors.Is(err, unlock.ErrNoPIN) {
		t.Fatalf("a PIN whose envelope does not read: got %v, want ErrNoPIN", err)
	}
	if _, err := service.Unlock(testReason); !errors.Is(err, unlock.ErrUnbound) {
		t.Fatalf("device authentication whose envelope does not read: got %v, want ErrUnbound", err)
	}
	if len(device.platform.Prompts()) != 0 {
		t.Fatal("an envelope that cannot open the vault asked the owner")
	}
	if methods, err := service.UnlockMethods(); err != nil || methods.PINSet || methods.BiometryEnabled {
		t.Fatalf("unreadable envelopes are still offered: %+v, error = %v", methods, err)
	}
}

func TestUnlockChangesWaitWhileANewPhraseIsStaged(t *testing.T) {
	service, _ := readyVault(t)
	if _, err := service.BeginRekey(openingOf(service), testPIN); !errors.Is(err, unlock.ErrNoPIN) {
		t.Fatalf("a PIN for a vault without one: got %v, want ErrNoPIN", err)
	}
	if _, err := service.BeginRekey(openingOf(service), ""); err != nil {
		t.Fatal(err)
	}
	if err := service.SetPIN(openingOf(service), testPIN); !errors.Is(err, ErrSetupInProgress) {
		t.Fatalf("setting a PIN during a new phrase: got %v, want ErrSetupInProgress", err)
	}
	if err := service.SetBiometryUnlock(openingOf(service), false); !errors.Is(err, ErrSetupInProgress) {
		t.Fatalf("changing device authentication during a new phrase: got %v, want ErrSetupInProgress", err)
	}
	service.DiscardRekey()
	if err := service.SetPIN(openingOf(service), testPIN); err != nil {
		t.Fatal(err)
	}
	phrase, err := service.BeginRekey(openingOf(service), testPIN)
	if err != nil {
		t.Fatal(err)
	}
	reopenTestVault(t, service)
	if _, err := service.ConfirmRekey(phrase); !errors.Is(err, ErrNoPendingSetup) {
		t.Fatalf("a new phrase after the vault locked: got %v, want ErrNoPendingSetup", err)
	}
}
