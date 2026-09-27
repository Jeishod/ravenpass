package api

import (
	"testing"
	"time"
)

// standardSecret is the shared secret RFC 6238 publishes its test vectors for.
const standardSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestOneTimeCodeSetupIsStoredCanonically(t *testing.T) {
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{
		Label:    "Mail",
		Password: "secret",
		TOTP:     "gezd gnbv gy3t qojq gezd gnbv gy3t qojq",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	if credential.TOTP != standardSecret {
		t.Fatalf("the setup was stored as %q", credential.TOTP)
	}
	code, err := service.GenerateOneTimeCode(credential.TOTP)
	if err != nil {
		t.Fatal(err)
	}
	if len(code.Code) != code.Digits || code.Digits != 6 || code.Period != 30 {
		t.Fatalf("code = %+v", code)
	}
	if code.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatalf("the code expired before it was shown: %+v", code)
	}
}

func TestCopyingTheOneTimeCodeNeverCopiesTheSecret(t *testing.T) {
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret", TOTP: standardSecret}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clipboard := attachPasteboard(service)
	schedule := func(time.Duration, func()) {}
	before, err := service.GenerateOneTimeCode(standardSecret)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.copyCredentialField(id, "totp", schedule); err != nil {
		t.Fatal(err)
	}
	after, err := service.GenerateOneTimeCode(standardSecret)
	if err != nil {
		t.Fatal(err)
	}
	if clipboard.text == standardSecret {
		t.Fatal("the shared secret reached the clipboard")
	}
	// The copy and these two readings can fall either side of an interval boundary.
	if clipboard.text != before.Code && clipboard.text != after.Code {
		t.Fatalf("the clipboard holds %q, which is neither current code", clipboard.text)
	}
}

func TestCredentialWithoutASetupHasNoCodeToCopy(t *testing.T) {
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clipboard := attachPasteboard(service)
	err = service.copyCredentialField(id, "totp", func(time.Duration, func()) {})
	if err == nil || err.Error() != failurePrefix+string(failureNoCode) {
		t.Fatalf("copying without a setup reported %v", err)
	}
	if clipboard.text != "" {
		t.Fatalf("the clipboard changed to %q", clipboard.text)
	}
	if _, err := service.GenerateOneTimeCode(""); err == nil || err.Error() != failurePrefix+string(failureNoCode) {
		t.Fatalf("generating without a setup reported %v", err)
	}
}

func TestUnusableSetupIsReportedAsSuch(t *testing.T) {
	service := newReadyService(t)
	_, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret", TOTP: "not base32!"}, nil)
	if err == nil || err.Error() != failurePrefix+string(failureInvalidCode) {
		t.Fatalf("an unusable setup reported %v", err)
	}
	summaries, err := service.ListCredentials()
	if err != nil || len(summaries) != 0 {
		t.Fatalf("a refused credential was stored: %+v, error = %v", summaries, err)
	}
}
