package captures

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"testing/synctest"
	"time"
)

var heldID = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

func keep(t *testing.T, held *Held[string], sender, capture string, offered ...string) string {
	t.Helper()
	id, err := held.Keep(sender, capture, offered)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestACaptureIsHeldForThreeMinutes(t *testing.T) {
	if Lifetime != 3*time.Minute {
		t.Fatalf("captures live %v", Lifetime)
	}
}

func TestACaptureExpiresAfterItsLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		held := New[string](Lifetime)
		defer held.Close()
		id := keep(t, held, "sender", "typed")
		time.Sleep(Lifetime - time.Second)
		synctest.Wait()
		if _, found := held.Find("sender", id); !found {
			t.Fatal("the capture expired before its lifetime ended")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if _, found := held.Find("sender", id); found {
			t.Fatal("the capture outlived its lifetime")
		}
		if _, err := held.Claim("sender", id, ""); !errors.Is(err, ErrNotHeld) {
			t.Fatalf("a claim after the capture expired: got %v, want ErrNotHeld", err)
		}
	})
}

func TestASaveRunsOnceWhileItIsClaimed(t *testing.T) {
	held := New[string](Lifetime)
	defer held.Close()
	id := keep(t, held, "sender", "typed", "target")
	if _, err := held.Claim("sender", id, "target"); err != nil {
		t.Fatal(err)
	}
	if _, err := held.Claim("sender", id, ""); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("a second claim while saving: got %v, want ErrNotHeld", err)
	}
	held.Release("sender", id, false)
	capture, err := held.Claim("sender", id, "")
	if err != nil || capture != "typed" {
		t.Fatalf("a claim after a refused save = %q, error = %v", capture, err)
	}
	held.Release("sender", id, true)
	if _, found := held.Find("sender", id); found {
		t.Fatal("a saved capture is still held")
	}
}

func TestAClaimNeedsATargetTheLatestOfferNamed(t *testing.T) {
	held := New[string](Lifetime)
	defer held.Close()
	id := keep(t, held, "sender", "typed", "first")
	if _, err := held.Claim("sender", id, "second"); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("a target not offered: got %v, want ErrNotHeld", err)
	}
	held.Offer("sender", id, []string{"second"})
	if _, err := held.Claim("sender", id, "first"); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("a target the latest offer dropped: got %v, want ErrNotHeld", err)
	}
	if _, err := held.Claim("sender", id, "second"); err != nil {
		t.Fatalf("a target the latest offer named: %v", err)
	}
}

func TestAFifthCapturePushesOutTheOldest(t *testing.T) {
	held := New[string](Lifetime)
	defer held.Close()
	var ids []string
	for i := range maxHeld + 1 {
		ids = append(ids, keep(t, held, "sender", fmt.Sprintf("typed %d", i)))
	}
	if _, found := held.Find("sender", ids[0]); found {
		t.Fatal("the oldest capture is held after a fifth")
	}
	for i, id := range ids[1:] {
		if capture, found := held.Find("sender", id); !found || capture != fmt.Sprintf("typed %d", i+1) {
			t.Fatalf("capture %d = %q, %v", i+1, capture, found)
		}
	}
	if count := held.Count("sender"); count != maxHeld {
		t.Fatalf("%d captures held", count)
	}
}

func TestOneSenderCannotReachAnothersCapture(t *testing.T) {
	held := New[string](Lifetime)
	defer held.Close()
	id := keep(t, held, "owner", "typed")
	if _, found := held.Find("other", id); found {
		t.Fatal("another sender found the capture")
	}
	if _, err := held.Claim("other", id, ""); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("another sender's claim: got %v, want ErrNotHeld", err)
	}
	held.Forget("other", id)
	held.ForgetSender("other")
	if _, found := held.Find("owner", id); !found {
		t.Fatal("another sender's requests forgot the capture")
	}
}

func TestForgettingASenderDropsOnlyItsCaptures(t *testing.T) {
	held := New[string](Lifetime)
	defer held.Close()
	keep(t, held, "forgotten", "typed")
	id := keep(t, held, "kept", "typed")
	held.ForgetSender("forgotten")
	if count := held.Count("forgotten"); count != 0 {
		t.Fatalf("a forgotten sender holds %d captures", count)
	}
	if _, found := held.Find("kept", id); !found {
		t.Fatal("forgetting one sender forgot another's capture")
	}
}

func TestClosingDropsEveryCaptureAndKeepsNoMore(t *testing.T) {
	held := New[string](Lifetime)
	keep(t, held, "sender", "typed")
	held.Close()
	if count := held.Count("sender"); count != 0 {
		t.Fatalf("a closed holder holds %d captures", count)
	}
	if _, err := held.Keep("sender", "late", nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("a capture kept after closing: got %v, want ErrClosed", err)
	}
}

func TestIdentifiersAreRandomAndDistinct(t *testing.T) {
	held := New[string](Lifetime)
	defer held.Close()
	seen := make(map[string]bool)
	for range maxHeld {
		id := keep(t, held, "sender", "typed")
		if !heldID.MatchString(id) || seen[id] {
			t.Fatalf("identifier %q", id)
		}
		seen[id] = true
	}
}
