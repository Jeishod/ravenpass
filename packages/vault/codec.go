package vault

import (
	"bytes"
	"errors"
	"unicode/utf8"

	"github.com/fxamacker/cbor/v2"
)

// Every vault structure is a CBOR array in RFC 8949 core deterministic encoding with text as byte strings, and reads only so.
const (
	// An index entry's passkey face is the deepest structure: index, entries, entry, faces, face.
	maxNesting = 5
	// The fewest map pairs the decoder allows; the vault writes no map.
	minMapPairs = 16
)

var (
	encoding   = newEncMode()
	decoding   = newDecMode()
	emptyArray = mustMarshal([]uint64{})
)

func newEncMode() cbor.UserBufferEncMode {
	options := cbor.CoreDetEncOptions()
	options.String = cbor.StringToByteString
	options.NilContainers = cbor.NilContainerAsEmpty
	mode, err := options.UserBufferEncMode()
	if err != nil {
		panic(err)
	}
	return mode
}

func newDecMode() cbor.DecMode {
	mode, err := cbor.DecOptions{
		DupMapKey:          cbor.DupMapKeyEnforcedAPF,
		IndefLength:        cbor.IndefLengthForbidden,
		TagsMd:             cbor.TagsForbidden,
		MaxNestedLevels:    maxNesting,
		MaxArrayElements:   maxEntries,
		MaxMapPairs:        minMapPairs,
		ByteStringToString: cbor.ByteStringToStringAllowed,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return mode
}

// mustMarshal encodes a value of a fixed shape, which cannot fail.
func mustMarshal(value any) []byte {
	encoded, err := encoding.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

// marshal encodes value into a capacity-byte buffer the caller clears; outgrowing capacity leaves uncleared copies in released memory.
func marshal(value any, capacity int) ([]byte, error) {
	buffer := bytes.NewBuffer(make([]byte, 0, capacity))
	if err := encoding.MarshalToBuffer(value, buffer); err != nil {
		clear(buffer.Bytes())
		return nil, err
	}
	return buffer.Bytes(), nil
}

// unmarshal decodes only a canonical encoding of value; on failure value may hold secrets from data that the caller clears.
func unmarshal(data []byte, value any) error {
	if err := decoding.Unmarshal(data, value); err != nil {
		return decodeError(err)
	}
	canonical, err := marshal(value, len(data))
	defer clear(canonical)
	if err != nil || !bytes.Equal(canonical, data) {
		return ErrMalformed
	}
	return nil
}

func decodeError(err error) error {
	var arrayErr *cbor.MaxArrayElementsError
	var mapErr *cbor.MaxMapPairsError
	var nestingErr *cbor.MaxNestedLevelError
	if errors.As(err, &arrayErr) || errors.As(err, &mapErr) || errors.As(err, &nestingErr) {
		return ErrResourceLimit
	}
	return ErrMalformed
}

// flag is a boolean written as the unsigned integer 0 or 1.
type flag bool

// MarshalCBOR writes the flag as 0 or 1.
func (f flag) MarshalCBOR() ([]byte, error) {
	if f {
		return encoding.Marshal(uint8(1))
	}
	return encoding.Marshal(uint8(0))
}

// UnmarshalCBOR reads 0 or 1.
func (f *flag) UnmarshalCBOR(data []byte) error {
	var value uint64
	if err := decoding.Unmarshal(data, &value); err != nil {
		return err
	}
	if value > 1 {
		return ErrMalformed
	}
	*f = value == 1
	return nil
}

// validText reports whether every value is UTF-8.
func validText(values ...string) bool {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return false
		}
	}
	return true
}

// A record is an array led by the schema of its kind.
func knownRecordSchema(schema uint64) bool {
	switch schema {
	case recordSchemaCredential, recordSchemaIdentity, recordSchemaAttachment, recordSchemaCard, recordSchemaNote, recordSchemaSeed:
		return true
	default:
		return false
	}
}

// readSchema checks that a record leads with schema; another kind's is malformed.
func readSchema(plaintext []byte, schema uint64) error {
	var head [1]*uint64
	if err := decoding.Unmarshal(plaintext, &head); err != nil {
		return decodeError(err)
	}
	switch {
	case head[0] == nil:
		return ErrMalformed
	case *head[0] == schema:
		return nil
	case knownRecordSchema(*head[0]):
		return ErrMalformed
	default:
		return ErrUnsupported
	}
}
