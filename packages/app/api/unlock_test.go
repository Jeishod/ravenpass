package api

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/verification"
	"github.com/dortanes/ravenpass/packages/vault"
)

func TestUnlockMethodsCarryWhatTheInterfaceNeeds(t *testing.T) {
	service := newReadyService(t)
	methods, err := service.GetUnlockMethods()
	if err != nil {
		t.Fatal(err)
	}
	if !methods.BiometryAvailable || !methods.BiometryEnabled || methods.PINSet {
		t.Fatalf("a new vault reports %+v", methods)
	}
	if methods.PINMinLength != vault.MinPINLength || methods.PINMaxLength != vault.MaxPINLength {
		t.Fatalf("the PIN rule reached the interface as %d to %d", methods.PINMinLength, methods.PINMaxLength)
	}
	if err := service.SetPIN(context.Background(), "135790", ""); err != nil {
		t.Fatal(err)
	}
	methods, err = service.GetUnlockMethods()
	if err != nil {
		t.Fatal(err)
	}
	if !methods.PINSet || methods.PINAttemptsLeft <= 0 {
		t.Fatalf("after setting a PIN: %+v", methods)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockWithPIN("135790"); err != nil {
		t.Fatalf("the PIN did not open the vault: %v", err)
	}
	if state, err := service.GetState(); err != nil || state.Phase != "ready" {
		t.Fatalf("state after a PIN unlock = %+v, error = %v", state, err)
	}
}

func TestAPINThatDoesNotFitIsRefusedByCode(t *testing.T) {
	service := newReadyService(t)
	owner := answerOwner(t, service, nil)
	if err := service.SetPIN(context.Background(), "12345", ""); err == nil || err.Error() != failurePrefix+string(failurePINInvalid) {
		t.Fatalf("a short PIN reported %v", err)
	}
	if owner.times() != 0 {
		t.Fatal("a PIN that does not fit asked the owner")
	}
	err := service.SetBiometryUnlock(context.Background(), false, "")
	if err == nil || err.Error() != failurePrefix+string(failureUnlockMethodRequired) {
		t.Fatalf("turning off the only way in reported %v", err)
	}
}

// newReadyServiceOnDevice is a ready service whose vault opens on device.
func newReadyServiceOnDevice(t *testing.T) (*Service, *stubDevice) {
	t.Helper()
	device := newStubDevice()
	service := newCreatedServiceOn(t, newStubKeys(), device, silentSites{}, filepath.Join(t.TempDir(), "icons"))
	return service, device
}

func TestDeviceAuthenticationAsksWithTheReasonInTheRecordedLanguage(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.Unlock(); err != nil {
		t.Fatalf("device authentication did not open the vault: %v", err)
	}
	if want := []string{service.preferences.Dialogs().UnlockVault}; !slices.Equal(device.platform.Prompts(), want) {
		t.Fatalf("the prompt said %q, want %q", device.platform.Prompts(), want)
	}
}

func TestAnUnverifiedOwnerLeavesTheVaultLockedWithItsCause(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	for answer, code := range map[error]failure{
		ownerauth.ErrCanceled:    failureAuthenticationCanceled,
		ownerauth.ErrFailed:      failureAuthenticationFailed,
		ownerauth.ErrUnavailable: failureUnlockUnavailable,
	} {
		if err := service.Lock(); err != nil {
			t.Fatal(err)
		}
		device.platform.Answer(answer)
		assertFailure(t, service.Unlock(), code)
		if state, err := service.GetState(); err != nil || state.Phase != "locked" {
			t.Fatalf("an owner answering %v left the state %+v, error = %v", answer, state, err)
		}
	}
}

func TestADeviceKeyTheDeviceNoLongerHoldsIsReported(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	device.platform.Reset()
	assertFailure(t, service.Unlock(), failureUnlockKeyMissing)
	if methods := unlockMethodsOf(t, service); methods.BiometryEnabled {
		t.Fatalf("a key the device no longer holds is still offered: %+v", methods)
	}
}

func TestAPINTheDeviceNoLongerHoldsIsNotOffered(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	answerOwner(t, service, nil)
	if err := service.SetPIN(context.Background(), "135790", ""); err != nil {
		t.Fatal(err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	device.pin.Reset()
	if methods := unlockMethodsOf(t, service); methods.PINSet || !methods.BiometryEnabled {
		t.Fatalf("a PIN the device no longer holds left %+v", methods)
	}
	assertFailure(t, service.UnlockWithPIN("135790"), failurePINMissing)
	if err := service.Unlock(); err != nil {
		t.Fatalf("device authentication did not open the vault: %v", err)
	}
}

func TestAPINThatCannotBeBoundIsNotSet(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	answerOwner(t, service, nil)
	device.pin.FailCreate(errors.New("the Secure Enclave refused"))
	assertFailure(t, service.SetPIN(context.Background(), "135790", ""), failureGeneral)
	if methods := unlockMethodsOf(t, service); methods.PINSet {
		t.Fatalf("a PIN that could not be bound left %+v", methods)
	}
}

// answerOwner makes the device's owner answer every later verification of service with answer.
func answerOwner(t *testing.T, service *Service, answer error) *answeringOwner {
	t.Helper()
	owner := &answeringOwner{answer: answer}
	service.owner = ownerVerifier(t, service.vault, owner, service.confirmations)
	return owner
}

func unlockMethodsOf(t *testing.T, service *Service) UnlockMethods {
	t.Helper()
	methods, err := service.GetUnlockMethods()
	if err != nil {
		t.Fatal(err)
	}
	return methods
}

func TestADeclinedOwnerLeavesTheUnlockMethodsUnchanged(t *testing.T) {
	for _, refusal := range []error{ownerauth.ErrCanceled, ownerauth.ErrFailed} {
		t.Run(refusal.Error(), func(t *testing.T) {
			service := newReadyService(t)
			answerOwner(t, service, refusal)
			assertFailure(t, service.SetPIN(context.Background(), "135790", ""), failureOwnerUnverified)
			if methods := unlockMethodsOf(t, service); methods.PINSet || !methods.BiometryEnabled {
				t.Fatalf("a declined PIN left %+v", methods)
			}

			answerOwner(t, service, nil)
			if err := service.SetPIN(context.Background(), "135790", ""); err != nil {
				t.Fatal(err)
			}
			answerOwner(t, service, refusal)
			assertFailure(t, service.SetPIN(context.Background(), "246801", "135790"), failureOwnerUnverified)
			assertFailure(t, service.RemovePIN(context.Background(), "135790"), failureOwnerUnverified)
			assertFailure(t, service.SetBiometryUnlock(context.Background(), false, "135790"), failureOwnerUnverified)
			if methods := unlockMethodsOf(t, service); !methods.PINSet || !methods.BiometryEnabled {
				t.Fatalf("declined changes left %+v", methods)
			}
			if err := service.Lock(); err != nil {
				t.Fatal(err)
			}
			if err := service.UnlockWithPIN("135790"); err != nil {
				t.Fatalf("a declined change replaced the PIN: %v", err)
			}
		})
	}
}

func TestAnApprovedOwnerChangesTheUnlockMethods(t *testing.T) {
	service := newReadyService(t)
	owner := answerOwner(t, service, nil)
	if err := service.SetPIN(context.Background(), "135790", ""); err != nil {
		t.Fatal(err)
	}
	if err := service.RemovePIN(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if err := service.SetPIN(context.Background(), "135790", ""); err != nil {
		t.Fatal(err)
	}
	if err := service.SetBiometryUnlock(context.Background(), false, ""); err != nil {
		t.Fatal(err)
	}
	if owner.times() != 4 {
		t.Fatalf("the owner was asked %d times for four changes", owner.times())
	}
	if methods := unlockMethodsOf(t, service); !methods.PINSet || methods.BiometryEnabled {
		t.Fatalf("approved changes left %+v", methods)
	}
}

func TestTurningDeviceAuthenticationOnTakesTheCurrentPIN(t *testing.T) {
	service, _ := pinVerifications(t)
	owner := answerOwner(t, service, nil)
	before := unlockMethodsOf(t, service).PINAttemptsLeft

	assertFailure(t, service.SetBiometryUnlock(context.Background(), true, ""), failurePINInvalid)
	assertFailure(t, service.SetBiometryUnlock(context.Background(), true, "999999"), failurePINWrong)
	methods := unlockMethodsOf(t, service)
	if methods.BiometryEnabled || methods.PINAttemptsLeft != before-1 {
		t.Fatalf("a wrong PIN left %+v, %d attempts before it", methods, before)
	}

	if err := service.SetBiometryUnlock(context.Background(), true, confirmationPIN); err != nil {
		t.Fatal(err)
	}
	methods = unlockMethodsOf(t, service)
	if !methods.BiometryEnabled || methods.PINAttemptsLeft != before {
		t.Fatalf("the right PIN left %+v, %d attempts before the wrong one", methods, before)
	}
	if owner.times() != 0 {
		t.Fatal("turning device authentication on showed the system prompt")
	}
	assertNoConfirmation(t, service)
}

func TestTheCurrentPINVerifiesWhereDeviceAuthenticationIsOff(t *testing.T) {
	service, _ := pinVerifications(t)
	owner := answerOwner(t, service, nil)
	before := unlockMethodsOf(t, service).PINAttemptsLeft

	assertFailure(t, service.SetPIN(context.Background(), "135790", ""), failurePINInvalid)
	assertFailure(t, service.RemovePIN(context.Background(), ""), failurePINInvalid)
	assertFailure(t, service.SetPIN(context.Background(), "135790", "999999"), failurePINWrong)
	assertFailure(t, service.RemovePIN(context.Background(), "999999"), failurePINWrong)
	if methods := unlockMethodsOf(t, service); !methods.PINSet || methods.BiometryEnabled || methods.PINAttemptsLeft != before-2 {
		t.Fatalf("refused changes left %+v, %d attempts before them", methods, before)
	}

	if err := service.SetPIN(context.Background(), "135790", confirmationPIN); err != nil {
		t.Fatalf("a verified PIN change: %v", err)
	}
	if owner.times() != 0 {
		t.Fatal("a vault without device authentication showed the system prompt")
	}
	assertNoConfirmation(t, service)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	assertFailure(t, service.UnlockWithPIN(confirmationPIN), failurePINWrong)
	if err := service.UnlockWithPIN("135790"); err != nil {
		t.Fatalf("the new PIN did not open the vault: %v", err)
	}
}

func TestDeviceAuthenticationVerifiesWhateverPINIsGiven(t *testing.T) {
	service, _ := pinVerifications(t)
	if err := service.SetBiometryUnlock(context.Background(), true, confirmationPIN); err != nil {
		t.Fatal(err)
	}
	declined := answerOwner(t, service, ownerauth.ErrCanceled)
	assertFailure(t, service.RemovePIN(context.Background(), confirmationPIN), failureOwnerUnverified)
	approved := answerOwner(t, service, nil)
	if err := service.SetPIN(context.Background(), "135790", "999999"); err != nil {
		t.Fatalf("device authentication did not verify the change: %v", err)
	}
	if declined.times() != 1 || approved.times() != 1 {
		t.Fatalf("the owner was asked %d and %d times", declined.times(), approved.times())
	}
	assertNoConfirmation(t, service)
}

func TestAVaultWithNeitherMethodChangesWithoutAsking(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	device.noDeviceOwner = true
	owner := answerOwner(t, service, ownerauth.ErrCanceled)
	if err := service.SetPIN(context.Background(), "135790", ""); err != nil {
		t.Fatalf("a vault opened with its recovery phrase refused a PIN: %v", err)
	}
	if owner.times() != 0 || !unlockMethodsOf(t, service).PINSet {
		t.Fatalf("setting the first PIN asked the owner %d times", owner.times())
	}
	assertNoConfirmation(t, service)
}

func TestAPromptThatCannotRunLeavesTheUnlockMethodsUnchanged(t *testing.T) {
	service := newReadyService(t)
	service.owner = unverifiableOwner{}
	assertFailure(t, service.SetPIN(context.Background(), "135790", ""), failureOwnerUnverified)
	assertFailure(t, service.SetBiometryUnlock(context.Background(), false, ""), failureOwnerUnverified)
	if methods := unlockMethodsOf(t, service); methods.PINSet || !methods.BiometryEnabled {
		t.Fatalf("an unverified owner left %+v", methods)
	}
}

func TestTurningOffDeviceAuthenticationWhileItIsOffDoesNotAsk(t *testing.T) {
	service, _ := pinVerifications(t)
	owner := answerOwner(t, service, ownerauth.ErrCanceled)
	before := unlockMethodsOf(t, service).PINAttemptsLeft
	if err := service.SetBiometryUnlock(context.Background(), false, ""); err != nil {
		t.Fatalf("turning off what is already off: %v", err)
	}
	assertNoConfirmation(t, service)
	if owner.times() != 0 || unlockMethodsOf(t, service).PINAttemptsLeft != before {
		t.Fatal("turning off what is already off asked the owner")
	}
}

func TestAChangeRightAfterCreationAsksTheOwner(t *testing.T) {
	service := newCreatedService(t, newStubKeys(), silentSites{}, filepath.Join(t.TempDir(), "icons"))
	owner := answerOwner(t, service, ownerauth.ErrCanceled)
	assertFailure(t, service.SetPIN(context.Background(), "135790", ""), failureOwnerUnverified)
	if owner.times() != 1 {
		t.Fatalf("the owner was asked %d times", owner.times())
	}
	if unlockMethodsOf(t, service).PINSet {
		t.Fatal("a refused owner set a PIN")
	}
}

func TestCreationRefusesAVaultWithoutAWayIn(t *testing.T) {
	home := t.TempDir()
	service, files := newServiceWithStorage(t, filepath.Join(home, "storage.json"), filepath.Join(home, "vault.rpv"))
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	phrase, err := service.BeginCreation()
	if err != nil {
		t.Fatal(err)
	}
	assertFailure(t, service.ConfirmCreation(phrase, UnlockChoice{}), failureUnlockMethodRequired)
	if state, err := service.GetState(); err != nil || state.Phase != "setup" {
		t.Fatalf("state after a refused creation = %+v, error = %v", state, err)
	}
}

// unverifiableOwner is a device whose authentication proves unavailable once asked for.
type unverifiableOwner struct{}

func (unverifiableOwner) Verify(context.Context, confirmation.Reason, func(verification.Method)) error {
	return verification.ErrUnverifiable
}

func (unverifiableOwner) VerifyDevice(context.Context, confirmation.Reason) error {
	return verification.ErrUnverifiable
}

func TestAnEndedCallLeavesTheUnlockMethodsUnchanged(t *testing.T) {
	service := newReadyService(t)
	service.owner = ownerVerifier(t, service.vault, silentOwner{}, service.confirmations)
	ctx, cancel := context.WithCancel(context.Background())
	ended := make(chan error, 1)
	go func() { ended <- service.SetPIN(ctx, "135790", "") }()
	cancel()
	if err := verificationEnded(t, ended); err == nil {
		t.Fatal("an ended call reported success")
	}
	assertNoConfirmation(t, service)
	if unlockMethodsOf(t, service).PINSet {
		t.Fatal("an ended call set a PIN")
	}
}
