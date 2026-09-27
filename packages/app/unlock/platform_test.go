package unlock

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/unlock/unlocktest"
)

const testReason = "unlock your vault"

func newTestPlatformCredentials(t *testing.T) (*PlatformCredentials, *unlocktest.PresenceBinding) {
	t.Helper()
	binding := unlocktest.NewPresenceBinding()
	credentials, err := NewPlatformCredentials(binding)
	if err != nil {
		t.Fatal(err)
	}
	return credentials, binding
}

func setTestPlatform(t *testing.T) *PlatformCredential {
	t.Helper()
	credentials, _ := newTestPlatformCredentials(t)
	record, err := credentials.Set(testVaultID, wrapNothing)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestAPlatformCredentialWrapsTheVaultKeyWithoutAskingTheOwner(t *testing.T) {
	credentials, binding := newTestPlatformCredentials(t)
	record, err := credentials.Set(testVaultID, wrapNothing)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Created() != 1 || len(binding.Prompts()) != 0 {
		t.Fatalf("binding made %d keys and showed %d prompts", binding.Created(), len(binding.Prompts()))
	}
	key, err := credentials.Key(record, testVaultID, testReason)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(record.Envelope, append([]byte("envelope"), key[:]...)) {
		t.Fatal("the key derived through the owner is not the one the vault key was wrapped for")
	}
	if !slices.Equal(binding.Prompts(), []string{testReason}) {
		t.Fatalf("the owner was asked %q", binding.Prompts())
	}
}

func TestEveryPlatformCredentialBindsANewHardwareKey(t *testing.T) {
	credentials, binding := newTestPlatformCredentials(t)
	first, err := credentials.Set(testVaultID, wrapNothing)
	if err != nil {
		t.Fatal(err)
	}
	second, err := credentials.Set(testVaultID, wrapNothing)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Created() != 2 || bytes.Equal(first.BoundKey, second.BoundKey) || bytes.Equal(first.Envelope, second.Envelope) {
		t.Fatal("binding again reused the earlier hardware key")
	}
}

func TestAnOwnerWhoIsNotVerifiedGetsNoKey(t *testing.T) {
	credentials, binding := newTestPlatformCredentials(t)
	record, err := credentials.Set(testVaultID, wrapNothing)
	if err != nil {
		t.Fatal(err)
	}
	for _, answer := range []error{ownerauth.ErrCanceled, ownerauth.ErrFailed, ownerauth.ErrUnavailable} {
		binding.Answer(answer)
		key, err := credentials.Key(record, testVaultID, testReason)
		if !errors.Is(err, answer) || errors.Is(err, ErrUnbound) {
			t.Fatalf("an owner answering %v: got %v", answer, err)
		}
		if key != [SecretSize]byte{} {
			t.Fatal("an owner who was not verified got a key")
		}
	}
}

func TestAPlatformCredentialTheHardwareRejectsIsUnbound(t *testing.T) {
	credentials, binding := newTestPlatformCredentials(t)
	record, err := credentials.Set(testVaultID, wrapNothing)
	if err != nil {
		t.Fatal(err)
	}
	otherVault := testVaultID
	otherVault[0] ^= 0xff
	if _, err := credentials.Key(record, otherVault, testReason); !errors.Is(err, ErrUnbound) {
		t.Fatalf("a credential bound for another vault: got %v, want ErrUnbound", err)
	}
	binding.Reset()
	if _, err := credentials.Key(record, testVaultID, testReason); !errors.Is(err, ErrUnbound) {
		t.Fatalf("a key the hardware no longer holds: got %v, want ErrUnbound", err)
	}
}

func TestAHardwareFailureSetsNoPlatformCredential(t *testing.T) {
	credentials, binding := newTestPlatformCredentials(t)
	failure := errors.New("the hardware refused")
	binding.FailCreate(failure)
	wrapped := false
	record, err := credentials.Set(testVaultID, func(key []byte) ([]byte, error) {
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
