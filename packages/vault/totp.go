package vault

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// A one-time code setup (RFC 6238) is stored as its bare secret when every parameter is default, else as its otpauth link.
const (
	defaultCodeDigits = 6
	defaultCodePeriod = 30
	minCodeDigits     = 6
	maxCodeDigits     = 8
	// A period over ten minutes is taken as a mistyped link.
	maxCodePeriod = 600
)

const setupScheme = "otpauth"

// OneTimeCode is the code a setup produces at one moment, and when it stops being accepted.
type OneTimeCode struct {
	Code      string
	Digits    int
	Period    int
	ExpiresAt time.Time
}

// CodeFace is what the list shows of a one-time code setup: digits per code and seconds per code.
type CodeFace struct {
	Digits int
	Period int
}

// SetupFace is the issuer and account an otpauth link names, trimmed.
type SetupFace struct {
	Issuer  string
	Account string
}

type codeSetup struct {
	canonical string
	secret    string
	digits    otp.Digits
	period    int
	algorithm otp.Algorithm
	encoder   otp.Encoder
	face      SetupFace
}

// NormalizeTOTP returns a one-time code setup as stored, empty for none, refusing one that cannot produce a code.
func NormalizeTOTP(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	setup, err := parseTOTP(raw)
	if err != nil {
		return "", err
	}
	return setup.canonical, nil
}

// ReadSetupLink reads an otpauth link's face, refusing a bare key, a link over MaxTOTPLength and any setup NormalizeTOTP refuses.
func ReadSetupLink(link string) (SetupFace, error) {
	trimmed := strings.TrimSpace(link)
	if !isSetupLink(trimmed) || !fits(trimmed, MaxTOTPLength) {
		return SetupFace{}, invalidTOTP()
	}
	setup, err := parseTOTP(trimmed)
	if err != nil {
		return SetupFace{}, err
	}
	return setup.face, nil
}

// HasCode reports whether the credential holds a one-time code setup.
func (input CredentialInput) HasCode() bool {
	return input.TOTP != ""
}

// CodeFace is the face of the credential's setup, zero when it holds none or an unusable one.
func (input CredentialInput) CodeFace() CodeFace {
	if !input.HasCode() {
		return CodeFace{}
	}
	setup, err := parseTOTP(input.TOTP)
	if err != nil {
		return CodeFace{}
	}
	return CodeFace{Digits: int(setup.digits), Period: setup.period}
}

// validCodeFace reports a face some setup can have, or none.
func validCodeFace(face CodeFace) bool {
	return face == CodeFace{} ||
		face.Digits >= minCodeDigits && face.Digits <= maxCodeDigits && face.Period > 0 && face.Period <= maxCodePeriod
}

// GenerateOneTimeCode derives the code a stored setup produces at the given moment.
func GenerateOneTimeCode(stored string, at time.Time) (OneTimeCode, error) {
	if strings.TrimSpace(stored) == "" {
		return OneTimeCode{}, ErrNoTOTP
	}
	setup, err := parseTOTP(stored)
	if err != nil {
		return OneTimeCode{}, err
	}
	code, err := setup.codeAt(at)
	if err != nil {
		return OneTimeCode{}, err
	}
	period := int64(setup.period)
	return OneTimeCode{
		Code:      code,
		Digits:    int(setup.digits),
		Period:    setup.period,
		ExpiresAt: time.Unix((at.Unix()/period+1)*period, 0).UTC(),
	}, nil
}

func parseTOTP(raw string) (codeSetup, error) {
	trimmed := strings.TrimSpace(raw)
	if !isSetupLink(trimmed) {
		link := url.URL{Scheme: setupScheme, Host: "totp", Path: "/", RawQuery: url.Values{"secret": {tidySecret(trimmed)}}.Encode()}
		trimmed = link.String()
	}
	// The parser defaults what a link omits and leaves an unusable secret to fail at generation.
	key, err := otp.NewKeyFromURL(trimmed)
	if err != nil {
		return codeSetup{}, invalidTOTP()
	}
	if !strings.EqualFold(key.Type(), "totp") {
		return codeSetup{}, invalidTOTP()
	}
	secret := tidySecret(key.Secret())
	period := key.Period()
	if secret == "" || period == 0 || period > maxCodePeriod {
		return codeSetup{}, invalidTOTP()
	}
	digits := key.Digits()
	if digits < minCodeDigits || digits > maxCodeDigits {
		return codeSetup{}, invalidTOTP()
	}
	setup := codeSetup{
		canonical: key.URL(),
		secret:    secret,
		digits:    digits,
		period:    int(period),
		algorithm: key.Algorithm(),
		encoder:   key.Encoder(),
		face: SetupFace{
			Issuer:  strings.TrimSpace(key.Issuer()),
			Account: strings.TrimSpace(key.AccountName()),
		},
	}
	if _, err := setup.codeAt(time.Unix(0, 0)); err != nil {
		return codeSetup{}, invalidTOTP()
	}
	if setup.ordinary() {
		setup.canonical = secret
	}
	return setup, nil
}

// isSetupLink reports a trimmed setup given as an otpauth link.
func isSetupLink(trimmed string) bool {
	return strings.HasPrefix(strings.ToLower(trimmed), setupScheme+"://")
}

func (s codeSetup) codeAt(at time.Time) (string, error) {
	return totp.GenerateCodeCustom(s.secret, at, totp.ValidateOpts{
		Period:    uint(s.period),
		Digits:    s.digits,
		Algorithm: s.algorithm,
		Encoder:   s.encoder,
	})
}

// ordinary reports a setup whose every parameter is the default, which its bare secret carries.
func (s codeSetup) ordinary() bool {
	return s.face == SetupFace{} &&
		s.digits == defaultCodeDigits && s.period == defaultCodePeriod &&
		s.algorithm == otp.AlgorithmSHA1 && s.encoder == otp.EncoderDefault
}

// tidySecret drops the spacing and padding sites print a setup key with.
func tidySecret(value string) string {
	return strings.ToUpper(strings.Map(func(character rune) rune {
		switch character {
		case ' ', '\t', '\n', '\r', '-', '=':
			return -1
		}
		return character
	}, value))
}

func invalidTOTP() error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, ErrInvalidTOTP)
}
