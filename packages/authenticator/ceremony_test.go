package authenticator

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	testOrigin = "https://login.example.com"
	testRPID   = "example.com"
)

// profiles are every Profile with the backup state its responses carry.
var profiles = []struct {
	name     string
	profile  Profile
	backedUp bool
}{
	{name: "browser extension", profile: BrowserExtension},
	{name: "platform provider", profile: PlatformProvider, backedUp: true},
}

type account struct {
	handle      []byte
	credentials []webauthn.Credential
}

func (a *account) WebAuthnID() []byte                         { return a.handle }
func (a *account) WebAuthnName() string                       { return "alex@example.com" }
func (a *account) WebAuthnDisplayName() string                { return "Alex" }
func (a *account) WebAuthnCredentials() []webauthn.Credential { return a.credentials }

func verifyingParty(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	party, err := webauthn.New(&webauthn.Config{
		RPID:          testRPID,
		RPDisplayName: "Example",
		RPOrigins:     []string{testOrigin},
	})
	if err != nil {
		t.Fatal(err)
	}
	return party
}

func checkedRPID(t *testing.T) string {
	t.Helper()
	rpID, err := CheckRelyingParty(testOrigin, testRPID)
	if err != nil {
		t.Fatal(err)
	}
	return rpID
}

func userVerification(verified bool) protocol.UserVerificationRequirement {
	if verified {
		return protocol.VerificationRequired
	}
	return protocol.VerificationDiscouraged
}

func encode(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

func publicKeyCredential(t *testing.T, credentialID []byte, response map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"id":                      encode(credentialID),
		"rawId":                   encode(credentialID),
		"type":                    "public-key",
		"authenticatorAttachment": "platform",
		"clientExtensionResults":  map[string]any{},
		"response":                response,
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func register(t *testing.T, party *webauthn.WebAuthn, user *account, key Key, profile Profile, verified bool) (webauthn.Credential, error) {
	t.Helper()
	options, session, err := party.BeginRegistration(user, webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
		UserVerification: userVerification(verified),
	}))
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := profile.Attest(checkedRPID(t), key.CredentialID, key.PrivateKey, verified)
	if err != nil {
		t.Fatal(err)
	}
	body := publicKeyCredential(t, key.CredentialID, map[string]any{
		"clientDataJSON":     encode(ClientData(Create, options.Response.Challenge, testOrigin)),
		"attestationObject":  encode(attestation.AttestationObject),
		"authenticatorData":  encode(attestation.AuthenticatorData),
		"publicKey":          encode(attestation.PublicKey),
		"publicKeyAlgorithm": attestation.Algorithm,
		"transports":         []string{"internal", "hybrid"},
	})
	parsed, err := protocol.ParseCredentialCreationResponseBytes(body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsed.Response.AttestationObject.RawAuthData, attestation.AuthenticatorData) {
		t.Fatal("the attestation object carries other authenticator data than the attestation reports")
	}
	credential, err := party.CreateCredential(user, *session, parsed)
	if err != nil {
		return webauthn.Credential{}, err
	}
	samePublicKey(t, credential.PublicKey, attestation.PublicKey)
	return *credential, nil
}

// samePublicKey checks that getPublicKey() returns the key the COSE key in the attestation holds.
func samePublicKey(t *testing.T, cose, spki []byte) {
	t.Helper()
	decoded, err := webauthncose.ParsePublicKey(cose)
	if err != nil {
		t.Fatal(err)
	}
	ec2, ok := decoded.(webauthncose.EC2PublicKeyData)
	if !ok {
		t.Fatalf("COSE key is %T, want EC2", decoded)
	}
	parsed, err := x509.ParsePKIXPublicKey(spki)
	if err != nil {
		t.Fatal(err)
	}
	point, err := parsed.(*ecdsa.PublicKey).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if ec2.Algorithm != AlgorithmES256 || !bytes.Equal(point, append(append([]byte{0x04}, ec2.XCoord...), ec2.YCoord...)) {
		t.Fatal("the SubjectPublicKeyInfo and the COSE key are different keys")
	}
}

// signIn returns the relying party's verdict on sign's answer to client data a browser at testOrigin builds.
func signIn(t *testing.T, party *webauthn.WebAuthn, user *account, credentialID []byte, verified bool, sign func(clientData []byte) (Assertion, error)) (*webauthn.Credential, protocol.AuthenticatorFlags, error) {
	t.Helper()
	options, session, err := party.BeginLogin(user, webauthn.WithUserVerification(userVerification(verified)))
	if err != nil {
		t.Fatal(err)
	}
	clientData := ClientData(Get, options.Response.Challenge, testOrigin)
	assertion, err := sign(clientData)
	if err != nil {
		t.Fatal(err)
	}
	body := publicKeyCredential(t, credentialID, map[string]any{
		"clientDataJSON":    encode(clientData),
		"authenticatorData": encode(assertion.AuthenticatorData),
		"signature":         encode(assertion.Signature),
		"userHandle":        encode(user.handle),
	})
	parsed, err := protocol.ParseCredentialRequestResponseBytes(body)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := party.ValidateLogin(user, *session, parsed)
	return credential, parsed.Response.AuthenticatorData.Flags, err
}

// Backup eligibility stays set across profiles while the backup state follows the signing profile.
func TestARelyingPartyAcceptsEveryRegistrationAndSignIn(t *testing.T) {
	for _, registered := range profiles {
		for _, verified := range []bool{true, false} {
			party := verifyingParty(t)
			user := &account{handle: []byte("user-handle-alex")}
			key := newKey(t)
			credential, err := register(t, party, user, key, registered.profile, verified)
			if err != nil {
				t.Fatalf("%s, verified %t: the relying party refused the registration: %v", registered.name, verified, err)
			}
			if !bytes.Equal(credential.ID, key.CredentialID) || !bytes.Equal(credential.Authenticator.AAGUID, AAGUID[:]) {
				t.Fatalf("%s, verified %t: registered credential ID % x with AAGUID % x", registered.name, verified, credential.ID, credential.Authenticator.AAGUID)
			}
			if credential.AttestationFormat != "none" || credential.Authenticator.SignCount != 0 {
				t.Fatalf("%s, verified %t: attestation %q with counter %d", registered.name, verified, credential.AttestationFormat, credential.Authenticator.SignCount)
			}
			flags := credential.Flags
			if !flags.UserPresent || flags.UserVerified != verified || !flags.BackupEligible || flags.BackupState != registered.backedUp {
				t.Fatalf("%s, verified %t: registration flags %+v", registered.name, verified, flags)
			}
			user.credentials = []webauthn.Credential{credential}

			for _, signing := range profiles {
				for _, counter := range []uint32{0, 7} {
					signedIn, flags, err := signIn(t, party, user, key.CredentialID, verified, func(clientData []byte) (Assertion, error) {
						return signing.profile.Assert(checkedRPID(t), key.PrivateKey, counter, verified, clientData)
					})
					if err != nil {
						t.Fatalf("registered by %s, signed by %s, verified %t, counter %d: the relying party refused the sign-in: %v", registered.name, signing.name, verified, counter, err)
					}
					if signedIn.Authenticator.SignCount != counter || signedIn.Authenticator.CloneWarning {
						t.Fatalf("registered by %s, signed by %s, verified %t, counter %d: relying party read counter %d, clone warning %t", registered.name, signing.name, verified, counter, signedIn.Authenticator.SignCount, signedIn.Authenticator.CloneWarning)
					}
					if !flags.HasUserPresent() || flags.HasUserVerified() != verified || !flags.HasBackupEligible() || flags.HasBackupState() != signing.backedUp || flags.HasAttestedCredentialData() {
						t.Fatalf("registered by %s, signed by %s, verified %t, counter %d: sign-in flags %08b", registered.name, signing.name, verified, counter, flags)
					}
				}
			}
		}
	}
}

func TestARelyingPartyAcceptsASignInSignedOverOnlyTheClientDataHash(t *testing.T) {
	for _, tested := range profiles {
		for _, verified := range []bool{true, false} {
			party := verifyingParty(t)
			user := &account{handle: []byte("user-handle-alex")}
			key := newKey(t)
			credential, err := register(t, party, user, key, tested.profile, verified)
			if err != nil {
				t.Fatalf("%s, verified %t: the relying party refused the registration: %v", tested.name, verified, err)
			}
			user.credentials = []webauthn.Credential{credential}
			signedIn, flags, err := signIn(t, party, user, key.CredentialID, verified, func(clientData []byte) (Assertion, error) {
				return tested.profile.AssertHash(checkedRPID(t), key.PrivateKey, 7, verified, sha256.Sum256(clientData))
			})
			if err != nil {
				t.Fatalf("%s, verified %t: the relying party refused the sign-in: %v", tested.name, verified, err)
			}
			if signedIn.Authenticator.SignCount != 7 || signedIn.Authenticator.CloneWarning {
				t.Fatalf("%s, verified %t: relying party read counter %d, clone warning %t", tested.name, verified, signedIn.Authenticator.SignCount, signedIn.Authenticator.CloneWarning)
			}
			if !flags.HasUserPresent() || flags.HasUserVerified() != verified || !flags.HasBackupEligible() || flags.HasBackupState() != tested.backedUp || flags.HasAttestedCredentialData() {
				t.Fatalf("%s, verified %t: sign-in flags %08b", tested.name, verified, flags)
			}
		}
	}
}

func TestASignInOverClientDataAndOverItsHashCarryTheSameAuthenticatorData(t *testing.T) {
	key := newKey(t)
	clientData := ClientData(Get, []byte("challenge"), testOrigin)
	private, err := parseKey(key.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(clientData)
	for _, tested := range profiles {
		overData, err := tested.profile.Assert(testRPID, key.PrivateKey, 5, true, clientData)
		if err != nil {
			t.Fatal(err)
		}
		overHash, err := tested.profile.AssertHash(testRPID, key.PrivateKey, 5, true, hash)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(overData.AuthenticatorData, overHash.AuthenticatorData) {
			t.Fatalf("%s: authenticator data % x over the client data, % x over its hash", tested.name, overData.AuthenticatorData, overHash.AuthenticatorData)
		}
		signed := sha256.Sum256(append(bytes.Clone(overHash.AuthenticatorData), hash[:]...))
		for name, assertion := range map[string]Assertion{"client data": overData, "hash": overHash} {
			if !ecdsa.VerifyASN1(&private.PublicKey, signed[:], assertion.Signature) {
				t.Fatalf("%s: the signature over the %s does not verify against the authenticator data and the hash", tested.name, name)
			}
		}
	}
}

func TestARelyingPartyThatRequiresVerificationRefusesAnUnverifiedResponse(t *testing.T) {
	party := verifyingParty(t)
	user := &account{handle: []byte("user-handle-alex")}
	key := newKey(t)
	credential, err := register(t, party, user, key, BrowserExtension, true)
	if err != nil {
		t.Fatal(err)
	}
	user.credentials = []webauthn.Credential{credential}

	options, session, err := party.BeginLogin(user, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		t.Fatal(err)
	}
	clientData := ClientData(Get, options.Response.Challenge, testOrigin)
	assertion, err := BrowserExtension.Assert(testRPID, key.PrivateKey, 1, false, clientData)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(publicKeyCredential(t, key.CredentialID, map[string]any{
		"clientDataJSON":    encode(clientData),
		"authenticatorData": encode(assertion.AuthenticatorData),
		"signature":         encode(assertion.Signature),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := party.ValidateLogin(user, *session, parsed); err == nil {
		t.Fatal("an unverified sign-in was accepted where verification is required")
	}
}

func TestARelyingPartyRefusesASignatureOverOtherClientData(t *testing.T) {
	party := verifyingParty(t)
	user := &account{handle: []byte("user-handle-alex")}
	key := newKey(t)
	credential, err := register(t, party, user, key, BrowserExtension, true)
	if err != nil {
		t.Fatal(err)
	}
	user.credentials = []webauthn.Credential{credential}

	options, session, err := party.BeginLogin(user)
	if err != nil {
		t.Fatal(err)
	}
	clientData := ClientData(Get, options.Response.Challenge, testOrigin)
	assertion, err := BrowserExtension.Assert(testRPID, key.PrivateKey, 1, true, ClientData(Get, options.Response.Challenge, "https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(publicKeyCredential(t, key.CredentialID, map[string]any{
		"clientDataJSON":    encode(clientData),
		"authenticatorData": encode(assertion.AuthenticatorData),
		"signature":         encode(assertion.Signature),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := party.ValidateLogin(user, *session, parsed); err == nil {
		t.Fatal("a signature over other client data was accepted")
	}
}
