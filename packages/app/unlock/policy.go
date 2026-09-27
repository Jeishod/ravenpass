// Package unlock owns the device's record of the ways a vault opens on it and the rules for them.
package unlock

import (
	"encoding/json"
	"errors"
	"time"
)

// MaxPINFailures is how many wrong PINs in a row remove the PIN.
const MaxPINFailures = 10

const recordVersion = 2

var (
	// ErrNoMethodLeft refuses a change that leaves no way into the vault.
	ErrNoMethodLeft = errors.New("a way to unlock must remain")
	// ErrNoPIN reports a vault without a PIN on this device.
	ErrNoPIN = errors.New("no PIN is set for this vault")
	// ErrPINRemoved reports the wrong PIN that removed the PIN.
	ErrPINRemoved = errors.New("the PIN was removed after too many wrong attempts")
	// ErrMalformed reports an unlock record that cannot be read.
	ErrMalformed = errors.New("unlock record is malformed")
	// ErrUnsupported reports an unlock record of another version.
	ErrUnsupported = errors.New("unlock record has an unsupported version")
	// ErrWrongPIN reports a PIN that does not match.
	ErrWrongPIN = errors.New("the PIN does not match")
	// ErrTooSoon refuses a PIN attempt made before the delay earned by wrong ones has passed.
	ErrTooSoon = errors.New("too soon for another PIN attempt")
	// ErrNotAvailable reports a device that cannot authenticate its owner.
	ErrNotAvailable = errors.New("the device cannot authenticate its owner")
	// ErrDisabled reports a way of unlocking turned off for the vault.
	ErrDisabled = errors.New("this way of unlocking is turned off for this vault")
)

// AttemptDelay is how long a further PIN attempt waits after failures wrong ones in a row.
func AttemptDelay(failures int) time.Duration {
	const free, longest = 3, 10
	if failures < free {
		return 0
	}
	seconds := 1 << min(failures-free, 4)
	return time.Duration(min(seconds, longest)) * time.Second
}

// Policy is one vault's ways in on this device; the zero value opens only with the recovery key.
type Policy struct {
	Version  int                 `json:"version"`
	Platform *PlatformCredential `json:"platform,omitempty"`
	PIN      *PIN                `json:"pin,omitempty"`
}

// HasPlatform reports whether the device's own authentication opens the vault.
func (p Policy) HasPlatform() bool { return p.Platform != nil }

// HasPIN reports whether a PIN opens the vault.
func (p Policy) HasPIN() bool { return p.PIN != nil }

// Usable reports a policy that still opens the vault on this device.
func (p Policy) Usable(ownerAvailable bool) bool {
	return p.HasPIN() || p.HasPlatform() && ownerAvailable
}

// Encode writes the policy as the current record version.
func (p Policy) Encode() ([]byte, error) {
	p.Version = recordVersion
	return json.Marshal(p)
}

// Decode reads a stored record; an unknown or malformed one is reported, never replaced, keeping its wrapped keys.
func Decode(data []byte) (Policy, error) {
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Policy{}, ErrMalformed
	}
	if header.Version != recordVersion {
		return Policy{}, ErrUnsupported
	}
	var stored Policy
	if err := json.Unmarshal(data, &stored); err != nil {
		return Policy{}, ErrMalformed
	}
	if stored.Platform != nil && !stored.Platform.complete() || stored.PIN != nil && !stored.PIN.complete() {
		return Policy{}, ErrMalformed
	}
	return stored, nil
}
