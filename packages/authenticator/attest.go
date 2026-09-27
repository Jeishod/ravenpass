package authenticator

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/binary"
	"errors"

	"github.com/fxamacker/cbor/v2"
)

// ErrInvalidCredentialID reports a credential ID that is empty or longer than WebAuthn §4 allows.
var ErrInvalidCredentialID = errors.New("passkey credential ID has an invalid length")

const maxCredentialIDSize = 1023

// COSE identifiers, RFC 9053 §7.1 and §7.2.
const (
	coseKeyTypeEC2 = 2
	coseCurveP256  = 1
)

// canonical is CTAP2 canonical CBOR, CTAP 2.1 §8: map keys sorted by encoded length, then bytes.
var canonical = func() cbor.EncMode {
	mode, err := cbor.CTAP2EncOptions().EncMode()
	if err != nil {
		panic(err)
	}
	return mode
}()

type coseEC2Key struct {
	KeyType   int    `cbor:"1,keyasint"`
	Algorithm int    `cbor:"3,keyasint"`
	Curve     int    `cbor:"-1,keyasint"`
	X         []byte `cbor:"-2,keyasint"`
	Y         []byte `cbor:"-3,keyasint"`
}

type attestationObject struct {
	Format    string   `cbor:"fmt"`
	Statement struct{} `cbor:"attStmt"`
	AuthData  []byte   `cbor:"authData"`
}

// Attestation is what a registration returns to the page.
type Attestation struct {
	// AttestationObject is {"fmt": "none", "attStmt": {}, "authData": AuthenticatorData} in CTAP2 canonical CBOR.
	AttestationObject []byte
	AuthenticatorData []byte
	// PublicKey is SubjectPublicKeyInfo DER, as getPublicKey() returns it.
	PublicKey []byte
	Algorithm int
}

// Attest builds the registration of a new passkey for rpID with a zero signature counter and Ravenpass's AAGUID.
func (p Profile) Attest(rpID string, credentialID, privateKey []byte, verified bool) (Attestation, error) {
	if len(credentialID) == 0 || len(credentialID) > maxCredentialIDSize {
		return Attestation{}, ErrInvalidCredentialID
	}
	key, err := parseKey(privateKey)
	if err != nil {
		return Attestation{}, err
	}
	attested, err := attestedCredentialData(AAGUID, credentialID, &key.PublicKey)
	if err != nil {
		return Attestation{}, err
	}
	authData := authenticatorData(rpID, p.flags(verified)|flagAttestedData, 0, attested)
	object, err := canonical.Marshal(attestationObject{Format: "none", AuthData: authData})
	if err != nil {
		return Attestation{}, err
	}
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return Attestation{}, err
	}
	return Attestation{
		AttestationObject: object,
		AuthenticatorData: authData,
		PublicKey:         publicKey,
		Algorithm:         AlgorithmES256,
	}, nil
}

// attestedCredentialData lays out WebAuthn §6.5.1: AAGUID, big-endian uint16 credential ID length, credential ID, COSE public key.
func attestedCredentialData(aaguid [16]byte, credentialID []byte, publicKey *ecdsa.PublicKey) ([]byte, error) {
	point, err := publicKey.Bytes()
	if err != nil {
		return nil, err
	}
	// point is the SEC 1 uncompressed form: 0x04, then X and Y of equal length.
	size := (len(point) - 1) / 2
	coseKey, err := canonical.Marshal(coseEC2Key{
		KeyType:   coseKeyTypeEC2,
		Algorithm: AlgorithmES256,
		Curve:     coseCurveP256,
		X:         point[1 : 1+size],
		Y:         point[1+size:],
	})
	if err != nil {
		return nil, err
	}
	data := make([]byte, 0, len(aaguid)+2+len(credentialID)+len(coseKey))
	data = append(data, aaguid[:]...)
	data = binary.BigEndian.AppendUint16(data, uint16(len(credentialID)))
	data = append(data, credentialID...)
	return append(data, coseKey...), nil
}
