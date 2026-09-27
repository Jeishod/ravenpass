package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// siteSetup is a site's setup link naming site and account, with eight digits every minute.
const siteSetup = "otpauth://totp/Example:alice@example.com?algorithm=SHA1&digits=8&issuer=Example&period=60&secret=" + standardSecret

// changesSeen is the change count the interface has not seen yet, zero when nothing changed.
func changesSeen(t *testing.T, service *Service) uint64 {
	t.Helper()
	ended, end := context.WithCancel(context.Background())
	end()
	count, err := service.AwaitVaultChange(ended, 0)
	if err != nil {
		return 0
	}
	return count
}

func pendingSetup(t *testing.T, service *Service) CodeSetup {
	t.Helper()
	setup, err := service.GetCodeSetup()
	if err != nil {
		t.Fatal(err)
	}
	return setup
}

// received holds link and answers the token that names it.
func received(t *testing.T, service *Service, link string) string {
	t.Helper()
	if !ControlsOf(service).ReceiveCodeSetup(link) {
		t.Fatalf("%s was refused", link)
	}
	return pendingSetup(t, service).Token
}

func TestALinkThatIsNoUsableSetupIsNotHeld(t *testing.T) {
	service := newReadyService(t)
	for name, link := range map[string]string{
		"a bare setup key":  standardSecret,
		"a web address":     "https://example.com/?secret=" + standardSecret,
		"counter based":     "otpauth://hotp/Example:alice?counter=1&secret=" + standardSecret,
		"secret not base32": "otpauth://totp/Example:alice?secret=not+base32!",
		"no secret":         "otpauth://totp/Example:alice",
	} {
		if ControlsOf(service).ReceiveCodeSetup(link) {
			t.Errorf("%s was held", name)
		}
	}
	if setup := pendingSetup(t, service); setup != (CodeSetup{}) {
		t.Fatalf("a refused link left %+v waiting", setup)
	}
	if count := changesSeen(t, service); count != 0 {
		t.Fatalf("a refused link told the interface of %d changes", count)
	}
}

func TestAReceivedSetupShowsItsNamesAndNeverItsSecret(t *testing.T) {
	service := newReadyService(t)
	token := received(t, service, siteSetup)
	setup := pendingSetup(t, service)
	if token == "" || setup != (CodeSetup{Pending: true, Token: token, Issuer: "Example", Account: "alice@example.com"}) {
		t.Fatalf("the waiting setup reads as %+v", setup)
	}
	shown, err := json.Marshal(setup)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(shown), standardSecret) {
		t.Fatalf("the interface was shown the secret: %s", shown)
	}
	if count := changesSeen(t, service); count != 1 {
		t.Fatalf("the interface was told of %d changes, want 1", count)
	}
}

func TestANewerSetupReplacesTheOneWaiting(t *testing.T) {
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{Label: "Example", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := received(t, service, siteSetup)
	second := received(t, service, "otpauth://totp/Other:bob?secret="+standardSecret)
	if setup := pendingSetup(t, service); setup.Issuer != "Other" || setup.Account != "bob" || second == first {
		t.Fatalf("the waiting setup reads as %+v", setup)
	}
	assertFailure(t, service.AddCodeSetup(first, id), failureCodeSetupExpired)
	if err := service.DismissCodeSetup(first); err != nil {
		t.Fatal(err)
	}
	if setup := pendingSetup(t, service); setup.Token != second {
		t.Fatal("dismissing the replaced setup dropped the newer one")
	}
	if credential, err := service.ReadCredential(id); err != nil || credential.TOTP != "" {
		t.Fatalf("the replaced setup reached the credential: %+v, %v", credential.CredentialInput, err)
	}
}

func TestAddingTheSetupGivesACredentialItsCode(t *testing.T) {
	for name, existing := range map[string]string{"without a code": "", "with a code": "JBSWY3DPEHPK3PXP"} {
		t.Run(name, func(t *testing.T) {
			service := newReadyService(t)
			id, err := service.CreateCredential(CredentialInput{Label: "Example", Login: "alice", Password: "secret", TOTP: existing}, nil)
			if err != nil {
				t.Fatal(err)
			}
			token := received(t, service, siteSetup)
			if err := service.AddCodeSetup(token, id); err != nil {
				t.Fatal(err)
			}
			credential, err := service.ReadCredential(id)
			if err != nil {
				t.Fatal(err)
			}
			want, err := vault.NormalizeTOTP(siteSetup)
			if err != nil {
				t.Fatal(err)
			}
			if credential.TOTP != want || credential.Password != "secret" || credential.Login != "alice" {
				t.Fatalf("the credential reads as %+v", credential.CredentialInput)
			}
			code, err := service.GenerateOneTimeCode(credential.TOTP)
			if err != nil || code.Digits != 8 || code.Period != 60 {
				t.Fatalf("code = %+v, error = %v", code, err)
			}
			if setup := pendingSetup(t, service); setup.Pending {
				t.Fatal("the added setup still waits")
			}
			assertFailure(t, service.AddCodeSetup(token, id), failureCodeSetupExpired)
		})
	}
}

func TestASetupAddedToWhatIsNoCredentialKeepsWaiting(t *testing.T) {
	service := newReadyService(t)
	note, err := service.CreateNote(NoteInput{Label: "Wi-Fi", Body: "ravenpass-home"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	token := received(t, service, siteSetup)
	for _, id := range []string{note, "0123456789abcdef0123456789abcdef", "not an id"} {
		assertFailure(t, service.AddCodeSetup(token, id), failureItemUnreadable)
	}
	if setup := pendingSetup(t, service); !setup.Pending {
		t.Fatal("a refused choice dropped the setup")
	}
}

func TestANewCredentialTakesTheSetupsNames(t *testing.T) {
	service := newReadyService(t)
	work := createTestGroup(t, service, "Work")
	if err := service.SetDefaultGroup(work); err != nil {
		t.Fatal(err)
	}
	token := received(t, service, siteSetup)
	id, err := service.CreateCredentialFromCodeSetup(token)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Label != "Example" || credential.Login != "alice@example.com" || credential.TOTP == "" || credential.Password != "" {
		t.Fatalf("the new credential reads as %+v", credential.CredentialInput)
	}
	assertSameGroups(t, summaryGroups(t, service, id), work)
	if setup := pendingSetup(t, service); setup.Pending {
		t.Fatal("the added setup still waits")
	}
	_, err = service.CreateCredentialFromCodeSetup(token)
	assertFailure(t, err, failureCodeSetupExpired)
}

func TestANewCredentialIsNamedByTheAccountWithoutAnIssuer(t *testing.T) {
	service := newReadyService(t)
	token := received(t, service, "otpauth://totp/alice@example.com?secret="+standardSecret)
	id, err := service.CreateCredentialFromCodeSetup(token)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Label != "alice@example.com" || credential.Login != "alice@example.com" {
		t.Fatalf("the new credential reads as %+v", credential.CredentialInput)
	}
}

func TestASetupNamingNoOneMakesNoCredential(t *testing.T) {
	service := newReadyService(t)
	token := received(t, service, "otpauth://totp/?digits=8&secret="+standardSecret)
	_, err := service.CreateCredentialFromCodeSetup(token)
	assertFailure(t, err, failureInvalidItem)
	if listed, err := service.ListCredentials(); err != nil || len(listed) != 0 {
		t.Fatalf("credentials = %+v, error = %v", listed, err)
	}
	if setup := pendingSetup(t, service); !setup.Pending {
		t.Fatal("a refused credential dropped the setup")
	}
}

func TestDismissingForgetsTheSetup(t *testing.T) {
	service := newReadyService(t)
	token := received(t, service, siteSetup)
	if err := service.DismissCodeSetup(token); err != nil {
		t.Fatal(err)
	}
	if setup := pendingSetup(t, service); setup != (CodeSetup{}) {
		t.Fatalf("a dismissed setup reads as %+v", setup)
	}
	_, err := service.CreateCredentialFromCodeSetup(token)
	assertFailure(t, err, failureCodeSetupExpired)
}

func TestTheSetupWaitsWhileTheVaultIsLocked(t *testing.T) {
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{Label: "Example", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if !ControlsOf(service).ReceiveCodeSetup(siteSetup) {
		t.Fatal("a setup arriving while the vault is locked was refused")
	}
	_, err = service.GetCodeSetup()
	assertFailure(t, err, failureVaultLocked)
	assertFailure(t, service.AddCodeSetup("1", id), failureVaultLocked)
	_, err = service.CreateCredentialFromCodeSetup("1")
	assertFailure(t, err, failureVaultLocked)
	assertFailure(t, service.DismissCodeSetup("1"), failureVaultLocked)
	if err := service.Unlock(); err != nil {
		t.Fatal(err)
	}
	setup := pendingSetup(t, service)
	if !setup.Pending {
		t.Fatal("the setup was dropped with the lock")
	}
	if err := service.AddCodeSetup(setup.Token, id); err != nil {
		t.Fatal(err)
	}
}

func TestAHeldSetupExpiresAfterItsLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var held heldCodeSetup
		face := vault.SetupFace{Issuer: "Example"}
		held.keep(siteSetup, face)
		time.Sleep(codeSetupLifetime - time.Second)
		synctest.Wait()
		got, token, pending := held.held()
		if !pending || got != face {
			t.Fatalf("before its lifetime ends the setup reads as %+v, %v", got, pending)
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if _, _, pending := held.held(); pending {
			t.Fatal("the setup outlived its lifetime")
		}
		assertFailure(t, held.apply(token, func(string, vault.SetupFace) error { return nil }), failureCodeSetupExpired)
	})
}

func TestAReplacingSetupGetsALifetimeOfItsOwn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var held heldCodeSetup
		held.keep(siteSetup, vault.SetupFace{Issuer: "First"})
		time.Sleep(codeSetupLifetime / 2)
		later := vault.SetupFace{Issuer: "Second"}
		held.keep(siteSetup, later)
		time.Sleep(codeSetupLifetime * 3 / 4)
		synctest.Wait()
		if got, _, pending := held.held(); !pending || got != later {
			t.Fatalf("the replaced setup's expiry left %+v, %v", got, pending)
		}
		time.Sleep(codeSetupLifetime / 2)
		synctest.Wait()
		if _, _, pending := held.held(); pending {
			t.Fatal("the replacing setup outlived its lifetime")
		}
	})
}
