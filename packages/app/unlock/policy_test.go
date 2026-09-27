package unlock

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/unlock/unlocktest"
	"github.com/dortanes/ravenpass/packages/vault"
)

var testVaultID = vault.ID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

func wrapNothing(key []byte) ([]byte, error) { return append([]byte("envelope"), key...), nil }

func newTestPINs(t *testing.T) (*PINs, *unlocktest.Binding) {
	t.Helper()
	binding := unlocktest.NewBinding()
	pins, err := NewPINs(binding, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pins, binding
}

func setTestPIN(t *testing.T, pins *PINs, pin string) *PIN {
	t.Helper()
	record, err := pins.Set(pin, testVaultID, wrapNothing)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestARecordSurvivesTheRoundTrip(t *testing.T) {
	pins, _ := newTestPINs(t)
	setup := setTestPIN(t, pins, "123456")
	platform := setTestPlatform(t)
	stored, err := Policy{Platform: platform, PIN: setup}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := Decode(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !read.HasPlatform() || !read.HasPIN() {
		t.Fatalf("the record came back as %+v", read)
	}
	if !bytes.Equal(read.PIN.Salt, setup.Salt) || !bytes.Equal(read.PIN.BoundKey, setup.BoundKey) ||
		!bytes.Equal(read.PIN.PeerKey, setup.PeerKey) || !bytes.Equal(read.PIN.Envelope, setup.Envelope) {
		t.Fatal("the PIN's material did not survive the round trip")
	}
	if !bytes.Equal(read.Platform.BoundKey, platform.BoundKey) || !bytes.Equal(read.Platform.PeerKey, platform.PeerKey) ||
		!bytes.Equal(read.Platform.Envelope, platform.Envelope) {
		t.Fatal("the platform credential did not survive the round trip")
	}
	if bytes.Contains(stored, []byte("123456")) {
		t.Fatal("the record holds the PIN itself")
	}
}

func TestARecordWithNothingOpensOnlyWithTheRecoveryKey(t *testing.T) {
	var policy Policy
	if policy.HasPlatform() || policy.HasPIN() || policy.Usable(true) {
		t.Fatalf("an empty record offers a way in: %+v", policy)
	}
}

func TestAPolicyIsUsableWhileOneWayRemains(t *testing.T) {
	pins, _ := newTestPINs(t)
	if !(Policy{PIN: setTestPIN(t, pins, "654321")}).Usable(false) {
		t.Fatal("a PIN alone was reported as no way in")
	}
	platform := Policy{Platform: setTestPlatform(t)}
	if !platform.Usable(true) {
		t.Fatal("device authentication alone was reported as no way in")
	}
	if platform.Usable(false) {
		t.Fatal("device authentication was reported as a way in where the device cannot authenticate")
	}
}

func TestARecordOfAnotherVersionIsUnsupported(t *testing.T) {
	cases := map[string][]byte{
		"no version":  []byte(`{"biometry":true}`),
		"version one": []byte(`{"version":1,"biometry":true}`),
		"a newer one": []byte(`{"version":3}`),
	}
	for name, data := range cases {
		if _, err := Decode(data); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: got %v, want ErrUnsupported", name, err)
		}
	}
}

func TestAnUnreadableRecordIsReportedRatherThanReplaced(t *testing.T) {
	pins, _ := newTestPINs(t)
	setup := setTestPIN(t, pins, "123456")
	platform := setTestPlatform(t)
	cases := map[string][]byte{
		"not JSON":         []byte("{"),
		"a newer version":  []byte(`{"version":99,"platform":{}}`),
		"a truncated salt": mustEncode(t, Policy{PIN: &PIN{Salt: setup.Salt[:4], BoundKey: setup.BoundKey, PeerKey: setup.PeerKey, Envelope: setup.Envelope}}),
		"no bound key":     mustEncode(t, Policy{PIN: &PIN{Salt: setup.Salt, PeerKey: setup.PeerKey, Envelope: setup.Envelope}}),
		"no peer key":      mustEncode(t, Policy{PIN: &PIN{Salt: setup.Salt, BoundKey: setup.BoundKey, Envelope: setup.Envelope}}),
		"no envelope":      mustEncode(t, Policy{PIN: &PIN{Salt: setup.Salt, BoundKey: setup.BoundKey, PeerKey: setup.PeerKey}}),
		"spent attempts":   mustEncode(t, Policy{PIN: &PIN{Salt: setup.Salt, BoundKey: setup.BoundKey, PeerKey: setup.PeerKey, Envelope: setup.Envelope, Failures: MaxPINFailures}}),
		"a platform credential without a bound key": mustEncode(t, Policy{Platform: &PlatformCredential{PeerKey: platform.PeerKey, Envelope: platform.Envelope}}),
		"a platform credential without a peer key":  mustEncode(t, Policy{Platform: &PlatformCredential{BoundKey: platform.BoundKey, Envelope: platform.Envelope}}),
		"a platform credential without an envelope": mustEncode(t, Policy{Platform: &PlatformCredential{BoundKey: platform.BoundKey, PeerKey: platform.PeerKey}}),
		"a platform credential of the wrong type":   []byte(`{"version":2,"platform":true}`),
	}
	for name, data := range cases {
		if _, err := Decode(data); !errors.Is(err, ErrMalformed) && !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: got %v, want it refused", name, err)
		}
	}
}

func TestOnlyAPINWithinTheRuleGetsMaterial(t *testing.T) {
	pins, binding := newTestPINs(t)
	if _, err := pins.Set("12345", testVaultID, wrapNothing); !errors.Is(err, vault.ErrInvalidPIN) {
		t.Fatalf("a short PIN: got %v, want ErrInvalidPIN", err)
	}
	if binding.Created() != 0 {
		t.Fatal("a refused PIN made a hardware key")
	}
}

func TestAPINDerivesItsKeyFromWhatTheRecordHolds(t *testing.T) {
	pins, _ := newTestPINs(t)
	setup := setTestPIN(t, pins, "123456")
	key, err := pins.Key(setup, "123456", testVaultID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(setup.Envelope, append([]byte("envelope"), key[:]...)) {
		t.Fatal("the key derived from the PIN is not the one the vault key was wrapped for")
	}
	if setup.Remaining() != MaxPINFailures {
		t.Fatalf("a new PIN has %d attempts left", setup.Remaining())
	}
	if !pins.Bound(setup, testVaultID) {
		t.Fatal("the device does not accept the key it just bound")
	}
}

func TestEverySetBindsANewHardwareKey(t *testing.T) {
	pins, binding := newTestPINs(t)
	first := setTestPIN(t, pins, "123456")
	second := setTestPIN(t, pins, "123456")
	if binding.Created() != 2 {
		t.Fatalf("two PINs made %d hardware keys", binding.Created())
	}
	if bytes.Equal(first.BoundKey, second.BoundKey) || bytes.Equal(first.PeerKey, second.PeerKey) {
		t.Fatal("setting the PIN again reused the earlier binding")
	}
	one, err := pins.Key(first, "123456", testVaultID)
	if err != nil {
		t.Fatal(err)
	}
	other, err := pins.Key(second, "123456", testVaultID)
	if err != nil {
		t.Fatal(err)
	}
	if one == other {
		t.Fatal("two bindings of the same PIN derived the same key")
	}
}

func TestAKeyTheHardwareRejectsIsUnbound(t *testing.T) {
	pins, binding := newTestPINs(t)
	setup := setTestPIN(t, pins, "123456")
	otherVault := testVaultID
	otherVault[0] ^= 0xff
	if pins.Bound(setup, otherVault) {
		t.Fatal("a PIN bound for one vault was accepted for another")
	}
	binding.Reset()
	if pins.Bound(setup, testVaultID) {
		t.Fatal("a key the hardware no longer holds was reported bound")
	}
	if _, err := pins.Key(setup, "123456", testVaultID); !errors.Is(err, ErrUnbound) {
		t.Fatalf("an attempt at an unbound PIN: got %v, want ErrUnbound", err)
	}
}

func TestAHardwareFailureSetsNoPIN(t *testing.T) {
	pins, binding := newTestPINs(t)
	failure := errors.New("the hardware refused")
	binding.FailCreate(failure)
	wrapped := false
	record, err := pins.Set("123456", testVaultID, func(key []byte) ([]byte, error) {
		wrapped = true
		return wrapNothing(key)
	})
	if record != nil || !errors.Is(err, failure) {
		t.Fatalf("a failed binding: got %v, %v", record, err)
	}
	if wrapped {
		t.Fatal("the vault key was wrapped without a binding")
	}
}

func mustEncode(t *testing.T, policy Policy) []byte {
	t.Helper()
	data, err := policy.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return data
}
