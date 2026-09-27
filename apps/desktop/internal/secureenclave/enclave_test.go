//go:build darwin && arm64 && cgo && secureenclave

package secureenclave

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/unlock"
)

var testSalt = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

func mustCreate(t *testing.T) ([]byte, []byte, [unlock.SecretSize]byte) {
	t.Helper()
	return mustCreateFor(t, testSalt)
}

func mustCreateFor(t *testing.T, salt []byte) ([]byte, []byte, [unlock.SecretSize]byte) {
	t.Helper()
	boundKey, peerKey, secret, err := Enclave{}.Create(salt)
	if err != nil {
		t.Fatal(err)
	}
	return boundKey, peerKey, secret
}

func otherSalt() []byte {
	salt := bytes.Clone(testSalt)
	salt[0] ^= 0xff
	return salt
}

func TestTheKeptKeysYieldTheSecretAgain(t *testing.T) {
	boundKey, peerKey, secret := mustCreate(t)
	// The device records keep a keychain label, never the key's representation.
	if len(boundKey) != boundKeyBytes || len(peerKey) != peerKeyBytes {
		t.Fatalf("kept %d bound key bytes and %d peer key bytes", len(boundKey), len(peerKey))
	}
	if secret == [unlock.SecretSize]byte{} {
		t.Fatal("the secret is empty")
	}
	again, err := Enclave{}.Derive(boundKey, peerKey, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	if again != secret {
		t.Fatal("the kept keys yielded a different secret")
	}
	if bytes.Contains(boundKey, secret[:]) || bytes.Contains(peerKey, secret[:]) {
		t.Fatal("the secret is stored in what is kept")
	}
}

func TestUsingAKeyRemovesTheOthersMadeForTheSameSalt(t *testing.T) {
	firstKey, firstPeer, first := mustCreate(t)
	secondKey, secondPeer, second := mustCreate(t)
	if bytes.Equal(firstKey, secondKey) || bytes.Equal(firstPeer, secondPeer) || first == second {
		t.Fatal("two bindings share a key or a secret")
	}
	// A record never stored for the second key leaves the first one usable.
	if again, err := (Enclave{}).Derive(firstKey, firstPeer, testSalt); err != nil || again != first {
		t.Fatalf("the earlier key before the new one was used: %v", err)
	}
	if _, err := (Enclave{}).Derive(secondKey, secondPeer, testSalt); !errors.Is(err, ErrRejected) {
		t.Fatalf("the new key after the earlier one was used: got %v, want ErrRejected", err)
	}
	thirdKey, thirdPeer, third := mustCreate(t)
	otherKey, otherPeer, other := mustCreateFor(t, otherSalt())
	if again, err := (Enclave{}).Derive(thirdKey, thirdPeer, testSalt); err != nil || again != third {
		t.Fatalf("the newest key: %v", err)
	}
	if _, err := (Enclave{}).Derive(firstKey, firstPeer, testSalt); !errors.Is(err, ErrRejected) {
		t.Fatalf("the earlier key after the newest one was used: got %v, want ErrRejected", err)
	}
	if again, err := (Enclave{}).Derive(otherKey, otherPeer, otherSalt()); err != nil || again != other {
		t.Fatalf("a key for another salt was removed: %v", err)
	}
}

func TestPINAndPresenceKeysForTheSameSaltAreKeptApart(t *testing.T) {
	presenceKey, presencePeer := mustCreateWithPresence(t)
	pinKey, pinPeer, secret := mustCreate(t)
	if again, err := (Enclave{}).Derive(pinKey, pinPeer, testSalt); err != nil || again != secret {
		t.Fatalf("the PIN key: %v", err)
	}
	if _, err := deriveBinding(policyUserPresence, presenceKey, presencePeer, testSalt, ""); !errors.Is(err, ErrInteractionRequired) {
		t.Fatalf("the presence key after the PIN key was used: got %v, want ErrInteractionRequired", err)
	}
}

func TestAKeyAnswersOnlyForItsOwnSalt(t *testing.T) {
	boundKey, peerKey, _ := mustCreate(t)
	if _, err := (Enclave{}).Derive(boundKey, peerKey, otherSalt()); !errors.Is(err, ErrRejected) {
		t.Fatalf("another salt: got %v, want ErrRejected", err)
	}
}

func TestAKeyTheSecureEnclaveDidNotMakeIsRejected(t *testing.T) {
	boundKey, peerKey, _ := mustCreate(t)
	tampered := bytes.Clone(boundKey)
	tampered[len(tampered)/2] ^= 0xff
	cases := map[string][2][]byte{
		"a tampered bound key":     {tampered, peerKey},
		"a truncated bound key":    {boundKey[:len(boundKey)/2], peerKey},
		"random bytes":             {bytes.Repeat([]byte{0x5a}, len(boundKey)), peerKey},
		"a peer key off the curve": {boundKey, bytes.Repeat([]byte{4}, peerKeyBytes)},
	}
	for name, keys := range cases {
		if _, err := (Enclave{}).Derive(keys[0], keys[1], testSalt); !errors.Is(err, ErrRejected) {
			t.Errorf("%s: got %v, want ErrRejected", name, err)
		}
	}
}

func mustCreateWithPresence(t *testing.T) ([]byte, []byte) {
	t.Helper()
	boundKey, peerKey, secret, err := PresenceEnclave{}.Create(testSalt)
	if err != nil {
		t.Fatal(err)
	}
	if len(boundKey) != boundKeyBytes || len(peerKey) != peerKeyBytes || secret == [unlock.SecretSize]byte{} {
		t.Fatalf("kept %d bound key bytes and %d peer key bytes", len(boundKey), len(peerKey))
	}
	return boundKey, peerKey
}

// Every derive here passes no reason, which forbids a prompt.
func TestAKeyThatNeedsItsOwnerIsNotUsedWithoutThem(t *testing.T) {
	boundKey, peerKey := mustCreateWithPresence(t)
	if _, err := deriveBinding(policyUserPresence, boundKey, peerKey, testSalt, ""); !errors.Is(err, ErrInteractionRequired) {
		t.Fatalf("a derive the owner did not allow: got %v, want ErrInteractionRequired", err)
	}
	if _, err := (Enclave{}).Derive(boundKey, peerKey, testSalt); !errors.Is(err, ErrRejected) {
		t.Fatalf("a PIN derive of a presence key's label: got %v, want ErrRejected", err)
	}
}

func TestEveryPresenceBindingMakesANewKey(t *testing.T) {
	firstKey, firstPeer := mustCreateWithPresence(t)
	secondKey, secondPeer := mustCreateWithPresence(t)
	if bytes.Equal(firstKey, secondKey) || bytes.Equal(firstPeer, secondPeer) {
		t.Fatal("two bindings share a key")
	}
}

func TestAPresenceDeriveWithoutAReasonIsRefused(t *testing.T) {
	boundKey, peerKey := mustCreateWithPresence(t)
	if _, err := (PresenceEnclave{}).Derive("", boundKey, peerKey, testSalt); !errors.Is(err, ErrNoReason) {
		t.Fatalf("a derive without a reason: got %v, want ErrNoReason", err)
	}
}

func TestAPresenceKeyTheSecureEnclaveDidNotMakeIsRejected(t *testing.T) {
	boundKey, peerKey := mustCreateWithPresence(t)
	if _, err := deriveBinding(policyUserPresence, bytes.Repeat([]byte{0x5a}, len(boundKey)), peerKey, testSalt, ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("random bytes: got %v, want ErrRejected", err)
	}
}

func TestEmptyInputsAreRefused(t *testing.T) {
	if _, _, _, err := (Enclave{}).Create(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("create without a salt: got %v, want ErrInvalid", err)
	}
	if _, _, _, err := (PresenceEnclave{}).Create(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a presence binding without a salt: got %v, want ErrInvalid", err)
	}
	boundKey, peerKey, _ := mustCreate(t)
	for name, inputs := range map[string][3][]byte{
		"no bound key": {nil, peerKey, testSalt},
		"no peer key":  {boundKey, nil, testSalt},
		"no salt":      {boundKey, peerKey, nil},
	} {
		if _, err := (Enclave{}).Derive(inputs[0], inputs[1], inputs[2]); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}
