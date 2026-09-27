package unlocktest

import (
	"crypto/hmac"
	"crypto/sha256"

	"github.com/dortanes/ravenpass/packages/vault"
)

// PINKey refuses the inputs vault.DerivePINKey refuses and derives with HMAC-SHA256 instead of 256 MiB of Argon2id,
// which under the race detector exceeds a hosted runner's memory.
func PINKey(pin string, salt, deviceSecret []byte) ([32]byte, error) {
	var key [32]byte
	if !vault.ValidPIN(pin) || len(salt) != vault.PINSaltSize || len(deviceSecret) != vault.PINSecretSize {
		return key, vault.ErrInvalidPIN
	}
	mac := hmac.New(sha256.New, append(append([]byte{}, salt...), deviceSecret...))
	mac.Write([]byte(pin))
	copy(key[:], mac.Sum(nil))
	return key, nil
}
