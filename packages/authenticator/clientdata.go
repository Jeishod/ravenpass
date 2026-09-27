package authenticator

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

// Ceremony is the type member of client data, WebAuthn §5.8.1.
type Ceremony string

// The ceremonies client data names.
const (
	Create Ceremony = "webauthn.create"
	Get    Ceremony = "webauthn.get"
)

// clientData fields are declared in the order Chrome serializes them, WebAuthn §5.8.1.1.
type clientData struct {
	Type        Ceremony `json:"type"`
	Challenge   string   `json:"challenge"`
	Origin      string   `json:"origin"`
	CrossOrigin bool     `json:"crossOrigin"`
}

// ClientData returns Chrome's clientDataJSON for a top-level document, escaping only quotes, backslashes and controls as CCDToString does.
func ClientData(ceremony Ceremony, challenge []byte, origin string) []byte {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(clientData{
		Type:      ceremony,
		Challenge: base64.RawURLEncoding.EncodeToString(challenge),
		Origin:    origin,
	})
	if err != nil {
		// Encoding three strings and a bool has no failure path in encoding/json.
		panic(err)
	}
	return bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))
}
