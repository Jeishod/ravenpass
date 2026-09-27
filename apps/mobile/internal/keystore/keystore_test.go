package keystore

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/ownerauth"
)

// softwareKeystore keeps its keys in memory and answers the owner prompt with answer.
type softwareKeystore struct {
	keys    map[string]*ecdh.PrivateKey
	sealing map[string]*rsa.PrivateKey
	answer  error
	reasons []string
	rsaBits int
}

func newSoftwareKeystore() *softwareKeystore {
	return &softwareKeystore{keys: map[string]*ecdh.PrivateKey{}, sealing: map[string]*rsa.PrivateKey{},
		rsaBits: minPresenceKeyBits}
}

func (s *softwareKeystore) CreateKey(alias string, presence bool) ([]byte, error) {
	if presence {
		key, err := rsa.GenerateKey(rand.Reader, s.rsaBits)
		if err != nil {
			return nil, err
		}
		s.sealing[alias] = key
		return x509.MarshalPKIXPublicKey(&key.PublicKey)
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	s.keys[alias] = key
	return x509.MarshalPKIXPublicKey(key.PublicKey())
}

func (s *softwareKeystore) Agree(alias string, peer []byte) ([]byte, error) {
	key, known := s.keys[alias]
	if !known {
		return nil, errors.New("no such key agreement key")
	}
	parsed, err := ecdhPublicKey(peer)
	if err != nil {
		return nil, err
	}
	return key.ECDH(parsed)
}

func (s *softwareKeystore) Decrypt(alias string, ciphertext []byte, reason string) ([]byte, error) {
	key, known := s.sealing[alias]
	if !known {
		return nil, errors.New("no such decryption key")
	}
	s.reasons = append(s.reasons, reason)
	if s.answer != nil {
		return nil, s.answer
	}
	return key.Decrypt(nil, ciphertext, &presenceOAEP)
}

var salt = bytes.Repeat([]byte{7}, 16)

func TestAPINBindingYieldsItsSecretAgainWithoutAsking(t *testing.T) {
	hardware := newSoftwareKeystore()
	binding := Binding{Hardware: hardware}
	boundKey, peerKey, created, err := binding.Create(salt)
	if err != nil {
		t.Fatal(err)
	}
	if _, agreement := hardware.keys[string(boundKey)]; !strings.HasPrefix(string(boundKey), "ravenpass.pin.") || !agreement {
		t.Fatalf("a PIN binding made %q, a key agreement key %v", boundKey, agreement)
	}
	derived, err := binding.Derive(boundKey, peerKey, salt)
	if err != nil || derived != created {
		t.Fatalf("derive = %x, %v; want %x", derived, err, created)
	}
	if len(hardware.reasons) != 0 {
		t.Fatalf("a PIN binding asked the owner %q", hardware.reasons)
	}
	if other, _ := binding.Derive(boundKey, peerKey, bytes.Repeat([]byte{8}, 16)); other == created {
		t.Fatal("another salt yields the same secret")
	}
}

func TestAPresenceBindingAsksTheOwnerOnlyWhenUsed(t *testing.T) {
	hardware := newSoftwareKeystore()
	binding := PresenceBinding{Hardware: hardware}
	boundKey, peerKey, created, err := binding.Create(salt)
	if err != nil {
		t.Fatal(err)
	}
	if _, sealing := hardware.sealing[string(boundKey)]; !strings.HasPrefix(string(boundKey), "ravenpass.unlock.") || !sealing {
		t.Fatalf("a presence binding made %q, a decryption key %v", boundKey, sealing)
	}
	if len(hardware.reasons) != 0 {
		t.Fatalf("creating a presence binding asked the owner %q", hardware.reasons)
	}
	derived, err := binding.Derive("unlock your vault", boundKey, peerKey, salt)
	if err != nil || derived != created {
		t.Fatalf("derive = %x, %v; want %x", derived, err, created)
	}
	if len(hardware.reasons) != 1 || hardware.reasons[0] != "unlock your vault" {
		t.Fatalf("the owner was asked %q", hardware.reasons)
	}
}

func TestAPresenceBindingPassesTheOwnersAnswerOn(t *testing.T) {
	hardware := newSoftwareKeystore()
	binding := PresenceBinding{Hardware: hardware}
	boundKey, peerKey, _, err := binding.Create(salt)
	if err != nil {
		t.Fatal(err)
	}
	for _, answer := range []error{ownerauth.ErrCanceled, ownerauth.ErrFailed, ownerauth.ErrUnavailable} {
		hardware.answer = answer
		if _, err := binding.Derive("unlock your vault", boundKey, peerKey, salt); !errors.Is(err, answer) {
			t.Fatalf("an owner answering %v: got %v", answer, err)
		}
	}
	if _, err := binding.Derive("", boundKey, peerKey, salt); !errors.Is(err, ErrNoReason) {
		t.Fatalf("a prompt without a reason: got %v", err)
	}
}

func TestPoliciesDeriveApartFromTheSameAgreement(t *testing.T) {
	shared := bytes.Repeat([]byte{1}, 32)
	pin, err := expand(bytes.Clone(shared), salt, pinPolicy)
	if err != nil {
		t.Fatal(err)
	}
	presence, err := expand(bytes.Clone(shared), salt, presencePolicy)
	if err != nil {
		t.Fatal(err)
	}
	if pin == presence {
		t.Fatal("a PIN and a presence binding derive the same secret from one agreement")
	}
}

func TestBindingsRefuseMissingParts(t *testing.T) {
	binding := Binding{Hardware: newSoftwareKeystore()}
	if _, _, _, err := binding.Create(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("create without a salt: got %v", err)
	}
	for name, parts := range map[string][3][]byte{
		"bound key": {nil, []byte("peer"), salt},
		"peer key":  {[]byte("key"), nil, salt},
		"salt":      {[]byte("key"), []byte("peer"), nil},
	} {
		if _, err := binding.Derive(parts[0], parts[1], parts[2]); !errors.Is(err, ErrInvalid) {
			t.Fatalf("derive without a %s: got %v", name, err)
		}
	}
}

func TestAKeyOfAnotherCurveIsRefused(t *testing.T) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKIXPublicKey(key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ecdhPublicKey(encoded); !errors.Is(err, ErrKey) {
		t.Fatalf("an X25519 key: got %v", err)
	}
}

func TestAPresenceKeyBelow2048BitsIsRefused(t *testing.T) {
	hardware := newSoftwareKeystore()
	hardware.rsaBits = 1024
	if _, _, _, err := (PresenceBinding{Hardware: hardware}).Create(salt); !errors.Is(err, ErrKey) {
		t.Fatalf("a 1024-bit presence key: got %v", err)
	}
}

func TestAPresenceBindingRefusesATamperedSeed(t *testing.T) {
	hardware := newSoftwareKeystore()
	binding := PresenceBinding{Hardware: hardware}
	boundKey, peerKey, _, err := binding.Create(salt)
	if err != nil {
		t.Fatal(err)
	}
	peerKey[len(peerKey)-1] ^= 1
	if _, err := binding.Derive("unlock your vault", boundKey, peerKey, salt); err == nil {
		t.Fatal("a tampered sealed seed derived a secret")
	}
}
