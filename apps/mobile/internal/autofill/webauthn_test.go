package autofill

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/linkproto"
)

// sharedCredentials is the WebAuthn JSON table the extension's responses test reads too.
type sharedCredentials struct {
	Registrations []struct {
		Name    string `json:"name"`
		Passkey struct {
			CredentialID       linkproto.Bytes `json:"credentialId"`
			ClientDataJSON     linkproto.Bytes `json:"clientDataJSON"`
			AttestationObject  linkproto.Bytes `json:"attestationObject"`
			AuthenticatorData  linkproto.Bytes `json:"authenticatorData"`
			PublicKey          linkproto.Bytes `json:"publicKey"`
			PublicKeyAlgorithm int             `json:"publicKeyAlgorithm"`
		} `json:"passkey"`
		CredProps bool            `json:"credProps"`
		JSON      json.RawMessage `json:"json"`
	} `json:"registrations"`
	Authentications []struct {
		Name    string `json:"name"`
		Passkey struct {
			CredentialID      linkproto.Bytes `json:"credentialId"`
			ClientDataJSON    linkproto.Bytes `json:"clientDataJSON"`
			AuthenticatorData linkproto.Bytes `json:"authenticatorData"`
			Signature         linkproto.Bytes `json:"signature"`
			UserHandle        linkproto.Bytes `json:"userHandle"`
		} `json:"passkey"`
		JSON json.RawMessage `json:"json"`
	} `json:"authentications"`
}

func TestCredentialJSONIsWhatTheSharedTableSays(t *testing.T) {
	data, err := os.ReadFile("../../../../packages/authenticator/testdata/credentials.json")
	if err != nil {
		t.Fatal(err)
	}
	var table sharedCredentials
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Registrations) == 0 || len(table.Authentications) == 0 {
		t.Fatal("the shared table misses registrations or authentications")
	}
	for _, test := range table.Registrations {
		t.Run(test.Name, func(t *testing.T) {
			p := test.Passkey
			got, err := registrationJSON(autofill.CreatedPasskey{
				CredentialID: p.CredentialID, ClientData: p.ClientDataJSON, AttestationObject: p.AttestationObject,
				AuthenticatorData: p.AuthenticatorData, PublicKey: p.PublicKey, Algorithm: p.PublicKeyAlgorithm,
			}, test.CredProps)
			sameJSON(t, got, err, test.JSON)
		})
	}
	for _, test := range table.Authentications {
		t.Run(test.Name, func(t *testing.T) {
			p := test.Passkey
			got, err := authenticationJSON(autofill.PasskeyAssertion{
				CredentialID: p.CredentialID, ClientData: p.ClientDataJSON, AuthenticatorData: p.AuthenticatorData,
				Signature: p.Signature, UserHandle: p.UserHandle,
			})
			sameJSON(t, got, err, test.JSON)
		})
	}
}

func sameJSON(t *testing.T, got []byte, err error, want json.RawMessage) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON %s, want %s", got, want)
	}
}
