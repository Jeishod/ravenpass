package linkproto

import (
	"bytes"
	"io"

	"github.com/flynn/noise"
)

// LinkInitiator starts the extension's side of Noise_XXpsk3 with the connection key's secret.
func LinkInitiator(static KeyPair, secret []byte, random io.Reader) (*Handshake, error) {
	return newHandshake(noise.Config{
		Pattern: noise.HandshakeXX, Initiator: true, StaticKeypair: static.noise(),
		PresharedKey: secret, PresharedKeyPlacement: pskPlacement, Random: random,
	})
}

// SessionInitiator starts the extension's side of Noise_IK towards the desktop app's static key.
func SessionInitiator(static KeyPair, desktop []byte, random io.Reader) (*Handshake, error) {
	if len(desktop) != KeySize {
		return nil, ErrMalformed
	}
	return newHandshake(noise.Config{
		Pattern: noise.HandshakeIK, Initiator: true, StaticKeypair: static.noise(),
		PeerStatic: bytes.Clone(desktop), Random: random,
	})
}
