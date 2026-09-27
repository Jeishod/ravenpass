package linkkey

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"
)

// rawKey builds key bytes field by field, with a checksum over the fields.
func rawKey(port uint16, secret [SecretSize]byte) []byte {
	raw := make([]byte, keyBytes)
	copy(raw, header[:])
	raw[versionOffset] = version
	binary.BigEndian.PutUint16(raw[portOffset:], port)
	copy(raw[secretOffset:], secret[:])
	sum := sha256.Sum256(raw[:checksumOffset])
	copy(raw[checksumOffset:], sum[:])
	return raw
}

func testSecret(fill byte) [SecretSize]byte {
	var secret [SecretSize]byte
	for i := range secret {
		secret[i] = fill + byte(i)
	}
	return secret
}

func TestEncodingWritesTheDocumentedLayout(t *testing.T) {
	for _, port := range []uint16{MinPort, 49152, 53117, 65535} {
		secret := testSecret(byte(port))
		text, err := Key{Port: port, Secret: secret}.Encode()
		if err != nil {
			t.Fatalf("port %d: %v", port, err)
		}
		raw, err := encoding.DecodeString(text)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, rawKey(port, secret)) {
			t.Fatalf("port %d: key bytes = %x", port, raw)
		}
	}
}

func TestEncodingRefusesAPortBelowTheMinimum(t *testing.T) {
	if _, err := (Key{Port: MinPort - 1}).Encode(); !errors.Is(err, ErrPort) {
		t.Fatalf("port %d: got %v, want ErrPort", MinPort-1, err)
	}
}
