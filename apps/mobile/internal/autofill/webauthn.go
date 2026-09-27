package autofill

import (
	"encoding/json"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/linkproto"
)

// credentialType is the one credential type, WebAuthn §5.8.2.
const credentialType = "public-key"

// placeholderClientData stands for a browser's client data: the browser answers the page with its own.
var placeholderClientData = []byte("{}")

// transports are "internal" for this device and "hybrid" for another device through it, sorted as WebAuthn §5.2.1.1 requires.
var transports = []string{"hybrid", "internal"}

// descriptor is a PublicKeyCredentialDescriptorJSON, WebAuthn §5.10.3.
type descriptor struct {
	Type string          `json:"type"`
	ID   linkproto.Bytes `json:"id"`
}

// credentialIDs skips descriptors of any type but credentialType.
func credentialIDs(listed []descriptor) [][]byte {
	var ids [][]byte
	for _, d := range listed {
		if d.Type == credentialType {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// requestOptions is what a sign-in reads of PublicKeyCredentialRequestOptionsJSON, WebAuthn §5.5.
type requestOptions struct {
	Challenge        linkproto.Bytes           `json:"challenge"`
	RPID             string                    `json:"rpId"`
	AllowCredentials []descriptor              `json:"allowCredentials"`
	UserVerification autofill.UserVerification `json:"userVerification"`
}

// creationOptions is what a creation reads of PublicKeyCredentialCreationOptionsJSON, WebAuthn §5.4.
type creationOptions struct {
	RP struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"rp"`
	User struct {
		ID          linkproto.Bytes `json:"id"`
		Name        string          `json:"name"`
		DisplayName string          `json:"displayName"`
	} `json:"user"`
	Challenge        linkproto.Bytes `json:"challenge"`
	PubKeyCredParams []struct {
		Type string `json:"type"`
		Alg  int    `json:"alg"`
	} `json:"pubKeyCredParams"`
	ExcludeCredentials     []descriptor `json:"excludeCredentials"`
	AuthenticatorSelection struct {
		UserVerification autofill.UserVerification `json:"userVerification"`
	} `json:"authenticatorSelection"`
	Extensions struct {
		CredProps bool `json:"credProps"`
	} `json:"extensions"`
}

// algorithms are the COSE algorithms the relying party accepts.
func (o creationOptions) algorithms() []int {
	var accepted []int
	for _, parameter := range o.PubKeyCredParams {
		if parameter.Type == credentialType {
			accepted = append(accepted, parameter.Alg)
		}
	}
	return accepted
}

// credentialJSON is a PublicKeyCredential as its toJSON() writes it, WebAuthn §5.1.
type credentialJSON struct {
	ID                      linkproto.Bytes `json:"id"`
	RawID                   linkproto.Bytes `json:"rawId"`
	Type                    string          `json:"type"`
	AuthenticatorAttachment string          `json:"authenticatorAttachment"`
	Response                any             `json:"response"`
	ClientExtensionResults  extensionsJSON  `json:"clientExtensionResults"`
}

// extensionsJSON is an AuthenticationExtensionsClientOutputsJSON, WebAuthn §5.8.
type extensionsJSON struct {
	CredProps *credPropsJSON `json:"credProps,omitempty"`
}

// credPropsJSON is a CredentialPropertiesOutput, WebAuthn §10.1.3; every passkey Ravenpass creates is discoverable.
type credPropsJSON struct {
	RK bool `json:"rk"`
}

// assertionJSON is an AuthenticatorAssertionResponseJSON, WebAuthn §5.2.2.
type assertionJSON struct {
	ClientDataJSON    linkproto.Bytes `json:"clientDataJSON"`
	AuthenticatorData linkproto.Bytes `json:"authenticatorData"`
	Signature         linkproto.Bytes `json:"signature"`
	UserHandle        linkproto.Bytes `json:"userHandle,omitempty"`
}

// attestationJSON is an AuthenticatorAttestationResponseJSON, WebAuthn §5.2.1.
type attestationJSON struct {
	ClientDataJSON     linkproto.Bytes `json:"clientDataJSON"`
	AuthenticatorData  linkproto.Bytes `json:"authenticatorData"`
	Transports         []string        `json:"transports"`
	PublicKey          linkproto.Bytes `json:"publicKey"`
	PublicKeyAlgorithm int             `json:"publicKeyAlgorithm"`
	AttestationObject  linkproto.Bytes `json:"attestationObject"`
}

// authenticationJSON is the AuthenticationResponseJSON the relying party receives for signed.
func authenticationJSON(signed autofill.PasskeyAssertion) ([]byte, error) {
	return credentialOf(signed.CredentialID, assertionJSON{
		ClientDataJSON:    clientDataOr(signed.ClientData),
		AuthenticatorData: signed.AuthenticatorData,
		Signature:         signed.Signature,
		UserHandle:        signed.UserHandle,
	}, extensionsJSON{})
}

// registrationJSON is the RegistrationResponseJSON the relying party receives for created; credProps answers that extension.
func registrationJSON(created autofill.CreatedPasskey, credProps bool) ([]byte, error) {
	var extensions extensionsJSON
	if credProps {
		extensions.CredProps = &credPropsJSON{RK: true}
	}
	return credentialOf(created.CredentialID, attestationJSON{
		ClientDataJSON:     clientDataOr(created.ClientData),
		AuthenticatorData:  created.AuthenticatorData,
		Transports:         transports,
		PublicKey:          created.PublicKey,
		PublicKeyAlgorithm: created.Algorithm,
		AttestationObject:  created.AttestationObject,
	}, extensions)
}

func credentialOf(credentialID []byte, response any, extensions extensionsJSON) ([]byte, error) {
	return json.Marshal(credentialJSON{
		ID: credentialID, RawID: credentialID, Type: credentialType, AuthenticatorAttachment: "platform",
		Response: response, ClientExtensionResults: extensions,
	})
}

func clientDataOr(built []byte) []byte {
	if built == nil {
		return placeholderClientData
	}
	return built
}
