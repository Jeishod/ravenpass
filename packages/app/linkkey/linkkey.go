// Package linkkey encodes the one-time connection key an extension links with.
package linkkey

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
)

const (
	// SecretSize is the length of the one-time secret, the pre-shared key of the link handshake.
	SecretSize = 32
	// MinPort is the lowest port a key may carry.
	MinPort = 1024

	// Layout: header[3] version[1] port[2, big-endian] secret[32] checksum[4, SHA-256 prefix].
	keyBytes       = 42
	version        = 1
	versionOffset  = 3
	portOffset     = 4
	secretOffset   = 6
	checksumOffset = 38
)

// header lets the extension recognise a key on the clipboard without reading anything else as one.
var header = [3]byte{0x9a, 0x5c, 0x13}

var encoding = base64.RawURLEncoding

// ErrPort is returned by Encode for a port below MinPort.
var ErrPort = errors.New("connection key port is out of range")

// Key is what a connection key carries.
type Key struct {
	Port   uint16
	Secret [SecretSize]byte
}

// Encode writes the key as text.
func (k Key) Encode() (string, error) {
	if k.Port < MinPort {
		return "", ErrPort
	}
	var raw [keyBytes]byte
	copy(raw[:], header[:])
	raw[versionOffset] = version
	binary.BigEndian.PutUint16(raw[portOffset:], k.Port)
	copy(raw[secretOffset:], k.Secret[:])
	sum := sha256.Sum256(raw[:checksumOffset])
	copy(raw[checksumOffset:], sum[:keyBytes-checksumOffset])
	return encoding.EncodeToString(raw[:]), nil
}
