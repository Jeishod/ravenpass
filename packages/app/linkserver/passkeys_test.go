package linkserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/dortanes/ravenpass/packages/app/linkproto"
)

var (
	passkeyID     = bytes.Repeat([]byte{0x5a}, 16)
	passkeyIDText = base64.RawURLEncoding.EncodeToString(passkeyID)
	signature     = []byte{0x30, 0x44, 0x02}
	userHandle    = []byte{1, 2, 3, 4}
)

func b64(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }

func (v *fakeVault) Passkeys(query linkproto.PasskeyQuery) (linkproto.Passkeys, error) {
	if err := v.call(fmt.Sprintf("passkeys %s %s %d", query.Origin, query.RPID, len(query.Allow))); err != nil {
		return linkproto.Passkeys{}, err
	}
	if query.Origin != exampleOrigin {
		return linkproto.Passkeys{RPID: "example.org"}, nil
	}
	return linkproto.Passkeys{RPID: "example.com", Passkeys: []linkproto.PasskeyOption{{Credential: exampleID, CredentialID: passkeyID, Account: "alex", Label: "Example"}}}, nil
}

func (v *fakeVault) PasskeyTargets(query linkproto.PasskeyQuery) (linkproto.PasskeyTargets, error) {
	if err := v.call(fmt.Sprintf("targets %s %s %s %d", query.Origin, query.RPID, query.Account, len(query.Exclude))); err != nil {
		return linkproto.PasskeyTargets{}, err
	}
	if query.Origin != exampleOrigin {
		return linkproto.PasskeyTargets{RPID: "example.org"}, nil
	}
	return linkproto.PasskeyTargets{RPID: "example.com", Targets: []linkproto.PasskeyTarget{{Credential: exampleID, Label: "Example", Account: "alex"}}, Excluded: true}, nil
}

// verify lets a call that may wait for the person answer once verifying, when set, does.
func (v *fakeVault) verify(ctx context.Context, asked func(linkproto.Progress)) error {
	v.mu.Lock()
	verifying := v.verifying
	v.mu.Unlock()
	if verifying == nil {
		return nil
	}
	return verifying(ctx, asked)
}

func (v *fakeVault) verifyWith(verifying func(ctx context.Context, asked func(linkproto.Progress)) error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.verifying = verifying
}

func (v *fakeVault) CreatePasskey(ctx context.Context, creation linkproto.PasskeyCreation, asked func(linkproto.Progress)) (linkproto.CreatedPasskey, error) {
	call := fmt.Sprintf("passkey-create %s %s %s %s %x %t %s", creation.Origin, creation.RPID, creation.RPName, creation.User.Name, creation.User.ID, creation.Verify, creation.Target)
	if err := v.call(call); err != nil {
		return linkproto.CreatedPasskey{}, err
	}
	if err := v.verify(ctx, asked); err != nil {
		return linkproto.CreatedPasskey{}, err
	}
	return linkproto.CreatedPasskey{
		Credential: exampleID, CredentialID: passkeyID, ClientDataJSON: []byte(`{"type":"webauthn.create"}`),
		AttestationObject: []byte{0xa3}, AuthenticatorData: []byte{0x49}, PublicKey: []byte{0x30, 0x59}, PublicKeyAlgorithm: -7,
	}, nil
}

func (v *fakeVault) SignPasskey(ctx context.Context, signIn linkproto.PasskeySignIn, asked func(linkproto.Progress)) (linkproto.PasskeyAssertion, error) {
	call := fmt.Sprintf("passkey-sign %s %s %t %s %s", signIn.Origin, signIn.RPID, signIn.Verify, signIn.Credential, b64(signIn.CredentialID))
	if err := v.call(call); err != nil {
		return linkproto.PasskeyAssertion{}, err
	}
	if err := v.verify(ctx, asked); err != nil {
		return linkproto.PasskeyAssertion{}, err
	}
	return linkproto.PasskeyAssertion{
		CredentialID: passkeyID, ClientDataJSON: []byte(`{"type":"webauthn.get"}`), AuthenticatorData: []byte{0x1d},
		Signature: signature, UserHandle: userHandle,
	}, nil
}

var (
	challengeText = b64([]byte("a challenge of at least sixteen bytes"))
	createRequest = `{"id":7,"type":"passkey-create","origin":"https://example.com","rpId":"example.com","rpName":"Example",` +
		`"user":{"id":"` + b64(userHandle) + `","name":"alex","displayName":"Alex"},"challenge":"` + challengeText + `","verify":true,"target":"new","exclude":[]}`
	signRequest = `{"id":7,"type":"passkey-sign","origin":"https://example.com","rpId":"example.com","challenge":"` + challengeText + `",` +
		`"verify":true,"credential":"` + exampleID + `","credentialId":"` + passkeyIDText + `"}`
	createCall = "passkey-create https://example.com example.com Example alex 01020304 true new"
	signCall   = "passkey-sign https://example.com example.com true " + exampleID + " " + passkeyIDText
	created    = `{"id":7,"result":{"credential":"` + exampleID + `","credentialId":"` + passkeyIDText + `","clientDataJSON":"` + b64([]byte(`{"type":"webauthn.create"}`)) +
		`","attestationObject":"ow","authenticatorData":"SQ","publicKey":"MFk","publicKeyAlgorithm":-7}}`
	signed = `{"id":7,"result":{"credentialId":"` + passkeyIDText + `","clientDataJSON":"` + b64([]byte(`{"type":"webauthn.get"}`)) +
		`","authenticatorData":"HQ","signature":"` + b64(signature) + `","userHandle":"` + b64(userHandle) + `"}}`
)

func TestAPasskeysRequestListsPasskeysOrTargets(t *testing.T) {
	extension, vault, port := linked(t)
	conn, transport := extension.openSession(t, port)
	get := `{"id":1,"type":"passkeys","origin":"https://example.com","rpId":"example.com","mode":"get","allow":["` + passkeyIDText + `"]}`
	want := `{"id":1,"result":{"rpId":"example.com","passkeys":[{"credential":"` + exampleID + `","credentialId":"` + passkeyIDText + `","account":"alex","label":"Example"}]}}`
	if reply := request(t, conn, transport, get); reply != want {
		t.Fatalf("passkeys = %s", reply)
	}
	create := `{"id":2,"type":"passkeys","origin":"https://example.com","mode":"create","exclude":["` + passkeyIDText + `"],"account":"alex"}`
	want = `{"id":2,"result":{"rpId":"example.com","targets":[{"credential":"` + exampleID + `","label":"Example","account":"alex"}],"excluded":true}}`
	if reply := request(t, conn, transport, create); reply != want {
		t.Fatalf("targets = %s", reply)
	}
	if reply := request(t, conn, transport, `{"id":3,"type":"passkeys","origin":"https://example.org","mode":"get"}`); reply != `{"id":3,"result":{"rpId":"example.org","passkeys":[]}}` {
		t.Fatalf("no passkeys = %s", reply)
	}
	if reply := request(t, conn, transport, `{"id":4,"type":"passkeys","origin":"https://example.org","mode":"create"}`); reply != `{"id":4,"result":{"rpId":"example.org","targets":[],"excluded":false}}` {
		t.Fatalf("no targets = %s", reply)
	}
	want4 := []string{"passkeys https://example.com example.com 1", "targets https://example.com  alex 1", "passkeys https://example.org  0", "targets https://example.org   0"}
	if calls := vault.takeCalls(); !slices.Equal(calls, want4) {
		t.Fatalf("the vault was asked %q", calls)
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPasskeyRequestsBeyondTheirBoundsAreRefusedBeforeTheVault(t *testing.T) {
	extension, vault, port := linked(t)
	conn, transport := extension.openSession(t, port)
	longChallenge := b64(make([]byte, linkproto.MaxChallengeBytes+1))
	for payload, code := range map[string]string{
		`{"id":1,"type":"passkeys","origin":"example.com","mode":"get"}`:                                                                        "invalid-origin",
		`{"id":1,"type":"passkeys","origin":"https://example.com","mode":"conditional"}`:                                                        "invalid-request",
		`{"id":1,"type":"passkey-create","origin":"https://example.com/path","challenge":"AQ","user":{"id":"AQ"}}`:                              "invalid-origin",
		`{"id":1,"type":"passkey-create","origin":"https://example.com","target":"new","user":{"id":"AQ"},"challenge":"` + longChallenge + `"}`: "invalid-request",
		`{"id":1,"type":"passkey-create","origin":"https://example.com","target":"new","challenge":"AQ"}`:                                       "invalid-request",
		`{"id":1,"type":"passkey-sign","origin":"https://example.com","challenge":"AQ","credential":"` + exampleID + `"}`:                       "invalid-request",
		`{"id":1,"type":"passkey-sign","origin":"","challenge":"AQ","credentialId":"AQ"}`:                                                       "invalid-origin",
	} {
		if reply := request(t, conn, transport, payload); reply != `{"id":1,"error":"`+code+`"}` {
			t.Fatalf("%s: %s, want %s", payload, reply, code)
		}
	}
	if calls := vault.takeCalls(); len(calls) != 0 {
		t.Fatalf("the vault was asked %q", calls)
	}
	sendRequest(t, conn, transport, `{"id":1,"type":"passkey-sign","origin":"https://example.com","challenge":"AQ==","credentialId":"AQ"}`)
	wantStatus(t, closeStatus(t, conn), linkproto.CloseMalformed)
}

func TestPasskeyCreationAndSignInAnswerWhatThePageReceives(t *testing.T) {
	extension, vault, port := linked(t)
	conn, transport := extension.openSession(t, port)
	if reply := request(t, conn, transport, createRequest); reply != created {
		t.Fatalf("created = %s", reply)
	}
	if reply := request(t, conn, transport, signRequest); reply != signed {
		t.Fatalf("signed = %s", reply)
	}
	if calls := vault.takeCalls(); !slices.Equal(calls, []string{createCall, signCall}) {
		t.Fatalf("the vault was asked %q", calls)
	}
	for _, reply := range []string{created, signed} {
		if strings.Contains(strings.ToLower(reply), "private") || strings.Contains(reply, `"key"`) {
			t.Fatalf("a reply names a key: %s", reply)
		}
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPasskeyRefusalsBecomeErrorCodes(t *testing.T) {
	extension, vault, port := linked(t)
	conn, transport := extension.openSession(t, port)
	for refusal, code := range map[error]string{
		ErrLocked:              "locked",
		ErrNotFound:            "not-found",
		ErrNoMatch:             "no-match",
		ErrDeclined:            "declined",
		ErrUnverifiable:        "unverifiable",
		ErrInvalidOrigin:       "invalid-origin",
		ErrInvalidRelyingParty: "invalid-rp",
		ErrExcluded:            "excluded",
		ErrPasskeysFull:        "full",
	} {
		vault.refuseWith(refusal)
		for _, payload := range []string{createRequest, signRequest, `{"id":7,"type":"passkeys","origin":"https://example.com","mode":"get"}`} {
			if reply := request(t, conn, transport, payload); reply != `{"id":7,"error":"`+code+`"}` {
				t.Fatalf("%v: %s, want %s", refusal, reply, code)
			}
		}
	}
	vault.refuseWith(nil)
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

// verifiedAfter is a verification that reports progress, waits for wait, then ends with err.
func verifiedAfter(progress linkproto.Progress, wait time.Duration, err error) func(context.Context, func(linkproto.Progress)) error {
	return func(ctx context.Context, asked func(linkproto.Progress)) error {
		asked(progress)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
		return err
	}
}

func TestPasskeyRequestsReportProgressWhileThePersonVerifies(t *testing.T) {
	server, vault := newServer(t, recordsPath(t), func(s *Server) {
		s.progressInterval = 40 * time.Millisecond
		s.idleTimeout = 100 * time.Millisecond
	})
	key := begin(t, server)
	extension := newExtension(t)
	extension.linkWith(t, key)
	conn, transport := extension.openSession(t, int(key.Port))
	for payload, answer := range map[string]string{createRequest: created, signRequest: signed} {
		vault.verifyWith(verifiedAfter(linkproto.ProgressConfirmOnDevice, 250*time.Millisecond, nil))
		sendRequest(t, conn, transport, payload)
		progress, final := progressThen(t, conn, transport)
		if len(progress) < 2 || slices.ContainsFunc(progress, func(p string) bool { return p != "confirm-on-device" }) {
			t.Fatalf("progress = %q", progress)
		}
		if final.ID != 7 || final.Error != "" {
			t.Fatalf("after the progress = %+v", final)
		}
		sendRequest(t, conn, transport, `{"id":8,"type":"status"}`)
		if reply := string(nextMessage(t, conn, transport)); reply != `{"id":8,"result":{"vault":"locked"}}` {
			t.Fatalf("a request after %s = %s", answer, reply)
		}

		vault.verifyWith(verifiedAfter(linkproto.ProgressConfirmInRavenpass, 10*time.Millisecond, ErrDeclined))
		sendRequest(t, conn, transport, payload)
		progress, refusal := progressThen(t, conn, transport)
		if !slices.Equal(progress, []string{"confirm-in-ravenpass"}) || refusal.ID != 7 || refusal.Error != "declined" || refusal.Result != nil {
			t.Fatalf("progress %q, then %+v", progress, refusal)
		}
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

func TestAnEndedSessionEndsItsPasskeyVerification(t *testing.T) {
	server, vault := newServer(t, recordsPath(t), func(s *Server) { s.progressInterval = 20 * time.Millisecond })
	key := begin(t, server)
	extension := newExtension(t)
	extension.linkWith(t, key)
	ended := make(chan error, 1)
	vault.verifyWith(func(ctx context.Context, asked func(linkproto.Progress)) error {
		asked(linkproto.ProgressConfirmOnDevice)
		<-ctx.Done()
		ended <- ctx.Err()
		return ctx.Err()
	})
	conn, transport := extension.openSession(t, int(key.Port))
	sendRequest(t, conn, transport, signRequest)
	if first := nextFrame(t, conn, transport); first.Progress != "confirm-on-device" {
		t.Fatalf("first message = %+v", first)
	}
	if err := conn.CloseNow(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-ended:
		if err == nil {
			t.Fatal("the verification ended without an error")
		}
	case <-time.After(testWait):
		t.Fatal("the verification kept waiting after the session ended")
	}
}
