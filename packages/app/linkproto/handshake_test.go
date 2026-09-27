package linkproto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func generateKeyPair(t *testing.T) KeyPair {
	t.Helper()
	key, err := GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func randomSecret(t *testing.T) []byte {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	return secret
}

// pass writes the sender's next handshake message and reads it on the receiving side.
func pass(t *testing.T, sender, receiver *Handshake, payload []byte) []byte {
	t.Helper()
	message, err := sender.Write(payload)
	if err != nil {
		t.Fatal(err)
	}
	received, err := receiver.Read(message)
	if err != nil {
		t.Fatal(err)
	}
	return received
}

// exchange seals plaintext on one transport and opens it on the other.
func exchange(t *testing.T, sender, receiver *Transport, plaintext string) {
	t.Helper()
	sealed, err := sender.Seal([]byte(plaintext))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(plaintext)) {
		t.Fatal("a transport message carries its plaintext")
	}
	opened, err := receiver.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != plaintext {
		t.Fatalf("opened %q, want %q", opened, plaintext)
	}
}

func TestLinkHandshakeAuthenticatesBothStaticKeysWithTheSecret(t *testing.T) {
	extension, desktop, secret := generateKeyPair(t), generateKeyPair(t), randomSecret(t)
	initiator, err := LinkInitiator(extension, secret, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	responder, err := LinkResponder(desktop, secret, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if payload := pass(t, initiator, responder, nil); len(payload) != 0 {
		t.Fatalf("first message carried %q", payload)
	}
	if payload := pass(t, responder, initiator, nil); len(payload) != 0 {
		t.Fatalf("second message carried %q", payload)
	}
	payload := pass(t, initiator, responder, []byte(`{"name":"Chrome · macOS"}`))
	request, err := ParseLinkRequest(payload)
	if err != nil || request.Name != "Chrome · macOS" {
		t.Fatalf("link request = %+v, error = %v", request, err)
	}
	if !bytes.Equal(responder.PeerStatic(), extension.Public) || !bytes.Equal(initiator.PeerStatic(), desktop.Public) {
		t.Fatal("a side learned the wrong static key")
	}
	if initiator.Transport() == nil || responder.Transport() == nil {
		t.Fatal("the handshake did not complete after three messages")
	}
	exchange(t, responder.Transport(), initiator.Transport(), `{"type":"linked","language":"en"}`)
	exchange(t, initiator.Transport(), responder.Transport(), "towards the desktop app")
}

func TestLinkHandshakeFailsWithAnotherSecret(t *testing.T) {
	initiator, err := LinkInitiator(generateKeyPair(t), randomSecret(t), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	responder, err := LinkResponder(generateKeyPair(t), randomSecret(t), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pass(t, initiator, responder, nil)
	pass(t, responder, initiator, nil)
	third, err := initiator.Write([]byte(`{"name":"Chrome · macOS"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := responder.Read(third); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("third message under another secret: got %v, want ErrAuthentication", err)
	}
	if responder.Transport() != nil {
		t.Fatal("a failed handshake produced a transport")
	}
}

func TestSessionHandshakeCarriesRequests(t *testing.T) {
	extension, desktop := generateKeyPair(t), generateKeyPair(t)
	initiator, err := SessionInitiator(extension, desktop.Public, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	responder, err := SessionResponder(desktop, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if payload := pass(t, initiator, responder, nil); len(payload) != 0 {
		t.Fatalf("first message carried %q", payload)
	}
	if !bytes.Equal(responder.PeerStatic(), extension.Public) {
		t.Fatal("the desktop app learned the wrong static key")
	}
	if payload := pass(t, responder, initiator, []byte(`{"language":"ru"}`)); string(payload) != `{"language":"ru"}` {
		t.Fatalf("second message carried %q", payload)
	}
	if initiator.Transport() == nil || responder.Transport() == nil {
		t.Fatal("the handshake did not complete after two messages")
	}
	exchange(t, initiator.Transport(), responder.Transport(), `{"id":1,"type":"status"}`)
	exchange(t, responder.Transport(), initiator.Transport(), `{"id":1,"result":{"vault":"locked"}}`)
}

func TestSessionHandshakeFailsTowardsAnotherDesktopKey(t *testing.T) {
	initiator, err := SessionInitiator(generateKeyPair(t), generateKeyPair(t).Public, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	responder, err := SessionResponder(generateKeyPair(t), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	first, err := initiator.Write(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := responder.Read(first); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("first message towards another key: got %v, want ErrAuthentication", err)
	}
	if _, err := SessionInitiator(generateKeyPair(t), []byte("short"), rand.Reader); !errors.Is(err, ErrMalformed) {
		t.Fatalf("a desktop key of the wrong size: got %v", err)
	}
}

func TestHandshakeRefusesShortAndOversizedMessages(t *testing.T) {
	responder, err := SessionResponder(generateKeyPair(t), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := responder.Read(make([]byte, 31)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("a short message: got %v, want ErrMalformed", err)
	}
	if _, err := responder.Read(make([]byte, MaxMessageBytes+1)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("an oversized message: got %v, want ErrMalformed", err)
	}
}

func TestTransportRefusesTamperingAndOversizedMessages(t *testing.T) {
	extension, desktop := generateKeyPair(t), generateKeyPair(t)
	initiator, err := SessionInitiator(extension, desktop.Public, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	responder, err := SessionResponder(desktop, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pass(t, initiator, responder, nil)
	pass(t, responder, initiator, nil)
	sending, receiving := initiator.Transport(), responder.Transport()

	sealed, err := sending.Seal([]byte(`{"id":1,"type":"status"}`))
	if err != nil {
		t.Fatal(err)
	}
	sealed[0] ^= 0x01
	if _, err := receiving.Open(sealed); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("an altered message: got %v, want ErrAuthentication", err)
	}
	if _, err := receiving.Open(make([]byte, MaxMessageBytes+1)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("an oversized message: got %v, want ErrMalformed", err)
	}
	if _, err := sending.Seal(make([]byte, MaxMessageBytes-tagSize+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("an oversized plaintext: got %v, want ErrTooLarge", err)
	}
	if _, err := sending.Seal(make([]byte, MaxMessageBytes-tagSize)); err != nil {
		t.Fatalf("the largest plaintext: %v", err)
	}
}

func TestKeyPairValidity(t *testing.T) {
	key := generateKeyPair(t)
	if !key.Valid() {
		t.Fatal("a generated key pair is invalid")
	}
	other := generateKeyPair(t)
	for name, pair := range map[string]KeyPair{
		"mismatched halves": {Private: key.Private, Public: other.Public},
		"short private key": {Private: key.Private[:31], Public: key.Public},
		"short public key":  {Private: key.Private, Public: key.Public[:31]},
		"empty":             {},
	} {
		if pair.Valid() {
			t.Errorf("%s: reported valid", name)
		}
	}
}

func TestAllowedOrigins(t *testing.T) {
	if !AllowedOrigin("chrome-extension://cceiadaelnccfbakmhcleifjfilkakag") {
		t.Fatal("the extension's origin is refused")
	}
	for _, origin := range []string{"", "null", "https://example.com", "chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "chrome-extension://cceiadaelnccfbakmhcleifjfilkakag/"} {
		if AllowedOrigin(origin) {
			t.Errorf("origin %q is allowed", origin)
		}
	}
}

func TestLinkRequestsNeedAName(t *testing.T) {
	longest := strings.Repeat("é", MaxNameLength)
	request, err := ParseLinkRequest([]byte(`{"name":"` + longest + `"}`))
	if err != nil || request.Name != longest {
		t.Fatalf("a name of %d characters: %+v, %v", MaxNameLength, request, err)
	}
	for _, payload := range []string{
		``, `null`, `[]`, `{}`, `{"name":""}`, `{"name":7}`,
		`{"name":"` + longest + `e"}`,
	} {
		if _, err := ParseLinkRequest([]byte(payload)); !errors.Is(err, ErrMalformed) {
			t.Errorf("payload %q: got %v, want ErrMalformed", payload, err)
		}
	}
}

func TestSessionRequestsNeedAnIDAndAType(t *testing.T) {
	request, err := ParseRequest([]byte(`{"id":7,"type":"status"}`))
	if err != nil || !reflect.DeepEqual(request, Request{ID: 7, Type: RequestStatus}) {
		t.Fatalf("status request = %+v, error = %v", request, err)
	}
	request, err = ParseRequest([]byte(`{"id":0,"type":"fill"}`))
	if err != nil || !reflect.DeepEqual(request, Request{ID: 0, Type: "fill"}) {
		t.Fatalf("unknown request = %+v, error = %v", request, err)
	}
	for _, payload := range []string{
		``, `null`, `{"type":"status"}`, `{"id":null,"type":"status"}`, `{"id":1}`,
		`{"id":1,"type":""}`, `{"id":1.5,"type":"status"}`, `{"id":"1","type":"status"}`,
		`{"id":1,"type":"suggest","origin":"https://example.com","purpose":1}`,
	} {
		if _, err := ParseRequest([]byte(payload)); !errors.Is(err, ErrMalformed) {
			t.Errorf("payload %q: got %v, want ErrMalformed", payload, err)
		}
	}
}

func TestASuggestRequestNamesAPurposeThisProtocolDefines(t *testing.T) {
	tests := []struct {
		payload string
		want    Purpose
		known   bool
	}{
		{`{"id":1,"type":"suggest","origin":"https://example.com"}`, "", false},
		{`{"id":1,"type":"suggest","origin":"https://example.com","purpose":"sign-in"}`, PurposeSignIn, true},
		{`{"id":1,"type":"suggest","origin":"https://example.com","purpose":"code"}`, PurposeCode, true},
		{`{"id":1,"type":"suggest","origin":"https://example.com","purpose":"Code"}`, "", false},
		{`{"id":1,"type":"suggest","origin":"https://example.com","purpose":"password"}`, "", false},
	}
	for _, test := range tests {
		request, err := ParseRequest([]byte(test.payload))
		if err != nil {
			t.Fatalf("payload %q: %v", test.payload, err)
		}
		if purpose, known := request.SuggestPurpose(); purpose != test.want || known != test.known {
			t.Errorf("payload %q: purpose %q, %v; want %q, %v", test.payload, purpose, known, test.want, test.known)
		}
	}
}
