package extensionaccess

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/linkproto"
	"github.com/dortanes/ravenpass/packages/app/linkserver"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/app/verification"
	"github.com/dortanes/ravenpass/packages/vault"
)

const browserExtension = "extension-1"

func confirmingAccess(t *testing.T, credentials *fakeCredentials, verifier *fakeVerifier) *Access {
	t.Helper()
	access, err := New(credentials, &fakeIdentities{}, &fakeIcons{}, verifier, &fakeChoices{confirmFills: true}, &fakeUnlocks{})
	if err != nil {
		t.Fatal(err)
	}
	return access
}

func mailCredentials() *fakeCredentials {
	return &fakeCredentials{
		matched: vaultservice.Suggestion{ID: mailID, Label: "Mail", Account: "alex", Site: "example.com", Exact: true},
		fill:    vaultservice.Fill{Login: "alex", Password: "secret"},
		code:    vault.OneTimeCode{Code: "287082", Digits: 6, Period: 30},
	}
}

func TestAConfirmedFillWaitsForThePersonBeforeTheVaultReleasesAnything(t *testing.T) {
	credentials := mailCredentials()
	verifier := &fakeVerifier{method: verification.MethodDevice}
	access := confirmingAccess(t, credentials, verifier)
	var asked []linkproto.Progress
	fill, err := access.Fill(context.Background(), browserExtension, mailID.String(), "https://login.example.com", func(p linkproto.Progress) { asked = append(asked, p) })
	if err != nil || fill != (linkproto.Fill{Login: "alex", Password: "secret"}) {
		t.Fatalf("fill = %+v, error = %v", fill, err)
	}
	if !slices.Equal(verifier.reasons, []confirmation.Reason{confirmation.Filling("login.example.com", "alex")}) {
		t.Fatalf("the person was asked for %+v", verifier.reasons)
	}
	if !slices.Equal(asked, []linkproto.Progress{linkproto.ProgressConfirmOnDevice}) {
		t.Fatalf("progress = %q", asked)
	}
	want := []string{"match " + mailID.String(), "fill " + mailID.String()}
	if !slices.Equal(credentials.calls, want) {
		t.Fatalf("calls = %q, want %q", credentials.calls, want)
	}
	wantPages(t, credentials, "https://login.example.com", "https://login.example.com")
}

func TestAConfirmedCodeWaitsForThePersonToo(t *testing.T) {
	credentials := mailCredentials()
	credentials.matched.Account = ""
	verifier := &fakeVerifier{method: verification.MethodPIN}
	access := confirmingAccess(t, credentials, verifier)
	var asked []linkproto.Progress
	code, err := access.OneTimeCode(context.Background(), mailID.String(), "https://example.com", func(p linkproto.Progress) { asked = append(asked, p) })
	if err != nil || code.Code != "287082" {
		t.Fatalf("code = %+v, error = %v", code, err)
	}
	if !slices.Equal(verifier.reasons, []confirmation.Reason{confirmation.Filling("example.com", "Mail")}) {
		t.Fatalf("a credential without an account was named as %+v", verifier.reasons)
	}
	if !slices.Equal(asked, []linkproto.Progress{linkproto.ProgressConfirmInRavenpass}) {
		t.Fatalf("progress = %q", asked)
	}
	if want := []string{"match " + mailID.String(), "code " + mailID.String()}; !slices.Equal(credentials.calls, want) {
		t.Fatalf("calls = %q, want %q", credentials.calls, want)
	}
}

func TestADeclinedOrUnverifiableFillReleasesNothing(t *testing.T) {
	for cause, want := range map[error]error{
		verification.ErrDeclined:     linkserver.ErrDeclined,
		verification.ErrUnverifiable: linkserver.ErrUnverifiable,
	} {
		credentials := mailCredentials()
		access := confirmingAccess(t, credentials, &fakeVerifier{method: verification.MethodPIN, err: cause})
		ignore := func(linkproto.Progress) {}
		if _, err := access.Fill(context.Background(), browserExtension, mailID.String(), "https://example.com", ignore); !errors.Is(err, want) {
			t.Fatalf("fill after %v: got %v, want %v", cause, err, want)
		}
		if _, err := access.OneTimeCode(context.Background(), mailID.String(), "https://example.com", ignore); !errors.Is(err, want) {
			t.Fatalf("code after %v: got %v, want %v", cause, err, want)
		}
		if want := []string{"match " + mailID.String(), "match " + mailID.String()}; !slices.Equal(credentials.calls, want) {
			t.Fatalf("after %v the vault was asked %q", cause, credentials.calls)
		}
	}
}

func TestAConfirmedFillIsFilledOnceMoreForTheSameExtensionAndPageWithoutAsking(t *testing.T) {
	verifier := &fakeVerifier{method: verification.MethodDevice}
	access := confirmingAccess(t, mailCredentials(), verifier)
	now := time.Unix(1_790_000_000, 0)
	access.confirmed.now = func() time.Time { return now }
	ignore := func(linkproto.Progress) {}
	fill := func(extension, origin string) {
		t.Helper()
		if _, err := access.Fill(context.Background(), extension, mailID.String(), origin, ignore); err != nil {
			t.Fatal(err)
		}
	}
	wantAsked := func(what string, want int) {
		t.Helper()
		if len(verifier.reasons) != want {
			t.Fatalf("%s: the person was asked %d times, want %d", what, len(verifier.reasons), want)
		}
	}

	fill(browserExtension, "https://example.com")
	fill(browserExtension, "https://example.com")
	wantAsked("the password step after a confirmed sign-in", 1)
	fill(browserExtension, "https://example.com")
	wantAsked("a third fill", 2)

	fill("extension-2", "https://example.com")
	wantAsked("another extension", 3)
	fill(browserExtension, "https://login.example.com")
	wantAsked("another page", 4)

	fill(browserExtension, "https://www.example.com")
	now = now.Add(confirmedFillLifetime)
	fill(browserExtension, "https://www.example.com")
	wantAsked("a fill after the lifetime", 6)

	if _, err := access.OneTimeCode(context.Background(), mailID.String(), "https://example.org", ignore); err != nil {
		t.Fatal(err)
	}
	fill(browserExtension, "https://example.org")
	wantAsked("a fill after a confirmed code", 8)
}

func TestADeclinedFillSparesNoLaterConfirmation(t *testing.T) {
	verifier := &fakeVerifier{method: verification.MethodPIN, err: verification.ErrDeclined}
	access := confirmingAccess(t, mailCredentials(), verifier)
	ignore := func(linkproto.Progress) {}
	if _, err := access.Fill(context.Background(), browserExtension, mailID.String(), "https://example.com", ignore); !errors.Is(err, linkserver.ErrDeclined) {
		t.Fatalf("declined fill: %v", err)
	}
	verifier.err = nil
	if _, err := access.Fill(context.Background(), browserExtension, mailID.String(), "https://example.com", ignore); err != nil {
		t.Fatal(err)
	}
	if len(verifier.reasons) != 2 {
		t.Fatalf("the person was asked %d times, want 2", len(verifier.reasons))
	}
}

func TestAddWebsiteAddsThePageToACredentialThatMatchesIt(t *testing.T) {
	credentials := mailCredentials()
	access, _ := newAccess(t, credentials, &fakeIcons{})
	if err := access.AddWebsite(mailID.String(), "https://secure.example.com"); err != nil {
		t.Fatal(err)
	}
	want := []string{"match " + mailID.String(), "add " + mailID.String() + " https://secure.example.com"}
	if !slices.Equal(credentials.calls, want) {
		t.Fatalf("calls = %q, want %q", credentials.calls, want)
	}
	wantPages(t, credentials, "https://secure.example.com")
}

func TestAddWebsiteChangesNothingForACredentialThatDoesNotMatch(t *testing.T) {
	for cause, want := range map[error]error{
		vaultservice.ErrNoMatch:  linkserver.ErrNoMatch,
		vault.ErrNotFound:        linkserver.ErrNotFound,
		vaultservice.ErrNotReady: linkserver.ErrLocked,
	} {
		credentials := mailCredentials()
		credentials.err = cause
		access, _ := newAccess(t, credentials, &fakeIcons{})
		if err := access.AddWebsite(mailID.String(), "https://example.org"); !errors.Is(err, want) {
			t.Fatalf("add refused with %v: got %v, want %v", cause, err, want)
		}
		if want := []string{"match " + mailID.String()}; !slices.Equal(credentials.calls, want) {
			t.Fatalf("after %v the vault was asked %q", cause, credentials.calls)
		}
	}
	credentials := mailCredentials()
	access, _ := newAccess(t, credentials, &fakeIcons{})
	if err := access.AddWebsite("not-an-id", "https://example.com"); !errors.Is(err, linkserver.ErrNotFound) {
		t.Fatalf("an unknown credential: got %v", err)
	}
	if len(credentials.calls) != 0 {
		t.Fatalf("an unknown credential reached the vault: %q", credentials.calls)
	}
}

func TestAConfirmedFillTheVaultRefusesAsksNoOne(t *testing.T) {
	for cause, want := range map[error]error{
		vaultservice.ErrNoMatch:  linkserver.ErrNoMatch,
		vault.ErrNotFound:        linkserver.ErrNotFound,
		vaultservice.ErrNotReady: linkserver.ErrLocked,
	} {
		credentials := mailCredentials()
		credentials.err = cause
		verifier := &fakeVerifier{}
		access := confirmingAccess(t, credentials, verifier)
		if _, err := access.Fill(context.Background(), browserExtension, mailID.String(), "https://example.org", unasked(t)); !errors.Is(err, want) {
			t.Fatalf("fill refused with %v: got %v, want %v", cause, err, want)
		}
		if len(verifier.reasons) != 0 {
			t.Fatalf("a fill refused with %v asked the person", cause)
		}
	}
}
