package vaultservice

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// syncedDevices is one vault in a synced folder: the desktop created it and the phone recovered it.
type syncedDevices struct {
	mac, phone           *Service
	macFiles, phoneFiles *memoryFiles
	macKeys              *memoryKeys
	// phrase is the recovery phrase the vault was created with.
	phrase string
}

func newSyncedDevices(t *testing.T) syncedDevices {
	t.Helper()
	devices := syncedDevices{macFiles: &memoryFiles{}, phoneFiles: &memoryFiles{}, macKeys: newMemoryKeys()}
	devices.mac = newTestService(t, devices.macFiles, devices.macKeys)
	phrase, _ := createTestVault(t, devices.mac)
	devices.phrase = phrase
	deliver(devices.macFiles, devices.phoneFiles)
	devices.phone = newTestService(t, devices.phoneFiles, newMemoryKeys())
	if _, err := devices.phone.BeginRecovery(phrase); err != nil {
		t.Fatal(err)
	}
	if _, err := devices.phone.ConfirmRecovery(true, MethodChoice{Biometry: true}); err != nil {
		t.Fatal(err)
	}
	return devices
}

// deliver brings the file one device saved to the other device's folder.
func deliver(from, to *memoryFiles) {
	from.mu.Lock()
	data := bytes.Clone(from.data)
	from.mu.Unlock()
	to.mu.Lock()
	to.data = data
	to.mu.Unlock()
}

func saveItems(t *testing.T, service *Service, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if _, err := service.CreateCredential(vault.CredentialInput{Label: label, Password: "secret"}, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func labelsOf(t *testing.T, service *Service) []string {
	t.Helper()
	entries, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	labels := make([]string, len(entries))
	for i, entry := range entries {
		labels[i] = entry.Label
	}
	return labels
}

func headOf(t *testing.T, service *Service) vault.Head {
	t.Helper()
	state, err := service.State()
	if err != nil || state.Phase != PhaseReady {
		t.Fatalf("state = %+v, error = %v", state, err)
	}
	return state.Head
}

func witnessOf(t *testing.T, service *Service, id vault.ID) vault.Witness {
	t.Helper()
	witness, err := service.loadWitness(id.String())
	if err != nil || witness == nil {
		t.Fatalf("witness = %v, error = %v", witness, err)
	}
	return *witness
}

func TestUnlockOpensAFileAnotherDeviceSavedSeveralTimes(t *testing.T) {
	devices := newSyncedDevices(t)
	devices.mac.Lock()
	saveItems(t, devices.phone, "one", "two", "three")
	deliver(devices.phoneFiles, devices.macFiles)
	if _, err := devices.mac.Unlock(testReason); err != nil {
		t.Fatalf("a file three saves ahead: %v", err)
	}
	head := headOf(t, devices.mac)
	if head != headOf(t, devices.phone) || witnessOf(t, devices.mac, head.VaultID) != vault.WitnessFor(head) {
		t.Fatal("the Mac did not acknowledge the phone's version")
	}
	if labels := labelsOf(t, devices.mac); len(labels) != 3 {
		t.Fatalf("the Mac shows %v", labels)
	}
}

func TestAnOpenVaultAdoptsTheSavesOfAnotherDevice(t *testing.T) {
	devices := newSyncedDevices(t)
	if change := devices.mac.Follow(); change != FileUnchanged {
		t.Fatalf("its own file: %v", change)
	}
	saveItems(t, devices.phone, "one", "two")
	deliver(devices.phoneFiles, devices.macFiles)
	if change := devices.mac.Follow(); change != FileAdopted {
		t.Fatalf("the phone's saves: %v", change)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if count, err := devices.mac.AwaitVaultChange(ctx, 0); err != nil || count != 1 {
		t.Fatalf("change count = %d, error = %v", count, err)
	}
	head := headOf(t, devices.mac)
	if head != headOf(t, devices.phone) || witnessOf(t, devices.mac, head.VaultID) != vault.WitnessFor(head) {
		t.Fatal("the Mac did not acknowledge the phone's version")
	}
	if change := devices.mac.Follow(); change != FileUnchanged {
		t.Fatalf("the adopted file again: %v", change)
	}
	saveItems(t, devices.mac, "saved on the Mac")
	deliver(devices.macFiles, devices.phoneFiles)
	if change := devices.phone.Follow(); change != FileAdopted || len(labelsOf(t, devices.phone)) != 3 {
		t.Fatalf("the phone following back: %v", change)
	}
}

func TestFollowingLeavesTheVaultOpenWhileTheFileIsOutOfReach(t *testing.T) {
	devices := newSyncedDevices(t)
	devices.macFiles.mu.Lock()
	devices.macFiles.unreadable = errors.New("the folder is out of reach")
	devices.macFiles.mu.Unlock()
	if change := devices.mac.Follow(); change != FileUnchanged || !devices.mac.Unlocked() {
		t.Fatalf("a file out of reach: %v", change)
	}
	devices.macFiles.unreadable = nil
	devices.mac.Lock()
	if change := devices.mac.Follow(); change != FileUnchanged {
		t.Fatalf("a locked vault: %v", change)
	}
}

func TestAnOlderFileLocksTheOpenVaultAndIsRefused(t *testing.T) {
	devices := newSyncedDevices(t)
	deliver(devices.macFiles, devices.phoneFiles)
	saveItems(t, devices.mac, "newer")
	deliver(devices.phoneFiles, devices.macFiles)
	if change := devices.mac.Follow(); change != FileRefused || devices.mac.Unlocked() {
		t.Fatalf("an older file: %v", change)
	}
	if _, err := devices.mac.Unlock(testReason); !errors.Is(err, ErrOlderCopy) {
		t.Fatalf("unlocking an older file: got %v, want ErrOlderCopy", err)
	}
}

func TestChangesMadeAtTheSameRevisionAreOfferedAsDiverged(t *testing.T) {
	devices := newSyncedDevices(t)
	saveItems(t, devices.mac, "saved on the Mac")
	saveItems(t, devices.phone, "saved on the phone")
	deliver(devices.phoneFiles, devices.macFiles)
	if change := devices.mac.Follow(); change != FileRefused {
		t.Fatalf("a file at the same revision: %v", change)
	}
	if _, err := devices.mac.Unlock(testReason); !errors.Is(err, ErrDiverged) {
		t.Fatalf("unlocking it: got %v, want ErrDiverged", err)
	}
	head, err := devices.mac.AdoptDiverged()
	if err != nil {
		t.Fatal(err)
	}
	if head != headOf(t, devices.phone) {
		t.Fatal("the Mac did not adopt the phone's version")
	}
}

func TestADivergedFileOpensOnlyOnceItsOwnerAdoptsIt(t *testing.T) {
	devices := newSyncedDevices(t)
	saveItems(t, devices.mac, "saved on the Mac")
	macHead := headOf(t, devices.mac)
	saveItems(t, devices.phone, "saved on the phone", "saved on the phone again")
	deliver(devices.phoneFiles, devices.macFiles)
	if change := devices.mac.Follow(); change != FileRefused || devices.mac.Unlocked() {
		t.Fatalf("a diverged file: %v", change)
	}
	if _, err := devices.mac.AdoptDiverged(); !errors.Is(err, ErrNoPendingSetup) {
		t.Fatalf("adopting before an unlock: got %v, want ErrNoPendingSetup", err)
	}
	if _, err := devices.mac.Unlock(testReason); !errors.Is(err, ErrDiverged) {
		t.Fatalf("unlocking a diverged file: got %v, want ErrDiverged", err)
	}
	if state, err := devices.mac.State(); err != nil || state.Phase != PhaseDiverged {
		t.Fatalf("state while held = %+v, error = %v", state, err)
	}
	if _, err := devices.mac.List(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("a held vault was readable: %v", err)
	}
	if witnessOf(t, devices.mac, macHead.VaultID) != vault.WitnessFor(macHead) {
		t.Fatal("holding the file moved the Mac's witness")
	}
	if _, err := devices.mac.Unlock(testReason); !errors.Is(err, ErrDiverged) {
		t.Fatalf("unlocking it again: got %v, want ErrDiverged", err)
	}
	devices.mac.Lock()
	if _, err := devices.mac.AdoptDiverged(); !errors.Is(err, ErrNoPendingSetup) {
		t.Fatalf("adopting after a lock: got %v, want ErrNoPendingSetup", err)
	}
	if _, err := devices.mac.Unlock(testReason); !errors.Is(err, ErrDiverged) {
		t.Fatal(err)
	}
	head, err := devices.mac.AdoptDiverged()
	if err != nil {
		t.Fatal(err)
	}
	if head != headOf(t, devices.phone) || witnessOf(t, devices.mac, head.VaultID) != vault.WitnessFor(head) {
		t.Fatal("the Mac did not acknowledge the phone's version")
	}
	if labels := labelsOf(t, devices.mac); len(labels) != 2 {
		t.Fatalf("the adopted version shows %v", labels)
	}
	saveItems(t, devices.mac, "saved after adopting")
	deliver(devices.macFiles, devices.phoneFiles)
	if change := devices.phone.Follow(); change != FileAdopted {
		t.Fatalf("the phone following the adopted version: %v", change)
	}
}
