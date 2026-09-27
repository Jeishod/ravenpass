package ownerauth

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakePlatform struct {
	unavailable bool
	answer      error
	reasons     []string
	contexts    []context.Context
}

func (f *fakePlatform) Available() bool { return !f.unavailable }

func (f *fakePlatform) Authenticate(ctx context.Context, reason string) error {
	f.reasons = append(f.reasons, reason)
	f.contexts = append(f.contexts, ctx)
	return f.answer
}

func TestTheOwnerPromptSaysWhatItIsFor(t *testing.T) {
	platform := &fakePlatform{}
	owner := &Authenticator{platform: platform}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reason := "share “passport.pdf” from Alex with example.com"
	if err := owner.AuthenticateOwner(ctx, reason); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(platform.reasons, []string{reason}) {
		t.Fatalf("the prompts said %q", platform.reasons)
	}
	if platform.contexts[0] != ctx {
		t.Fatal("the owner prompt does not end with the caller's context")
	}
	for _, answer := range []error{ErrCanceled, ErrFailed, ErrUnavailable} {
		platform.answer = answer
		if err := owner.AuthenticateOwner(ctx, reason); !errors.Is(err, answer) {
			t.Fatalf("an owner answering %v: got %v", answer, err)
		}
	}
}

func TestAPromptWithoutAReasonIsNeverShown(t *testing.T) {
	platform := &fakePlatform{}
	owner := &Authenticator{platform: platform}
	if err := owner.AuthenticateOwner(context.Background(), ""); !errors.Is(err, ErrNoReason) {
		t.Fatalf("a prompt without a reason: got %v, want ErrNoReason", err)
	}
	if len(platform.reasons) != 0 {
		t.Fatal("a prompt without a reason was shown")
	}
}

func TestDeviceOwnerAvailabilityIsAskedWithoutPrompting(t *testing.T) {
	platform := &fakePlatform{}
	owner := &Authenticator{platform: platform}
	if !owner.DeviceOwnerAvailable() {
		t.Fatal("a device that can authenticate its owner reported otherwise")
	}
	platform.unavailable = true
	if owner.DeviceOwnerAvailable() {
		t.Fatal("a device that cannot authenticate its owner reported otherwise")
	}
	if len(platform.reasons) != 0 {
		t.Fatal("asking whether authentication works prompted for it")
	}
}
