package api

import (
	"context"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/ownerauth"
)

func TestANewRecoveryPhraseNeedsTheOwnerAndItsWords(t *testing.T) {
	service := newReadyService(t)
	owner := answerOwner(t, service, ownerauth.ErrCanceled)
	if _, err := service.BeginRecoveryPhraseChange(context.Background(), "", ""); err == nil {
		t.Fatal("a declined owner received a new phrase")
	} else {
		assertFailure(t, err, failureOwnerUnverified)
	}
	answerOwner(t, service, nil)
	phrase, err := service.BeginRecoveryPhraseChange(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if owner.times() != 1 {
		t.Fatalf("the declined owner was asked %d times", owner.times())
	}
	assertFailure(t, service.ConfirmRecoveryPhraseChange("other words"), failureRecoveryKeyMismatch)
	if err := service.ConfirmRecoveryPhraseChange(phrase); err != nil {
		t.Fatal(err)
	}
	assertFailure(t, service.ConfirmRecoveryPhraseChange(phrase), failureSetupNotActive)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.Unlock(); err != nil {
		t.Fatalf("device authentication after a new phrase: %v", err)
	}
}

func TestTheCurrentPINVerifiesANewPhraseWhereDeviceAuthenticationIsOff(t *testing.T) {
	service, _ := pinVerifications(t)
	owner := answerOwner(t, service, nil)
	before := unlockMethodsOf(t, service).PINAttemptsLeft
	assertFailure(t, beginPhraseChange(service, ""), failurePINInvalid)
	assertFailure(t, beginPhraseChange(service, "999999"), failurePINWrong)
	if left := unlockMethodsOf(t, service).PINAttemptsLeft; left != before-1 {
		t.Fatalf("one wrong PIN left %d attempts of %d", left, before)
	}
	phrase, err := service.BeginRecoveryPhraseChange(context.Background(), confirmationPIN, "")
	if err != nil {
		t.Fatal(err)
	}
	if owner.times() != 0 {
		t.Fatal("the PIN verified the owner, yet the system prompt was shown")
	}
	if left := unlockMethodsOf(t, service).PINAttemptsLeft; left != before {
		t.Fatalf("the right PIN left %d attempts of %d", left, before)
	}
	service.CancelRecoveryPhraseChange()
	assertFailure(t, service.ConfirmRecoveryPhraseChange(phrase), failureSetupNotActive)
	phrase, err = service.BeginRecoveryPhraseChange(context.Background(), confirmationPIN, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ConfirmRecoveryPhraseChange(phrase); err != nil {
		t.Fatal(err)
	}
	assertNoConfirmation(t, service)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockWithPIN(confirmationPIN); err != nil {
		t.Fatalf("the PIN after a new phrase: %v", err)
	}
}

func TestDeviceAuthenticationVerifiesANewPhraseAndThePINKeepsOpeningTheVault(t *testing.T) {
	service := newReadyService(t)
	answerOwner(t, service, nil)
	if err := service.SetPIN(context.Background(), confirmationPIN, ""); err != nil {
		t.Fatal(err)
	}
	before := unlockMethodsOf(t, service).PINAttemptsLeft
	declined := answerOwner(t, service, ownerauth.ErrCanceled)
	assertFailure(t, beginPhraseChange(service, confirmationPIN), failureOwnerUnverified)
	if left := unlockMethodsOf(t, service).PINAttemptsLeft; left != before {
		t.Fatalf("a declined owner counted a PIN attempt: %d of %d left", left, before)
	}
	approved := answerOwner(t, service, nil)
	assertFailure(t, beginPhraseChange(service, "999999"), failurePINWrong)
	if left := unlockMethodsOf(t, service).PINAttemptsLeft; left != before-1 {
		t.Fatalf("a wrong PIN after the owner verified left %d attempts of %d", left, before)
	}
	phrase, err := service.BeginRecoveryPhraseChange(context.Background(), confirmationPIN, "")
	if err != nil {
		t.Fatal(err)
	}
	if declined.times() != 1 || approved.times() != 2 {
		t.Fatalf("the owner was asked %d and %d times", declined.times(), approved.times())
	}
	if err := service.ConfirmRecoveryPhraseChange(phrase); err != nil {
		t.Fatal(err)
	}
	assertNoConfirmation(t, service)
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockWithPIN(confirmationPIN); err != nil {
		t.Fatalf("the PIN after a new phrase: %v", err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.Unlock(); err != nil {
		t.Fatalf("device authentication after a new phrase: %v", err)
	}
}

// unrelatedPhrase is a valid recovery phrase no test vault is sealed for.
const unrelatedPhrase = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon " +
	"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"

func TestANewPhraseForAVaultWithNeitherWayInTakesTheCurrentPhrase(t *testing.T) {
	service, device := newReadyServiceOnDevice(t)
	answerOwner(t, service, nil)
	current, err := service.BeginRecoveryPhraseChange(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ConfirmRecoveryPhraseChange(current); err != nil {
		t.Fatal(err)
	}
	device.noDeviceOwner = true
	owner := answerOwner(t, service, ownerauth.ErrCanceled)
	if _, err := service.BeginRecoveryPhraseChange(context.Background(), "", ""); err == nil {
		t.Fatal("a vault with neither way in gave a new phrase without its current one")
	} else {
		assertFailure(t, err, failureRecoveryPhraseInvalid)
	}
	_, err = service.BeginRecoveryPhraseChange(context.Background(), "", unrelatedPhrase)
	assertFailure(t, err, failureRecoveryKeyMismatch)
	phrase, err := service.BeginRecoveryPhraseChange(context.Background(), "", current)
	if err != nil {
		t.Fatalf("the current phrase: %v", err)
	}
	if owner.times() != 0 {
		t.Fatalf("a vault with neither way in asked the owner %d times", owner.times())
	}
	if err := service.ConfirmRecoveryPhraseChange(phrase); err != nil {
		t.Fatal(err)
	}
}

func beginPhraseChange(service *Service, pin string) error {
	_, err := service.BeginRecoveryPhraseChange(context.Background(), pin, "")
	return err
}
