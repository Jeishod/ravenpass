package unlock

import (
	"errors"
	"testing"
	"time"
)

const throttledVault = "0123456789abcdef0123456789abcdef"

func TestThrottleRefusesAttemptsUntilTheEarnedDelayPasses(t *testing.T) {
	now := time.Unix(1_000, 0)
	throttle := NewThrottle(func() time.Time { return now })
	throttle.Record(throttledVault, 2)
	if err := throttle.Allow(throttledVault); err != nil {
		t.Fatalf("an attempt after two wrong PINs: %v", err)
	}
	throttle.Record(throttledVault, 5)
	delay := AttemptDelay(5)
	now = now.Add(delay - time.Nanosecond)
	if err := throttle.Allow(throttledVault); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("an attempt before the delay passed: got %v, want ErrTooSoon", err)
	}
	if err := throttle.Allow("fedcba9876543210fedcba9876543210"); err != nil {
		t.Fatalf("another vault was held back: %v", err)
	}
	now = now.Add(time.Nanosecond)
	if err := throttle.Allow(throttledVault); err != nil {
		t.Fatalf("an attempt once the delay passed: %v", err)
	}
}

func TestThrottleClearsAtZeroFailures(t *testing.T) {
	now := time.Unix(1_000, 0)
	throttle := NewThrottle(func() time.Time { return now })
	throttle.Record(throttledVault, MaxPINFailures-1)
	throttle.Record(throttledVault, 0)
	if err := throttle.Allow(throttledVault); err != nil {
		t.Fatalf("a cleared throttle refused: %v", err)
	}
}
