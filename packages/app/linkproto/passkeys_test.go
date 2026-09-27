package linkproto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func encoded(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func parsed(t *testing.T, payload string) Request {
	t.Helper()
	request, err := ParseRequest([]byte(payload))
	if err != nil {
		t.Fatalf("%s: %v", payload, err)
	}
	return request
}

func TestAPasskeysRequestCarriesItsModeAndLists(t *testing.T) {
	first, second := bytes.Repeat([]byte{1}, 16), bytes.Repeat([]byte{2}, 32)
	request := parsed(t, `{"id":1,"type":"passkeys","origin":"https://login.example.com","rpId":"example.com","mode":"get","allow":["`+encoded(first)+`","`+encoded(second)+`"]}`)
	query, valid := request.PasskeyQuery()
	want := PasskeyQuery{Origin: "https://login.example.com", RPID: "example.com", Mode: PasskeyGet, Allow: [][]byte{first, second}, Exclude: [][]byte{}}
	if !valid || !reflect.DeepEqual(query, want) {
		t.Fatalf("query = %+v, want %+v", query, want)
	}
	request = parsed(t, `{"id":2,"type":"passkeys","origin":"https://example.com","mode":"create","exclude":["`+encoded(first)+`"],"account":"alex"}`)
	query, valid = request.PasskeyQuery()
	want = PasskeyQuery{Origin: "https://example.com", Mode: PasskeyCreate, Allow: [][]byte{}, Exclude: [][]byte{first}, Account: "alex"}
	if !valid || !reflect.DeepEqual(query, want) {
		t.Fatalf("query = %+v, want %+v", query, want)
	}
	tooMany := make([]string, MaxCredentialIDs+1)
	for i := range tooMany {
		tooMany[i] = `"AQ"`
	}
	for name, payload := range map[string]string{
		"no mode":           `{"id":1,"type":"passkeys","origin":"https://example.com"}`,
		"an unknown mode":   `{"id":1,"type":"passkeys","origin":"https://example.com","mode":"conditional"}`,
		"an empty ID":       `{"id":1,"type":"passkeys","origin":"https://example.com","mode":"get","allow":[""]}`,
		"a long ID":         `{"id":1,"type":"passkeys","origin":"https://example.com","mode":"get","allow":["` + encoded(make([]byte, MaxCredentialIDBytes+1)) + `"]}`,
		"too many IDs":      `{"id":1,"type":"passkeys","origin":"https://example.com","mode":"create","exclude":[` + strings.Join(tooMany, ",") + `]}`,
		"a long account":    `{"id":1,"type":"passkeys","origin":"https://example.com","mode":"create","account":"` + strings.Repeat("a", MaxPasskeyNameLength+1) + `"}`,
		"a long relying ID": `{"id":1,"type":"passkeys","origin":"https://example.com","mode":"get","rpId":"` + strings.Repeat("a", MaxOriginLength+1) + `"}`,
	} {
		if _, valid := parsed(t, payload).PasskeyQuery(); valid {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestAPasskeyCreateRequestCarriesTheUserAndChallenge(t *testing.T) {
	challenge, handle := bytes.Repeat([]byte{7}, 32), []byte{1, 2, 3}
	request := parsed(t, `{"id":3,"type":"passkey-create","origin":"https://example.com","rpId":"example.com","rpName":"Example",`+
		`"user":{"id":"`+encoded(handle)+`","name":"alex@example.com","displayName":"Alex"},"challenge":"`+encoded(challenge)+`",`+
		`"verify":true,"target":"new","exclude":[],"algorithms":[-8,-7]}`)
	creation, valid := request.PasskeyCreation()
	want := PasskeyCreation{
		Origin: "https://example.com", RPID: "example.com", RPName: "Example",
		User:      PasskeyUser{ID: handle, Name: "alex@example.com", DisplayName: "Alex"},
		Challenge: challenge, Verify: true, Target: TargetNew, Exclude: [][]byte{},
	}
	if !valid || !reflect.DeepEqual(creation, want) {
		t.Fatalf("creation = %+v, want %+v", creation, want)
	}
	base := `{"id":1,"type":"passkey-create","origin":"https://example.com","target":"new",`
	for name, payload := range map[string]string{
		"no challenge":     base + `"user":{"id":"AQ"}}`,
		"a long challenge": base + `"user":{"id":"AQ"},"challenge":"` + encoded(make([]byte, MaxChallengeBytes+1)) + `"}`,
		"no user handle":   base + `"challenge":"AQ"}`,
		"a long handle":    base + `"challenge":"AQ","user":{"id":"` + encoded(make([]byte, MaxUserHandleBytes+1)) + `"}}`,
		"a long name":      base + `"challenge":"AQ","user":{"id":"AQ","name":"` + strings.Repeat("n", MaxPasskeyNameLength+1) + `"}}`,
		"a long site name": base + `"challenge":"AQ","user":{"id":"AQ"},"rpName":"` + strings.Repeat("n", MaxPasskeyNameLength+1) + `"}`,
		"no target":        `{"id":1,"type":"passkey-create","origin":"https://example.com","challenge":"AQ","user":{"id":"AQ"}}`,
		"no ES256":         base + `"challenge":"AQ","user":{"id":"AQ"},"algorithms":[-257,-8]}`,
	} {
		if _, valid := parsed(t, payload).PasskeyCreation(); valid {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestAPasskeySignRequestNamesTheCredentialAndPasskey(t *testing.T) {
	challenge, id := bytes.Repeat([]byte{7}, 32), bytes.Repeat([]byte{9}, 16)
	request := parsed(t, `{"id":4,"type":"passkey-sign","origin":"https://example.com","rpId":"example.com","challenge":"`+encoded(challenge)+`",`+
		`"verify":false,"credential":"0102","credentialId":"`+encoded(id)+`"}`)
	signIn, valid := request.PasskeySignIn()
	want := PasskeySignIn{Origin: "https://example.com", RPID: "example.com", Challenge: challenge, Credential: "0102", CredentialID: id}
	if !valid || !reflect.DeepEqual(signIn, want) {
		t.Fatalf("sign-in = %+v, want %+v", signIn, want)
	}
	for name, payload := range map[string]string{
		"no passkey":   `{"id":1,"type":"passkey-sign","origin":"https://example.com","challenge":"AQ","credential":"0102"}`,
		"no challenge": `{"id":1,"type":"passkey-sign","origin":"https://example.com","credential":"0102","credentialId":"AQ"}`,
	} {
		if _, valid := parsed(t, payload).PasskeySignIn(); valid {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestPasskeyBytesAreUnpaddedBase64URL(t *testing.T) {
	for _, payload := range []string{
		`{"id":1,"type":"passkey-sign","challenge":"AQ=="}`,
		`{"id":1,"type":"passkey-sign","challenge":"a+b/"}`,
		`{"id":1,"type":"passkey-sign","challenge":7}`,
		`{"id":1,"type":"passkeys","allow":"AQ"}`,
		`{"id":1,"type":"passkey-create","user":"alex"}`,
		`{"id":1,"type":"passkey-create","verify":"true"}`,
		`{"id":1,"type":"passkey-create","algorithms":[-7.5]}`,
	} {
		if _, err := ParseRequest([]byte(payload)); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", payload, err)
		}
	}
	result, err := json.Marshal(PasskeyAssertion{CredentialID: []byte{0xfb, 0xff}, ClientDataJSON: []byte("{}"), AuthenticatorData: []byte{1}, Signature: []byte{2}})
	want := `{"credentialId":"-_8","clientDataJSON":"e30","authenticatorData":"AQ","signature":"Ag","userHandle":""}`
	if err != nil || string(result) != want {
		t.Fatalf("assertion = %s, error = %v", result, err)
	}
	created, err := json.Marshal(CreatedPasskey{Credential: "0102", PublicKeyAlgorithm: -7})
	want = `{"credential":"0102","credentialId":"","clientDataJSON":"","attestationObject":"","authenticatorData":"","publicKey":"","publicKeyAlgorithm":-7}`
	if err != nil || string(created) != want {
		t.Fatalf("created = %s, error = %v", created, err)
	}
}
