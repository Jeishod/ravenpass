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
	if err := mac.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	saveItems(t, mac, "GitHub")
	before := bytes.Clone(devices.macFiles.data)
	if _, err := mac.BeginRekey(""); !errors.Is(err, vault.ErrInvalidPIN) {
		t.Fatalf("a new phrase without the PIN: got %v, want ErrInvalidPIN", err)
	}
	if _, err := mac.BeginRekey("999999"); !errors.Is(err, unlock.ErrWrongPIN) {
		t.Fatalf("a new phrase with a wrong PIN: got %v, want ErrWrongPIN", err)
	}
	if methods, err := mac.UnlockMethods(); err != nil || methods.PINAttemptsLeft != unlock.MaxPINFailures-1 {
		t.Fatalf("the wrong PIN was not counted: %+v, error = %v", methods, err)
	}
	phrase, err := mac.BeginRekey(testPIN)
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
	if err := service.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	service.pinThrottle = unlock.NewThrottle(func() time.Time { return now })
	for attempt := 1; attempt < unlock.MaxPINFailures; attempt++ {
		if _, err := service.BeginRekey("999999"); !errors.Is(err, unlock.ErrWrongPIN) {
			t.Fatalf("attempt %d: got %v, want ErrWrongPIN", attempt, err)
		}
		now = now.Add(unlock.AttemptDelay(attempt))
	}
	if _, err := service.BeginRekey("999999"); !errors.Is(err, unlock.ErrPINRemoved) {
		t.Fatalf("the last attempt: got %v, want ErrPINRemoved", err)
	}
	if service.Unlocked() {
		t.Fatal("the vault stayed open after its PIN was removed")
	}
	if _, err := service.BeginRekey(""); !errors.Is(err, ErrNotReady) {
		t.Fatalf("a new phrase after the PIN was removed: got %v, want ErrNotReady", err)
	}
}

func TestAnotherDeviceNeedsTheNewRecoveryPhrase(t *testing.T) {
	devices := newSyncedDevices(t)
	if err := devices.phone.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	phrase, err := devices.mac.BeginRekey("")
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
	phrase, err := mac.BeginRekey("")
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

func TestANewPhraseInterruptedAfterTheFileIsReplacedNeedsThatPhrase(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	service := newTestService(t, files, keys)
	createTestVaultWith(t, service, MethodChoice{PIN: testPIN})
	phrase, err := service.BeginRekey(testPIN)
	if err != nil {
		t.Fatal(err)
	}
	files.failAfterReplace = true
	if _, err := service.ConfirmRekey(phrase); !errors.Is(err, storage.ErrDurabilityUncertain) {
		t.Fatalf("an interrupted save: got %v, want ErrDurabilityUncertain", err)
	}
	if service.Unlocked() {
		t.Fatal("the vault stayed open after an interrupted save")
	}
	if _, err := service.UnlockWithPIN(testPIN); !errors.Is(err, vault.ErrKeyReplaced) {
		t.Fatalf("the PIN whose record was not moved: got %v, want ErrKeyReplaced", err)
	}
	if _, err := service.BeginRecovery(phrase); err != nil {
		t.Fatalf("the new phrase does not open the replaced file: %v", err)
	}
}

func TestANewPhraseForAFileThatChangedLeavesTheVaultAsItWas(t *testing.T) {
	devices := newSyncedDevices(t)
	if err := devices.mac.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	phrase, err := devices.mac.BeginRekey(testPIN)
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
	if _, err := service.BeginRekey(testPIN); !errors.Is(err, unlock.ErrNoPIN) {
		t.Fatalf("a PIN for a vault without one: got %v, want ErrNoPIN", err)
	}
	if _, err := service.BeginRekey(""); err != nil {
		t.Fatal(err)
	}
	if err := service.SetPIN(testPIN); !errors.Is(err, ErrSetupInProgress) {
		t.Fatalf("setting a PIN during a new phrase: got %v, want ErrSetupInProgress", err)
	}
	if err := service.SetBiometryUnlock(false); !errors.Is(err, ErrSetupInProgress) {
		t.Fatalf("changing device authentication during a new phrase: got %v, want ErrSetupInProgress", err)
	}
	service.DiscardRekey()
	if err := service.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	phrase, err := service.BeginRekey(testPIN)
	if err != nil {
		t.Fatal(err)
	}
	reopenTestVault(t, service)
	if _, err := service.ConfirmRekey(phrase); !errors.Is(err, ErrNoPendingSetup) {
		t.Fatalf("a new phrase after the vault locked: got %v, want ErrNoPendingSetup", err)
	}
}
