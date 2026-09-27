package vault

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// standardSecret is the RFC 6238 test-vector secret "12345678901234567890" in base32.
const standardSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestSetupKeyIsStoredAsOneSecret(t *testing.T) {
	for _, typed := range []string{
		"JBSWY3DPEHPK3PXP",
		"jbswy3dpehpk3pxp",
		"jbsw y3dp ehpk 3pxp",
		"JBSW-Y3DP-EHPK-3PXP",
		"  JBSWY3DPEHPK3PXP====  ",
	} {
		stored, err := NormalizeTOTP(typed)
		if err != nil {
			t.Errorf("%q was refused: %v", typed, err)
			continue
		}
		if stored != "JBSWY3DPEHPK3PXP" {
			t.Errorf("%q was stored as %q", typed, stored)
		}
	}
}

func TestOrdinaryLinkIsStoredAsItsSecret(t *testing.T) {
	stored, err := NormalizeTOTP("otpauth://totp/?secret=JBSWY3DPEHPK3PXP&period=30&digits=6&algorithm=SHA1")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("a link asking for nothing unusual was stored as %q", stored)
	}
}

func TestLinkKeepsWhatTheSiteAskedFor(t *testing.T) {
	link := "otpauth://totp/Example:me@example.com?algorithm=SHA256&digits=8&issuer=Example&period=60&secret=" + standardSecret
	stored, err := NormalizeTOTP(link)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "otpauth://totp/") {
		t.Fatalf("a link with its own parameters was stored as %q", stored)
	}
	code, err := GenerateOneTimeCode(stored, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if code.Period != 60 || code.Digits != 8 || len(code.Code) != 8 {
		t.Fatalf("the stored parameters were not kept: %+v", code)
	}
	ordinary, err := GenerateOneTimeCode(standardSecret, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.Code == code.Code {
		t.Fatal("the digest and interval made no difference to the code")
	}
}

func TestUnusableSetupIsRefused(t *testing.T) {
	for name, typed := range map[string]string{
		"no secret":             "otpauth://totp/Example",
		"empty secret":          "otpauth://totp/Example?secret=",
		"secret not base32":     "otpauth://totp/Example?secret=not+base32!",
		"counter based":         "otpauth://hotp/Example?secret=" + standardSecret + "&counter=1",
		"another scheme":        "https://example.com/?secret=" + standardSecret,
		"interval of zero":      "otpauth://totp/Example?secret=" + standardSecret + "&period=0",
		"interval far too long": "otpauth://totp/Example?secret=" + standardSecret + "&period=86400",
		"too few digits":        "otpauth://totp/Example?secret=" + standardSecret + "&digits=4",
		"too many digits":       "otpauth://totp/Example?secret=" + standardSecret + "&digits=9",
		"punctuation only":      "!!!!",
	} {
		stored, err := NormalizeTOTP(typed)
		if !errors.Is(err, ErrInvalidTOTP) || !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v, want an invalid one-time code setup", name, err)
		}
		if stored != "" {
			t.Errorf("%s: a refused setup returned %q", name, stored)
		}
	}
}

func TestSetupLinkShowsItsIssuerAndAccount(t *testing.T) {
	for link, want := range map[string]SetupFace{
		"otpauth://totp/Example:alice@example.com?secret=" + standardSecret:                         {Issuer: "Example", Account: "alice@example.com"},
		"otpauth://totp/Example:%20alice@example.com?issuer=Example&secret=" + standardSecret:       {Issuer: "Example", Account: "alice@example.com"},
		"otpauth://totp/Old:alice?issuer=New%20Name&secret=" + standardSecret:                       {Issuer: "New Name", Account: "alice"},
		"otpauth://totp/alice@example.com?secret=" + standardSecret:                                 {Account: "alice@example.com"},
		"otpauth://totp/?issuer=Example&secret=" + standardSecret:                                   {Issuer: "Example"},
		"  otpauth://totp/?secret=" + standardSecret + "  ":                                         {},
		"OTPAUTH://totp/Example:alice?algorithm=SHA256&digits=8&period=60&secret=" + standardSecret: {Issuer: "Example", Account: "alice"},
	} {
		face, err := ReadSetupLink(link)
		if err != nil {
			t.Errorf("%q was refused: %v", link, err)
			continue
		}
		if face != want {
			t.Errorf("%q reads as %+v, want %+v", link, face, want)
		}
	}
}

func TestSetupLinkRefusesWhatCannotBeStored(t *testing.T) {
	for name, link := range map[string]string{
		"a bare setup key":      standardSecret,
		"an empty link":         "",
		"no secret":             "otpauth://totp/Example:alice",
		"secret not base32":     "otpauth://totp/Example:alice?secret=not+base32!",
		"counter based":         "otpauth://hotp/Example:alice?secret=" + standardSecret + "&counter=1",
		"another scheme":        "https://example.com/?secret=" + standardSecret,
		"too few digits":        "otpauth://totp/Example:alice?secret=" + standardSecret + "&digits=4",
		"interval far too long": "otpauth://totp/Example:alice?secret=" + standardSecret + "&period=86400",
		"longer than a setup":   "otpauth://totp/" + strings.Repeat("A", MaxTOTPLength-38) + "?secret=JBSWY3DPEHPK3PXP",
	} {
		face, err := ReadSetupLink(link)
		if !errors.Is(err, ErrInvalidTOTP) || !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v, want an invalid one-time code setup", name, err)
		}
		if face != (SetupFace{}) {
			t.Errorf("%s: a refused link read as %+v", name, face)
		}
	}
}

func TestNoSetupIsNotAFailure(t *testing.T) {
	for _, typed := range []string{"", "   "} {
		stored, err := NormalizeTOTP(typed)
		if err != nil || stored != "" {
			t.Errorf("%q: got %q, %v; want no setup and no error", typed, stored, err)
		}
	}
	if _, err := GenerateOneTimeCode("", time.Now()); !errors.Is(err, ErrNoTOTP) {
		t.Errorf("generating without a setup: got %v, want ErrNoTOTP", err)
	}
}

// The vectors are the SHA-1 ones RFC 6238 Appendix B publishes.
func TestCodesMatchTheStandardVectors(t *testing.T) {
	setup := "otpauth://totp/?digits=8&secret=" + standardSecret
	for at, expected := range map[int64]string{
		59:          "94287082",
		1111111109:  "07081804",
		1111111111:  "14050471",
		1234567890:  "89005924",
		2000000000:  "69279037",
		20000000000: "65353130",
	} {
		code, err := GenerateOneTimeCode(setup, time.Unix(at, 0))
		if err != nil {
			t.Fatal(err)
		}
		if code.Code != expected {
			t.Errorf("at %d: got %q, want %q", at, code.Code, expected)
		}
	}
}

func TestCodeLastsUntilTheEndOfItsInterval(t *testing.T) {
	// The interval holding these moments runs from 1699999980 to 1700000010.
	for _, at := range []int64{1699999980, 1699999981, 1700000009} {
		code, err := GenerateOneTimeCode(standardSecret, time.Unix(at, 0))
		if err != nil {
			t.Fatal(err)
		}
		if code.ExpiresAt.Unix() != 1700000010 {
			t.Fatalf("at %d the code expires at %d, want the end of its interval", at, code.ExpiresAt.Unix())
		}
	}
	current, err := GenerateOneTimeCode(standardSecret, time.Unix(1699999980, 0))
	if err != nil {
		t.Fatal(err)
	}
	next, err := GenerateOneTimeCode(standardSecret, current.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if next.Code == current.Code {
		t.Fatal("the next interval produced the same code")
	}
}

func TestCredentialKeepsItsOneTimeCodeSetup(t *testing.T) {
	session, _ := populatedSession(t)
	pending, id, err := session.PrepareCreate(CredentialInput{
		Label:    "With a second factor",
		Password: "secret",
		TOTP:     "jbsw y3dp ehpk 3pxp",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	ticket, err := session.BeginSelection(id)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := session.ReadSelected(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if credential.TOTP != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("the stored setup came back as %q", credential.TOTP)
	}
	cleared := ""
	pending, err = session.PrepareEdit(id, CredentialPatch{TOTP: &cleared})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	ticket, err = session.BeginSelection(id)
	if err != nil {
		t.Fatal(err)
	}
	credential, err = session.ReadSelected(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if credential.TOTP != "" {
		t.Fatalf("a cleared setup came back as %q", credential.TOTP)
	}
}

func TestCredentialRecordKeepsItsSetupOrNone(t *testing.T) {
	for _, setup := range []string{"", standardSecret} {
		plaintext, err := encodeCredentialRecord(CredentialInput{Label: "Example", Password: "secret", TOTP: setup})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeCredentialRecord(plaintext)
		if err != nil || decoded.Password != "secret" || decoded.TOTP != setup {
			t.Fatalf("setup %q came back as %+v, %v", setup, decoded, err)
		}
	}
}

// codeElement is an entry in the current form with the given summary and code face.
func codeElement(id ID, digest [32]byte, kind uint64, summary []byte, digits, period uint64) []byte {
	fields := currentFields(id, digest, kind, summary)
	fields[fieldDigits], fields[fieldPeriod] = encodeUint(digits), encodeUint(period)
	return encodeArray(fields...)
}

// wantCodes fails unless each item lists the code face want says.
func wantCodes(t *testing.T, session *Session, want map[ID]CodeFace) {
	t.Helper()
	for id, code := range want {
		if entry := listedEntry(t, session, id); entry.Code != code {
			t.Fatalf("code of %s = %+v, want %+v", id, entry.Code, code)
		}
	}
}

func TestCodeFaceIsTheSetupsDigitsAndPeriod(t *testing.T) {
	for setup, want := range map[string]CodeFace{
		"":             {},
		standardSecret: {Digits: 6, Period: 30},
		"otpauth://totp/Example?digits=8&period=60&secret=" + standardSecret: {Digits: 8, Period: 60},
		"otpauth://totp/Example?secret=" + standardSecret + "&period=0":      {},
	} {
		if got := (CredentialInput{TOTP: setup}).CodeFace(); got != want {
			t.Errorf("%q: got %+v, want %+v", setup, got, want)
		}
	}
}

func TestIndexKnowsEachCredentialsCodeFace(t *testing.T) {
	long := "otpauth://totp/Example?digits=8&period=60&secret=" + standardSecret
	created, ids := vaultWith(t,
		CredentialInput{Label: "Coded", Password: "one", TOTP: standardSecret},
		CredentialInput{Label: "Bare", Password: "two"},
		CredentialInput{Label: "Kept", Password: "three", TOTP: "jbsw y3dp ehpk 3pxp"},
		CredentialInput{Label: "Long", Password: "four", TOTP: long},
	)
	session := created.Session
	defer session.Lock()
	identity := commitIdentity(t, session, IdentityInput{Label: "Me"}, nil)
	card := commitCard(t, session, fullCard(), nil)
	note := commitNote(t, session, fullNote(), nil)
	seed := commitSeed(t, session, fullPhraseSeed(), nil)
	imported := commitBatch(t, session, []NewItem{
		{Credential: &CredentialInput{Label: "Imported", Password: "five", TOTP: long}},
		{Credential: &CredentialInput{Label: "Imported bare", Password: "six"}},
	}).Items
	ordinary, longer := CodeFace{Digits: 6, Period: 30}, CodeFace{Digits: 8, Period: 60}
	want := map[ID]CodeFace{ids[0]: ordinary, ids[1]: {}, ids[2]: ordinary, ids[3]: longer, imported[0]: longer, imported[1]: {}, identity: {}, card: {}, note: {}, seed: {}}
	wantCodes(t, session, want)

	cleared, added, changed, password := "", standardSecret, long, "rotated"
	for id, patch := range map[ID]CredentialPatch{ids[0]: {TOTP: &cleared}, ids[1]: {TOTP: &added}, ids[2]: {TOTP: &changed}, ids[3]: {Password: &password}} {
		pending, err := session.PrepareEdit(id, patch)
		commitPending(t, session, pending, err)
	}
	want[ids[0]], want[ids[1]], want[ids[2]] = CodeFace{}, ordinary, longer
	wantCodes(t, session, want)

	container, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWithRecovery(container, created.RecoveryPhrase)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Lock()
	wantCodes(t, reopened, want)
}

func TestSetupIsBoundedLikeEveryOtherField(t *testing.T) {
	session, _ := populatedSession(t)
	link := "otpauth://totp/" + strings.Repeat("A", MaxTOTPLength-39) + "?secret=JBSWY3DPEHPK3PXP"
	if len(link) != MaxTOTPLength {
		t.Fatalf("the test link is %d characters, not the limit", len(link))
	}
	pending, _, err := session.PrepareCreate(CredentialInput{Label: "Example", Password: "secret", TOTP: link}, nil)
	if err != nil {
		t.Fatalf("a setup at its limit was refused: %v", err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	over := "otpauth://totp/" + strings.Repeat("A", MaxTOTPLength-38) + "?secret=JBSWY3DPEHPK3PXP"
	if _, _, err := session.PrepareCreate(CredentialInput{Label: "Example", Password: "secret", TOTP: over}, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a setup over its limit: got %v, want ErrInvalidInput", err)
	}
}
