package vault

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/asn1"
	"math"
	"slices"
	"strings"
	"time"
)

const (
	minCredentialIDBytes = 16
	maxCredentialIDBytes = 1023
	maxUserHandleBytes   = 64
	maxPasskeyKeyBytes   = 512
)

// A passkey in a record is [credentialID, rpID, userHandle, userName, userDisplayName, privateKey, counter, discoverable, createdAt].
type wirePasskey struct {
	_               struct{} `cbor:",toarray"`
	CredentialID    []byte
	RPID            string
	UserHandle      []byte
	UserName        string
	UserDisplayName string
	PrivateKey      []byte
	Counter         uint32
	Discoverable    flag
	CreatedAt       uint64
}

type wirePasskeyFace struct {
	_               struct{} `cbor:",toarray"`
	CredentialID    []byte
	RPID            string
	UserName        string
	Discoverable    flag
	UserHandle      []byte
	UserDisplayName string
}

// Passkey is a WebAuthn credential for one relying party; only Session.ReadPasskey returns its private key.
type Passkey struct {
	CredentialID []byte
	// RPID is a host name in the form HostASCII gives it.
	RPID            string
	UserHandle      []byte
	UserName        string
	UserDisplayName string
	// PrivateKey is an ECDSA P-256 key in PKCS #8.
	PrivateKey   []byte
	Counter      uint32
	Discoverable bool
	// CreatedAt is kept in whole seconds, no earlier than the Unix epoch, and read back in UTC.
	CreatedAt time.Time
}

// PasskeyFace is what the index shows of a passkey without decrypting its record.
type PasskeyFace struct {
	CredentialID    []byte
	RPID            string
	UserHandle      []byte
	UserName        string
	UserDisplayName string
	Discoverable    bool
}

// validRelyingPartyID reports a host name in HostASCII form without a scheme, port, path or empty label.
func validRelyingPartyID(rpID string) bool {
	if rpID == "" || len(rpID) > MaxOriginLength || slices.Contains(strings.Split(rpID, "."), "") {
		return false
	}
	ascii, ok := HostASCII(rpID)
	return ok && ascii == rpID
}

func validPasskeyFace(face PasskeyFace) bool {
	return len(face.CredentialID) >= minCredentialIDBytes && len(face.CredentialID) <= maxCredentialIDBytes &&
		validRelyingPartyID(face.RPID) && len(face.UserHandle) >= 1 && len(face.UserHandle) <= maxUserHandleBytes &&
		fits(face.UserName, MaxLoginLength) && fits(face.UserDisplayName, MaxLoginLength)
}

// validPasskey reports a passkey within every limit, without parsing its key.
func validPasskey(passkey Passkey) bool {
	return validPasskeyFace(passkey.face()) &&
		len(passkey.PrivateKey) >= 1 && len(passkey.PrivateKey) <= maxPasskeyKeyBytes &&
		passkey.CreatedAt.Unix() >= 0
}

// face is what the index shows of the passkey, holding no memory the passkey holds.
func (p Passkey) face() PasskeyFace {
	return PasskeyFace{
		CredentialID: bytes.Clone(p.CredentialID), RPID: p.RPID, UserHandle: bytes.Clone(p.UserHandle),
		UserName: p.UserName, UserDisplayName: p.UserDisplayName, Discoverable: p.Discoverable,
	}
}

// p256Key reports an ECDSA P-256 key in PKCS #8 with no trailing bytes, which the PKCS #8 parser ignores.
func p256Key(der []byte) bool {
	var outer asn1.RawValue
	if rest, err := asn1.Unmarshal(der, &outer); err != nil || len(rest) != 0 {
		return false
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return false
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	return ok && ecdsaKey.Curve == elliptic.P256()
}

// ValidPasskey reports whether a credential can take passkey, apart from a repeated credential ID.
func ValidPasskey(passkey Passkey) bool {
	return validPasskey(passkey) && p256Key(passkey.PrivateKey)
}

// passkeyIndex is the position of the passkey with credentialID, or -1 when none has it.
func passkeyIndex(passkeys []Passkey, credentialID []byte) int {
	return slices.IndexFunc(passkeys, func(passkey Passkey) bool { return bytes.Equal(passkey.CredentialID, credentialID) })
}

// acceptPasskeys returns a credential's passkeys as stored, creation times truncated to whole seconds.
func acceptPasskeys(passkeys []Passkey) ([]Passkey, error) {
	if len(passkeys) > MaxCredentialPasskeys {
		return nil, ErrPasskeysFull
	}
	accepted := nilIfEmpty(slices.Clone(passkeys))
	for i := range accepted {
		accepted[i].CreatedAt = time.Unix(accepted[i].CreatedAt.Unix(), 0).UTC()
		if !ValidPasskey(accepted[i]) || passkeyIndex(accepted[:i], accepted[i].CredentialID) >= 0 {
			return nil, ErrInvalidInput
		}
	}
	return accepted, nil
}

// passkeyFaces are the faces of passkeys, holding no memory the passkeys hold; nil for none.
func passkeyFaces(passkeys []Passkey) []PasskeyFace {
	if len(passkeys) == 0 {
		return nil
	}
	faces := make([]PasskeyFace, len(passkeys))
	for i, passkey := range passkeys {
		faces[i] = passkey.face()
	}
	return faces
}

// clonedFaces copies faces, holding no memory they hold; nil for none.
func clonedFaces(faces []PasskeyFace) []PasskeyFace {
	if len(faces) == 0 {
		return nil
	}
	cloned := make([]PasskeyFace, len(faces))
	for i, face := range faces {
		face.CredentialID = bytes.Clone(face.CredentialID)
		face.UserHandle = bytes.Clone(face.UserHandle)
		cloned[i] = face
	}
	return cloned
}

// forgetPasskeyKeys clears each passkey's private key and drops it.
func forgetPasskeyKeys(passkeys []Passkey) {
	for i := range passkeys {
		clear(passkeys[i].PrivateKey)
		passkeys[i].PrivateKey = nil
	}
}

// wirePasskeys is the passkey list of a record. It shares each private key with passkeys.
func wirePasskeys(passkeys []Passkey) []wirePasskey {
	wires := make([]wirePasskey, len(passkeys))
	for i, passkey := range passkeys {
		wires[i] = wirePasskey{
			CredentialID: passkey.CredentialID, RPID: passkey.RPID, UserHandle: passkey.UserHandle,
			UserName: passkey.UserName, UserDisplayName: passkey.UserDisplayName, PrivateKey: passkey.PrivateKey,
			Counter: passkey.Counter, Discoverable: flag(passkey.Discoverable), CreatedAt: uint64(passkey.CreatedAt.Unix()),
		}
	}
	return wires
}

// forgetWirePasskeys clears the private key of each passkey of a record.
func forgetWirePasskeys(wires []wirePasskey) {
	for i := range wires {
		clear(wires[i].PrivateKey)
		wires[i].PrivateKey = nil
	}
}

// parsePasskeys reads a record's passkeys, refusing a repeated credential ID; each takes over its private key from wires.
func parsePasskeys(wires []wirePasskey) ([]Passkey, error) {
	if len(wires) > MaxCredentialPasskeys {
		return nil, ErrMalformed
	}
	passkeys := make([]Passkey, len(wires))
	for i, wire := range wires {
		if wire.CreatedAt > math.MaxInt64 {
			return nil, ErrMalformed
		}
		passkeys[i] = Passkey{
			CredentialID: wire.CredentialID, RPID: wire.RPID, UserHandle: wire.UserHandle,
			UserName: wire.UserName, UserDisplayName: wire.UserDisplayName, PrivateKey: wire.PrivateKey,
			Counter: wire.Counter, Discoverable: bool(wire.Discoverable), CreatedAt: time.Unix(int64(wire.CreatedAt), 0).UTC(),
		}
		if !validPasskey(passkeys[i]) || passkeyIndex(passkeys[:i], wire.CredentialID) >= 0 {
			return nil, ErrMalformed
		}
	}
	return nilIfEmpty(passkeys), nil
}

func wirePasskeyFaces(faces []PasskeyFace) []wirePasskeyFace {
	wires := make([]wirePasskeyFace, len(faces))
	for i, face := range faces {
		wires[i] = wirePasskeyFace{
			CredentialID: face.CredentialID, RPID: face.RPID, UserName: face.UserName,
			Discoverable: flag(face.Discoverable), UserHandle: face.UserHandle, UserDisplayName: face.UserDisplayName,
		}
	}
	return wires
}

// parsePasskeyFaces reads an entry's passkey faces, refusing a repeated credential ID or any face outside a credential.
func parsePasskeyFaces(wires []wirePasskeyFace, kind Kind) ([]PasskeyFace, error) {
	if len(wires) > MaxCredentialPasskeys || kind != KindCredential && len(wires) != 0 {
		return nil, ErrMalformed
	}
	faces := make([]PasskeyFace, len(wires))
	for i, wire := range wires {
		faces[i] = PasskeyFace{
			CredentialID: wire.CredentialID, RPID: wire.RPID, UserHandle: wire.UserHandle,
			UserName: wire.UserName, UserDisplayName: wire.UserDisplayName, Discoverable: bool(wire.Discoverable),
		}
		if !validPasskeyFace(faces[i]) || slices.ContainsFunc(faces[:i], func(other PasskeyFace) bool { return bytes.Equal(other.CredentialID, wire.CredentialID) }) {
			return nil, ErrMalformed
		}
	}
	return nilIfEmpty(faces), nil
}

// preparePasskeyChange prepares the credential at index with input's changed passkeys, keeping its membership.
func (s *Session) preparePasskeyChange(index int, input CredentialInput) (*Pending, error) {
	accepted, err := acceptInput(input)
	if err != nil {
		return nil, err
	}
	plaintext, err := encodeCredentialRecord(accepted)
	if err != nil {
		return nil, err
	}
	defer clear(plaintext)
	return s.prepareReplace(index, credentialEntry(accepted, s.entries[index].groups), plaintext)
}

// PrepareAddPasskey prepares the credential with id holding passkey after those it holds.
func (s *Session) PrepareAddPasskey(id ID, passkey Passkey) (*Pending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, input, err := s.credentialToChange(id)
	if err != nil {
		return nil, err
	}
	defer forgetPasskeyKeys(input.Passkeys)
	if len(input.Passkeys) >= MaxCredentialPasskeys {
		return nil, ErrPasskeysFull
	}
	input.Passkeys = append(slices.Clip(input.Passkeys), passkey)
	return s.preparePasskeyChange(index, input)
}

// withoutPasskeys returns a copy of passkeys without the credential IDs in removed.
func withoutPasskeys(passkeys []Passkey, removed [][]byte) ([]Passkey, error) {
	kept := slices.Clone(passkeys)
	for _, credentialID := range removed {
		position := passkeyIndex(kept, credentialID)
		if position < 0 {
			return nil, ErrNotFound
		}
		kept = slices.Delete(kept, position, position+1)
	}
	return kept, nil
}

// PrepareCountPasskeyUse advances a nonzero passkey counter and returns it with its save; a zero counter returns 0 and no save.
func (s *Session) PrepareCountPasskeyUse(id ID, credentialID []byte) (uint32, *Pending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, input, err := s.credentialToChange(id)
	if err != nil {
		return 0, nil, err
	}
	defer forgetPasskeyKeys(input.Passkeys)
	position := passkeyIndex(input.Passkeys, credentialID)
	if position < 0 {
		return 0, nil, ErrNotFound
	}
	counter := input.Passkeys[position].Counter
	if counter == 0 {
		return 0, nil, nil
	}
	if counter == math.MaxUint32 {
		return 0, nil, ErrResourceLimit
	}
	input.Passkeys = slices.Clone(input.Passkeys)
	input.Passkeys[position].Counter = counter + 1
	pending, err := s.preparePasskeyChange(index, input)
	if err != nil {
		return 0, nil, err
	}
	return counter + 1, pending, nil
}

// ReadPasskey returns a credential's passkey with its private key, which the caller must clear.
func (s *Session) ReadPasskey(id ID, credentialID []byte) (Passkey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return Passkey{}, ErrLocked
	}
	index := s.findKind(id, KindCredential)
	if index < 0 {
		return Passkey{}, ErrNotFound
	}
	input, err := s.decryptCredential(index)
	if err != nil {
		return Passkey{}, err
	}
	defer forgetPasskeyKeys(input.Passkeys)
	position := passkeyIndex(input.Passkeys, credentialID)
	if position < 0 {
		return Passkey{}, ErrNotFound
	}
	passkey := input.Passkeys[position]
	passkey.PrivateKey = bytes.Clone(passkey.PrivateKey)
	return passkey, nil
}
