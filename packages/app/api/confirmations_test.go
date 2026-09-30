package api

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/app/verification"
)

const confirmationPIN = "246801"

// hangLimit bounds waits that end at once when the code works; one Argon2id PIN derivation under -race takes tens of seconds on CI runners.
const hangLimit = 2 * time.Minute

var sharedPassport = confirmation.Sharing("passport.pdf", "Alex", "example.com")

// pinVerifications is a ready service verified by PIN alone and a verifier that queues PIN requests.
func pinVerifications(t *testing.T) (*Service, *verification.Verifier) {
	t.Helper()
	service := newReadyService(t)
	if err := service.SetPIN(context.Background(), confirmationPIN, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.SetBiometryUnlock(context.Background(), false, ""); err != nil {
		t.Fatal(err)
	}
	verifier, err := verification.New(service.vault, silentOwner{}, func(confirmation.Reason) string { return "share" }, service.confirmations)
	if err != nil {
		t.Fatal(err)
	}
	return service, verifier
}

func verifyReason(verifier *verification.Verifier, reason confirmation.Reason) <-chan error {
	ended := make(chan error, 1)
	go func() { ended <- verifier.Verify(context.Background(), reason, func(verification.Method) {}) }()
	return ended
}

func awaitConfirmation(t *testing.T, service *Service) Confirmation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), hangLimit)
	defer cancel()
	request, err := service.AwaitConfirmation(ctx, "")
	if err != nil {
		t.Fatalf("no confirmation arrived: %v", err)
	}
	return request
}

func verificationEnded(t *testing.T, ended <-chan error) error {
	t.Helper()
	select {
	case err := <-ended:
		return err
	case <-time.After(hangLimit):
		t.Fatal("the verification kept waiting")
		return nil
	}
}

func assertNoConfirmation(t *testing.T, service *Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if waiting, err := service.AwaitConfirmation(ctx, ""); err == nil {
		t.Fatalf("a confirmation still waits: %+v", waiting)
	}
}

func windowsOf(service *Service) *fakeWindows {
	return service.panel.(*fakeWindows)
}

func TestAVerificationIsConfirmedWithThePIN(t *testing.T) {
	service, verifier := pinVerifications(t)
	ended := verifyReason(verifier, sharedPassport)
	request := awaitConfirmation(t, service)
	want := Confirmation{ID: request.ID, Kind: "verify", Reason: "share", File: "passport.pdf", Identity: "Alex", Site: "example.com"}
	if request != want || request.ID == "" {
		t.Fatalf("confirmation = %+v", request)
	}
	assertFailure(t, service.ConfirmWithPIN(request.ID, "999999"), failurePINWrong)
	methods, err := service.GetUnlockMethods()
	if err != nil || methods.PINAttemptsLeft != unlock.MaxPINFailures-1 {
		t.Fatalf("attempts after a wrong PIN = %+v, error = %v", methods, err)
	}
	assertFailure(t, service.ConfirmWithPIN(request.ID, "12"), failurePINInvalid)
	if again := awaitConfirmation(t, service); again != request {
		t.Fatalf("after a wrong PIN the waiting confirmation is %+v", again)
	}
	if err := service.ConfirmWithPIN(request.ID, confirmationPIN); err != nil {
		t.Fatal(err)
	}
	if err := verificationEnded(t, ended); err != nil {
		t.Fatalf("a confirmed verification: %v", err)
	}
	if methods, err := service.GetUnlockMethods(); err != nil || methods.PINAttemptsLeft != unlock.MaxPINFailures {
		t.Fatalf("the right PIN did not clear the count: %+v, error = %v", methods, err)
	}
	assertFailure(t, service.ConfirmWithPIN(request.ID, confirmationPIN), failureConfirmationEnded)
	assertNoConfirmation(t, service)
}

func TestPasskeyVerificationsNameTheirReasonSiteAndAccount(t *testing.T) {
	service, verifier := pinVerifications(t)
	for reason, want := range map[confirmation.Reason]Confirmation{
		confirmation.SavingPasskey("example.com"):     {Kind: "verify", Reason: "save-passkey", Site: "example.com"},
		confirmation.SigningIn("example.com", "alex"): {Kind: "verify", Reason: "sign-in", Site: "example.com", Account: "alex"},
		confirmation.Filling("example.com", "alex"):   {Kind: "verify", Reason: "fill", Site: "example.com", Account: "alex"},
	} {
		ended := verifyReason(verifier, reason)
		request := awaitConfirmation(t, service)
		want.ID = request.ID
		if request != want || request.ID == "" {
			t.Fatalf("confirmation = %+v, want %+v", request, want)
		}
		if err := service.ConfirmWithPIN(request.ID, confirmationPIN); err != nil {
			t.Fatal(err)
		}
		if err := verificationEnded(t, ended); err != nil {
			t.Fatalf("a confirmed %s: %v", want.Reason, err)
		}
	}
}

func TestAVerificationCanBeDeclined(t *testing.T) {
	service, verifier := pinVerifications(t)
	ended := verifyReason(verifier, sharedPassport)
	request := awaitConfirmation(t, service)
	assertFailure(t, service.UnlockFromConfirmation(request.ID), failureConfirmationEnded)
	assertFailure(t, service.RecoverFromConfirmation(request.ID), failureConfirmationEnded)
	if err := service.DeclineConfirmation(request.ID); err != nil {
		t.Fatal(err)
	}
	if err := verificationEnded(t, ended); !errors.Is(err, verification.ErrDeclined) {
		t.Fatalf("a declined verification: got %v, want ErrDeclined", err)
	}
	assertFailure(t, service.DeclineConfirmation(request.ID), failureConfirmationEnded)
	if windowsOf(service).shown != 0 {
		t.Fatal("a verification brought the main window forward")
	}
}

func TestLockingEndsEveryConfirmation(t *testing.T) {
	service, verifier := pinVerifications(t)
	ended := verifyReason(verifier, sharedPassport)
	request := awaitConfirmation(t, service)
	unlocking := service.confirmations.PostUnlock(confirmation.RequesterExtension)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := verificationEnded(t, ended); !errors.Is(err, vaultservice.ErrNotReady) {
		t.Fatalf("a verification when the vault locked: got %v, want ErrNotReady", err)
	}
	assertFailure(t, service.ConfirmWithPIN(request.ID, confirmationPIN), failureConfirmationEnded)
	assertFailure(t, service.ConfirmWithPIN(unlocking, confirmationPIN), failureConfirmationEnded)
	assertNoConfirmation(t, service)
}

func TestAwaitConfirmationEndsWithItsCall(t *testing.T) {
	service := newReadyService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.AwaitConfirmation(ctx, ""); err == nil {
		t.Fatal("a canceled wait returned a confirmation")
	}
}

func TestThePageLearnsWhenItsRequestLeaves(t *testing.T) {
	service, verifier := pinVerifications(t)
	ended := verifyReason(verifier, sharedPassport)
	request := awaitConfirmation(t, service)
	left := make(chan Confirmation, 1)
	go func() {
		next, err := service.AwaitConfirmation(context.Background(), request.ID)
		if err == nil {
			left <- next
		}
	}()
	if err := service.DeclineConfirmation(request.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case next := <-left:
		if next != (Confirmation{}) {
			t.Fatalf("once nothing waits the page is shown %+v", next)
		}
	case <-time.After(hangLimit):
		t.Fatal("the page did not learn its request left")
	}
	if err := verificationEnded(t, ended); !errors.Is(err, verification.ErrDeclined) {
		t.Fatalf("a declined verification: got %v, want ErrDeclined", err)
	}
}

// lockedOnDevice is a locked service with device authentication and a PIN.
func lockedOnDevice(t *testing.T) (*Service, *stubDevice) {
	t.Helper()
	service, device := newReadyServiceOnDevice(t)
	if err := service.SetPIN(context.Background(), confirmationPIN, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	return service, device
}

// lockedWithPIN is a locked service with device authentication and a PIN, and an unlock request the panel shows once
// the owner canceled the device's prompt; the device allows later prompts.
func lockedWithPIN(t *testing.T) (*Service, Confirmation) {
	t.Helper()
	service, device := lockedOnDevice(t)
	device.platform.Answer(ownerauth.ErrCanceled)
	service.confirmations.PostUnlock(confirmation.RequesterAutofill)
	request := awaitConfirmation(t, service)
	if request != (Confirmation{ID: request.ID, Kind: "unlock", Requester: "autofill"}) || request.ID == "" {
		t.Fatalf("unlock confirmation = %+v", request)
	}
	if prompts := device.platform.Prompts(); len(prompts) != 1 {
		t.Fatalf("the device prompted %d times before the card showed, want once", len(prompts))
	}
	device.platform.Answer(nil)
	return service, request
}

func TestAnUnlockRequestTriesDeviceAuthenticationFirst(t *testing.T) {
	service, device := lockedOnDevice(t)
	unlocking := service.confirmations.PostUnlock(confirmation.RequesterExtension)
	ctx, cancel := context.WithTimeout(context.Background(), hangLimit)
	defer cancel()
	if err := service.confirmations.AwaitEnd(ctx, unlocking); err != nil {
		t.Fatalf("the unlock request outlived the device's prompt: %v", err)
	}
	assertReady(t, service)
	assertNoConfirmation(t, service)
	if want := []string{service.preferences.Dialogs().UnlockVault}; !slices.Equal(device.platform.Prompts(), want) {
		t.Fatalf("the device prompted %q, want %q", device.platform.Prompts(), want)
	}
	// The main window reloads after the request ends, on the goroutine that prompted.
	windows := windowsOf(service)
	deadline := time.Now().Add(hangLimit)
	for {
		windows.mu.Lock()
		reloaded, shown := windows.reloaded, windows.shown
		windows.mu.Unlock()
		if reloaded == 1 && shown == 0 {
			return
		}
		if reloaded > 1 || shown != 0 || time.Now().After(deadline) {
			t.Fatalf("the main window was reloaded %d and shown %d times, want reloaded once", reloaded, shown)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWithoutDeviceAuthenticationTheUnlockCardShowsAtOnce(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	if err := service.SetPIN(context.Background(), confirmationPIN, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.SetBiometryUnlock(context.Background(), false, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	prompted := len(device.platform.Prompts())
	unlocking := service.confirmations.PostUnlock(confirmation.RequesterExtension)
	if request := awaitConfirmation(t, service); request.ID != unlocking || request.Kind != "unlock" {
		t.Fatalf("unlock confirmation = %+v", request)
	}
	if len(device.platform.Prompts()) != prompted {
		t.Fatal("a vault without device authentication prompted the device")
	}
}

func assertReady(t *testing.T, service *Service) {
	t.Helper()
	if state, err := service.GetState(); err != nil || state.Phase != "ready" {
		t.Fatalf("state = %+v, error = %v", state, err)
	}
}

func TestAnUnlockRequestOpensTheVaultWithThePIN(t *testing.T) {
	service, request := lockedWithPIN(t)
	assertFailure(t, service.ConfirmWithPIN(request.ID, "999999"), failurePINWrong)
	if again := awaitConfirmation(t, service); again != request {
		t.Fatalf("after a wrong PIN the waiting confirmation is %+v", again)
	}
	if windowsOf(service).reloaded != 0 {
		t.Fatal("a wrong PIN reloaded the main window")
	}
	if err := service.ConfirmWithPIN(request.ID, confirmationPIN); err != nil {
		t.Fatal(err)
	}
	assertReady(t, service)
	assertNoConfirmation(t, service)
	assertFailure(t, service.ConfirmWithPIN(request.ID, confirmationPIN), failureConfirmationEnded)
	if windowsOf(service).shown != 0 {
		t.Fatal("unlocking from the panel brought the main window forward")
	}
	if windowsOf(service).reloaded != 1 {
		t.Fatalf("the main window was reloaded %d times, want once to leave its locked screen", windowsOf(service).reloaded)
	}
}

func TestAnUnlockRequestOpensTheVaultWithDeviceAuthentication(t *testing.T) {
	service, request := lockedWithPIN(t)
	if err := service.UnlockFromConfirmation(request.ID); err != nil {
		t.Fatal(err)
	}
	assertReady(t, service)
	assertNoConfirmation(t, service)
	assertFailure(t, service.UnlockFromConfirmation(request.ID), failureConfirmationEnded)
	if windowsOf(service).reloaded != 1 {
		t.Fatalf("the main window was reloaded %d times, want once to leave its locked screen", windowsOf(service).reloaded)
	}
}

func TestUnlockingInTheMainWindowEndsTheUnlockRequest(t *testing.T) {
	service, request := lockedWithPIN(t)
	if err := service.Unlock(); err != nil {
		t.Fatal(err)
	}
	assertNoConfirmation(t, service)
	assertFailure(t, service.DeclineConfirmation(request.ID), failureConfirmationEnded)
	if windowsOf(service).reloaded != 0 {
		t.Fatal("unlocking in the main window reloaded it")
	}
}

func TestRecoveryFromAnUnlockRequestShowsTheMainWindow(t *testing.T) {
	service, request := lockedWithPIN(t)
	if err := service.RecoverFromConfirmation(request.ID); err != nil {
		t.Fatal(err)
	}
	if windowsOf(service).shown != 1 {
		t.Fatalf("the main window was shown %d times", windowsOf(service).shown)
	}
	assertNoConfirmation(t, service)
	if state, err := service.GetState(); err != nil || state.Phase != "locked" {
		t.Fatalf("state after choosing recovery = %+v, error = %v", state, err)
	}
	assertFailure(t, service.RecoverFromConfirmation(request.ID), failureConfirmationEnded)
}

func TestAnUnlockRequestCanBeDeclined(t *testing.T) {
	service, request := lockedWithPIN(t)
	if err := service.DeclineConfirmation(request.ID); err != nil {
		t.Fatal(err)
	}
	assertNoConfirmation(t, service)
	if state, err := service.GetState(); err != nil || state.Phase != "locked" {
		t.Fatalf("state after declining = %+v, error = %v", state, err)
	}
}

func TestThePageSetsThePanelsHeight(t *testing.T) {
	service := newReadyService(t)
	service.FitConfirmation(212)
	service.FitConfirmation(248)
	if heights := windowsOf(service).heights; !slices.Equal(heights, []int{212, 248}) {
		t.Fatalf("the panel was fitted to %v", heights)
	}
}

func TestAWindowLearnsOfALanguageAnotherChose(t *testing.T) {
	service := newReadyService(t)
	changed := make(chan uint64, 1)
	go func() {
		count, err := service.AwaitLanguageChange(context.Background(), 0)
		if err == nil {
			changed <- count
		}
	}()
	if err := service.SetLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	select {
	case count := <-changed:
		if count != 1 {
			t.Fatalf("languages chosen = %d, want 1", count)
		}
	case <-time.After(hangLimit):
		t.Fatal("a waiting window did not learn of the language")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.AwaitLanguageChange(ctx, 1); err == nil {
		t.Fatal("a canceled wait returned a count")
	}
}
