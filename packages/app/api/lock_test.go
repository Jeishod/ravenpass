package api

import (
	"os"
	"testing"
	"testing/synctest"
	"time"
)

// copiedPassword copies a vault's one password and returns the clipboard and the copy's clearing.
func copiedPassword(t *testing.T) (*Service, *memoryPasteboard, func()) {
	t.Helper()
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clipboard := attachPasteboard(service)
	var discard func()
	if err := service.copyCredentialField(id, "password", func(_ time.Duration, clear func()) { discard = clear }); err != nil {
		t.Fatal(err)
	}
	return service, clipboard, discard
}

func TestLockingClearsTheCopy(t *testing.T) {
	service, clipboard, _ := copiedPassword(t)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if clipboard.text != "" {
		t.Fatal("locking left the copied password on the clipboard")
	}
}

func TestALockTheVaultRaisesOnItsOwnClearsWhatItLeftBehind(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, clipboard, _ := copiedPassword(t)
		if _, err := service.stagePhoto(photoFixture(t, "upright.webp")); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(service.vault.Storage().Current.Path); err != nil {
			t.Fatal(err)
		}
		if _, err := service.CreateCredential(CredentialInput{Label: "Bank", Password: "other"}, nil); err == nil {
			t.Fatal("a save to a vault file that went succeeded")
		}
		synctest.Wait()
		if service.vault.Unlocked() {
			t.Fatal("the failed save left the vault open")
		}
		if clipboard.text != "" {
			t.Fatal("the lock left the copied password on the clipboard")
		}
		_, err := service.CropIdentityPhoto(0, 0, 32)
		assertFailure(t, err, failureFileNotSelected)
	})
}

func TestLockingAwayLeavesTheCopyToItsClearingDelay(t *testing.T) {
	service, clipboard, discard := copiedPassword(t)
	if err := ControlsOf(service).LockAway(); err != nil {
		t.Fatal(err)
	}
	if service.vault.Unlocked() {
		t.Fatal("locking as the app left view kept the vault open")
	}
	if clipboard.text != "secret" {
		t.Fatal("locking as the app left view cleared the copy before it could be pasted")
	}
	discard()
	if clipboard.text != "" {
		t.Fatal("the copy outlived its clearing delay")
	}
}
