package vaultservice

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/unlock"
)

func TestAPINAttemptWhoseCountCannotBeStoredIsRefused(t *testing.T) {
	service, keys := readyVault(t)
	if err := service.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	name := service.device.policyVault
	stored := bytes.Clone(keys.policy[name])
	service.Lock()
	full := errors.New("the disk is full")
	for _, pin := range []string{"999999", testPIN} {
		keys.policySaveFailures = []error{full}
		if _, err := service.UnlockWithPIN(pin); !errors.Is(err, full) {
			t.Fatalf("PIN %s with a record that cannot be stored: got %v, want the store's failure", pin, err)
		}
		if service.Unlocked() {
			t.Fatalf("PIN %s opened the vault without its attempt being counted", pin)
		}
	}
	if stored == nil || !bytes.Equal(keys.policy[name], stored) {
		t.Fatal("a refused attempt changed the stored record")
	}
	methods, err := service.UnlockMethods()
	if err != nil || methods.PINAttemptsLeft != unlock.MaxPINFailures {
		t.Fatalf("refused attempts were counted: %+v, error = %v", methods, err)
	}
	if _, err := service.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("the PIN once the record can be stored again: %v", err)
	}
	keys.policySaveFailures = []error{full}
	if err := service.VerifyPIN(testPIN); !errors.Is(err, full) {
		t.Fatalf("checking the PIN with a record that cannot be stored: got %v, want the store's failure", err)
	}
}

func TestAPINAttemptIsStoredAsWrongBeforeItIsTried(t *testing.T) {
	service, keys := readyVault(t)
	if err := service.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	full := errors.New("the disk is full")
	// The first save counts the attempt; the second, clearing the count after the right PIN, fails.
	keys.policySaveFailures = []error{nil, full}
	if _, err := service.UnlockWithPIN(testPIN); !errors.Is(err, full) {
		t.Fatalf("a right PIN whose count cannot be cleared: got %v, want the store's failure", err)
	}
	if service.Unlocked() {
		t.Fatal("the vault opened although its record was not stored")
	}
	if pin := storedPIN(t, service, keys); pin.Failures != 1 {
		t.Fatalf("the stored record holds %d wrong PINs, want the attempt counted", pin.Failures)
	}
}

func TestTheLastPINAttemptRemovesThePINBeforeItIsTried(t *testing.T) {
	service, keys := readyVault(t)
	if err := service.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	for attempt := 1; attempt < unlock.MaxPINFailures; attempt++ {
		if _, err := service.UnlockWithPIN("999999"); !errors.Is(err, unlock.ErrWrongPIN) {
			t.Fatalf("attempt %d: got %v, want ErrWrongPIN", attempt, err)
		}
	}
	full := errors.New("the disk is full")
	keys.policySaveFailures = []error{nil, full}
	if _, err := service.UnlockWithPIN(testPIN); !errors.Is(err, full) {
		t.Fatalf("the last attempt whose PIN cannot be restored: got %v, want the store's failure", err)
	}
	if storedPolicy(t, service, keys).HasPIN() {
		t.Fatal("the PIN stayed stored while its last attempt ran")
	}
	if _, err := service.UnlockWithPIN(testPIN); !errors.Is(err, unlock.ErrNoPIN) {
		t.Fatalf("the PIN after its last attempt: got %v, want ErrNoPIN", err)
	}
}

func TestAPINTheDeviceCannotUseIsDroppedFromTheRecord(t *testing.T) {
	service, keys, device := readyVaultOnDevice(t)
	if err := service.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	name := service.device.policyVault
	service.Lock()
	device.pin.Reset()
	if methods, err := service.UnlockMethods(); err != nil || methods.PINSet {
		t.Fatalf("a PIN the device cannot use is offered: %+v, error = %v", methods, err)
	}
	policy, err := unlock.Decode(keys.policy[name])
	if err != nil {
		t.Fatal(err)
	}
	if policy.HasPIN() || !policy.HasPlatform() {
		t.Fatalf("the stored record holds %+v; want device authentication alone", policy)
	}
}

func TestTheDelayEarnedByWrongPINsOutlivesARestart(t *testing.T) {
	files := &memoryFiles{}
	keys := newMemoryKeys()
	device := newTestDevice()
	service := newTestServiceOn(t, files, keys, device)
	createTestVault(t, service)
	if err := service.SetPIN(testPIN); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	for range 3 {
		if _, err := service.UnlockWithPIN("999999"); !errors.Is(err, unlock.ErrWrongPIN) {
			t.Fatalf("a wrong PIN: got %v, want ErrWrongPIN", err)
		}
	}
	restarted := newTestServiceOn(t, files, keys, device)
	now := time.Unix(0, 0)
	restarted.pinThrottle = unlock.NewThrottle(func() time.Time { return now })
	if _, err := restarted.UnlockWithPIN(testPIN); !errors.Is(err, unlock.ErrTooSoon) {
		t.Fatalf("an attempt right after a restart: got %v, want ErrTooSoon", err)
	}
	now = now.Add(unlock.AttemptDelay(3))
	if _, err := restarted.UnlockWithPIN(testPIN); err != nil {
		t.Fatalf("an attempt once the delay passed: %v", err)
	}
}
