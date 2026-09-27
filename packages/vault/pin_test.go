package vault

import (
	"bytes"
	"errors"
	"testing"
)

func pinInputs(t *testing.T) ([]byte, []byte) {
	t.Helper()
	salt, err := NewPINSalt()
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, PINSecretSize)
	if err := randomBytes(secret); err != nil {
		t.Fatal(err)
	}
	return salt, secret
}

func TestAVaultKeyWrappedForAPINOpensWithThatPIN(t *testing.T) {
	session, _ := populatedSession(t)
	container, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	salt, secret := pinInputs(t)
	key, err := DerivePINKey("123456", salt, secret)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := session.WrapDeviceKey(key[:])
	if err != nil {
		t.Fatal(err)
	}
	witness := WitnessFor(mustHead(t, session))

	again, err := DerivePINKey("123456", salt, secret)
	if err != nil {
		t.Fatal(err)
	}
	opened, _, err := OpenWithDevice(container, again[:], envelope, &witness, nil)
	if err != nil {
		t.Fatalf("the same PIN did not open the vault: %v", err)
	}
	opened.Lock()

	wrong, err := DerivePINKey("123457", salt, secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenWithDevice(container, wrong[:], envelope, &witness, nil); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("a wrong PIN: got %v, want ErrAuthentication", err)
	}

	_, otherSecret := pinInputs(t)
	elsewhere, err := DerivePINKey("123456", salt, otherSecret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(elsewhere[:], key[:]) {
		t.Fatal("the device secret made no difference to the derived key")
	}
	if _, _, err := OpenWithDevice(container, elsewhere[:], envelope, &witness, nil); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("the same PIN on another device: got %v, want ErrAuthentication", err)
	}
}

func TestTheSaltMakesEveryPINKeyDifferent(t *testing.T) {
	_, secret := pinInputs(t)
	first, _ := NewPINSalt()
	second, _ := NewPINSalt()
	one, err := DerivePINKey("654321", first, secret)
	if err != nil {
		t.Fatal(err)
	}
	other, err := DerivePINKey("654321", second, secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(one[:], other[:]) {
		t.Fatal("two salts derived the same key")
	}
}

func TestOnlyADigitPINOfTheRightLengthIsAccepted(t *testing.T) {
	salt, secret := pinInputs(t)
	for name, pin := range map[string]string{
		"empty":        "",
		"too short":    "12345",
		"too long":     "1234567890123",
		"with letters": "12345a",
		"with spaces":  "123 456",
		"with a sign":  "+12345",
		"not ASCII":    "１２３４５６",
	} {
		if ValidPIN(pin) {
			t.Errorf("%s (%q) was accepted", name, pin)
		}
		if _, err := DerivePINKey(pin, salt, secret); !errors.Is(err, ErrInvalidPIN) {
			t.Errorf("%s (%q): got %v, want ErrInvalidPIN", name, pin, err)
		}
	}
	for _, pin := range []string{"000000", "123456", "123456789012"} {
		if !ValidPIN(pin) {
			t.Errorf("%q was refused", pin)
		}
	}
}

func TestDerivationRefusesInputsOfTheWrongSize(t *testing.T) {
	salt, secret := pinInputs(t)
	if _, err := DerivePINKey("123456", salt[:PINSaltSize-1], secret); !errors.Is(err, ErrInvalidPIN) {
		t.Errorf("a short salt: got %v, want ErrInvalidPIN", err)
	}
	if _, err := DerivePINKey("123456", salt, secret[:PINSecretSize-1]); !errors.Is(err, ErrInvalidPIN) {
		t.Errorf("a short device secret: got %v, want ErrInvalidPIN", err)
	}
}

func mustHead(t *testing.T, session *Session) Head {
	t.Helper()
	head, err := session.Head()
	if err != nil {
		t.Fatal(err)
	}
	return head
}
