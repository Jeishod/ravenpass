package linkproto

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/linkkey"
)

var update = flag.Bool("update", false, "rewrite testdata/vectors.json")

// The extension's key parser and Noise tests replay this file and must agree with it.
var vectorsPath = filepath.Join("testdata", "vectors.json")

type vectorFile struct {
	Keys       keyVectors        `json:"keys"`
	Handshakes []handshakeVector `json:"handshakes"`
}

type keyVectors struct {
	Valid   []validKeyVector   `json:"valid"`
	Invalid []invalidKeyVector `json:"invalid"`
}

type validKeyVector struct {
	Key    string `json:"key"`
	Port   int    `json:"port"`
	Secret string `json:"secret"`
}

type invalidKeyVector struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type keyPairVector struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

type handshakeVector struct {
	Protocol           string          `json:"protocol"`
	Prologue           string          `json:"prologue"`
	PSK                *string         `json:"psk"`
	InitiatorStatic    keyPairVector   `json:"initiatorStatic"`
	InitiatorEphemeral keyPairVector   `json:"initiatorEphemeral"`
	ResponderStatic    keyPairVector   `json:"responderStatic"`
	ResponderEphemeral keyPairVector   `json:"responderEphemeral"`
	HandshakeMessages  int             `json:"handshakeMessages"`
	HandshakeHash      string          `json:"handshakeHash"`
	Messages           []messageVector `json:"messages"`
}

type messageVector struct {
	Sender     string `json:"sender"`
	Payload    string `json:"payload"`
	Ciphertext string `json:"ciphertext"`
}

const (
	initiator = "initiator"
	responder = "responder"
)

// step is one message of a transcript: who sends it and what it carries.
type step struct {
	sender  string
	payload string
}

// fixedBytes derives 32 bytes from a label, so every vector is reproducible.
func fixedBytes(label string) []byte {
	sum := sha256.Sum256([]byte("ravenpass-link/1 vectors: " + label))
	return sum[:]
}

func fixedKeyPair(t *testing.T, label string) KeyPair {
	t.Helper()
	key, err := GenerateKeyPair(bytes.NewReader(fixedBytes(label)))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func keyPairHex(key KeyPair) keyPairVector {
	return keyPairVector{Private: hex.EncodeToString(key.Private), Public: hex.EncodeToString(key.Public)}
}

func encodeKey(t *testing.T, port uint16, secret []byte) string {
	t.Helper()
	key := linkkey.Key{Port: port}
	copy(key.Secret[:], secret)
	text, err := key.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// resealed returns a valid key altered by change, with a checksum over the altered bytes.
func resealed(t *testing.T, text string, change func([]byte)) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	change(raw)
	sum := sha256.Sum256(raw[:38])
	copy(raw[38:], sum[:4])
	return base64.RawURLEncoding.EncodeToString(raw)
}

func buildKeyVectors(t *testing.T) keyVectors {
	t.Helper()
	var vectors keyVectors
	for _, valid := range []struct {
		port  uint16
		label string
	}{{53117, "key 1"}, {1024, "key 2"}, {65535, "key 3"}} {
		secret := fixedBytes(valid.label)
		vectors.Valid = append(vectors.Valid, validKeyVector{
			Key: encodeKey(t, valid.port, secret), Port: int(valid.port), Secret: hex.EncodeToString(secret),
		})
	}
	base := vectors.Valid[0].Key
	raw, err := base64.RawURLEncoding.DecodeString(base)
	if err != nil {
		t.Fatal(err)
	}
	alteredChecksum := bytes.Clone(raw)
	alteredChecksum[41] ^= 0x01
	alteredSecret := bytes.Clone(raw)
	alteredSecret[6] ^= 0x01
	vectors.Invalid = []invalidKeyVector{
		{Key: "", Reason: "length"},
		{Key: base[:55], Reason: "length"},
		{Key: base + "A", Reason: "length"},
		{Key: base64.RawURLEncoding.EncodeToString(raw[:41]) + "=", Reason: "encoding"},
		{Key: "+" + base[1:], Reason: "encoding"},
		{Key: "/" + base[1:], Reason: "encoding"},
		{Key: "!" + base[1:], Reason: "encoding"},
		{Key: resealed(t, base, func(raw []byte) { raw[0] = 0x9b }), Reason: "header"},
		{Key: resealed(t, base, func(raw []byte) { raw[2] = 0x14 }), Reason: "header"},
		{Key: resealed(t, base, func(raw []byte) { raw[3] = 0 }), Reason: "version"},
		{Key: resealed(t, base, func(raw []byte) { raw[3] = 2 }), Reason: "version"},
		{Key: base64.RawURLEncoding.EncodeToString(alteredChecksum), Reason: "checksum"},
		{Key: base64.RawURLEncoding.EncodeToString(alteredSecret), Reason: "checksum"},
		{Key: resealed(t, base, func(raw []byte) { raw[4], raw[5] = 0x03, 0xff }), Reason: "port"},
		{Key: resealed(t, base, func(raw []byte) { raw[4], raw[5] = 0, 0 }), Reason: "port"},
	}
	return vectors
}

// transcript runs a handshake and its transport messages, checking each arrives intact.
func transcript(t *testing.T, sides map[string]*Handshake, handshakeMessages int, steps []step) ([]messageVector, []byte) {
	t.Helper()
	receiverOf := map[string]string{initiator: responder, responder: initiator}
	messages := make([]messageVector, 0, len(steps))
	for i, next := range steps {
		sender, receiver := sides[next.sender], sides[receiverOf[next.sender]]
		var message, received []byte
		var err error
		if i < handshakeMessages {
			if message, err = sender.Write([]byte(next.payload)); err != nil {
				t.Fatal(err)
			}
			if received, err = receiver.Read(message); err != nil {
				t.Fatal(err)
			}
		} else {
			if message, err = sender.Transport().Seal([]byte(next.payload)); err != nil {
				t.Fatal(err)
			}
			if received, err = receiver.Transport().Open(message); err != nil {
				t.Fatal(err)
			}
		}
		if string(received) != next.payload {
			t.Fatalf("message %d arrived as %q, want %q", i+1, received, next.payload)
		}
		messages = append(messages, messageVector{
			Sender: next.sender, Payload: hex.EncodeToString([]byte(next.payload)), Ciphertext: hex.EncodeToString(message),
		})
	}
	hash := sides[initiator].state.ChannelBinding()
	if !bytes.Equal(hash, sides[responder].state.ChannelBinding()) {
		t.Fatal("the sides ended with different handshake hashes")
	}
	return messages, bytes.Clone(hash)
}

func buildLinkVector(t *testing.T) handshakeVector {
	t.Helper()
	psk := fixedBytes("XXpsk3 psk")
	initiatorStatic, initiatorEphemeral := fixedKeyPair(t, "XXpsk3 initiator static"), fixedKeyPair(t, "XXpsk3 initiator ephemeral")
	responderStatic, responderEphemeral := fixedKeyPair(t, "XXpsk3 responder static"), fixedKeyPair(t, "XXpsk3 responder ephemeral")
	extension, err := LinkInitiator(initiatorStatic, psk, bytes.NewReader(initiatorEphemeral.Private))
	if err != nil {
		t.Fatal(err)
	}
	desktop, err := LinkResponder(responderStatic, psk, bytes.NewReader(responderEphemeral.Private))
	if err != nil {
		t.Fatal(err)
	}
	messages, hash := transcript(t, map[string]*Handshake{initiator: extension, responder: desktop}, 3, []step{
		{initiator, ""},
		{responder, ""},
		{initiator, `{"name":"Chrome · macOS"}`},
		{responder, `{"type":"linked","language":"ru","signIn":"field"}`},
		{initiator, ""},
		{responder, "second responder transport message"},
		{initiator, "second initiator transport message"},
	})
	pskHex := hex.EncodeToString(psk)
	return handshakeVector{
		Protocol:           "Noise_XXpsk3_25519_AESGCM_SHA256",
		Prologue:           hex.EncodeToString(prologue),
		PSK:                &pskHex,
		InitiatorStatic:    keyPairHex(initiatorStatic),
		InitiatorEphemeral: keyPairHex(initiatorEphemeral),
		ResponderStatic:    keyPairHex(responderStatic),
		ResponderEphemeral: keyPairHex(responderEphemeral),
		HandshakeMessages:  3,
		HandshakeHash:      hex.EncodeToString(hash),
		Messages:           messages,
	}
}

func buildSessionVector(t *testing.T) handshakeVector {
	t.Helper()
	initiatorStatic, initiatorEphemeral := fixedKeyPair(t, "IK initiator static"), fixedKeyPair(t, "IK initiator ephemeral")
	responderStatic, responderEphemeral := fixedKeyPair(t, "IK responder static"), fixedKeyPair(t, "IK responder ephemeral")
	extension, err := SessionInitiator(initiatorStatic, responderStatic.Public, bytes.NewReader(initiatorEphemeral.Private))
	if err != nil {
		t.Fatal(err)
	}
	desktop, err := SessionResponder(responderStatic, bytes.NewReader(responderEphemeral.Private))
	if err != nil {
		t.Fatal(err)
	}
	messages, hash := transcript(t, map[string]*Handshake{initiator: extension, responder: desktop}, 2, []step{
		{initiator, ""},
		{responder, `{"language":"ru","signIn":"card"}`},
		{initiator, `{"id":1,"type":"status"}`},
		{responder, `{"id":1,"result":{"vault":"unlocked"}}`},
		{initiator, `{"id":2,"type":"unlink"}`},
		{responder, `{"id":2,"result":{}}`},
	})
	return handshakeVector{
		Protocol:           "Noise_IK_25519_AESGCM_SHA256",
		Prologue:           hex.EncodeToString(prologue),
		InitiatorStatic:    keyPairHex(initiatorStatic),
		InitiatorEphemeral: keyPairHex(initiatorEphemeral),
		ResponderStatic:    keyPairHex(responderStatic),
		ResponderEphemeral: keyPairHex(responderEphemeral),
		HandshakeMessages:  2,
		HandshakeHash:      hex.EncodeToString(hash),
		Messages:           messages,
	}
}

func encodeVectors(t *testing.T, vectors vectorFile) []byte {
	t.Helper()
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(vectors); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestVectorsAreCurrent(t *testing.T) {
	vectors := vectorFile{
		Keys:       buildKeyVectors(t),
		Handshakes: []handshakeVector{buildLinkVector(t), buildSessionVector(t)},
	}
	encoded := encodeVectors(t, vectors)
	if *update {
		if err := os.MkdirAll(filepath.Dir(vectorsPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(vectorsPath, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	stored, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatalf("read %s: %v; run go test ./internal/linkproto -update", vectorsPath, err)
	}
	if !bytes.Equal(stored, encoded) {
		t.Fatalf("%s is stale; run go test ./internal/linkproto -update", vectorsPath)
	}
}
