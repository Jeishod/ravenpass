package vault

import "github.com/fxamacker/cbor/v2"

// Tests build structures field by field, including malformed ones, from these encoded values.

func encodeArray(values ...[]byte) []byte {
	elements := make([]cbor.RawMessage, len(values))
	for i, value := range values {
		elements[i] = value
	}
	return mustMarshal(elements)
}

func encodeBytes(value []byte) []byte { return mustMarshal(value) }

func encodeUint(value uint64) []byte { return mustMarshal(value) }

// mustEncode is a record encoding the test expects to succeed.
func mustEncode(encoded []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return encoded
}

func encodeTexts(values []string) [][]byte {
	encoded := make([][]byte, len(values))
	for i, value := range values {
		encoded[i] = mustMarshal(value)
	}
	return encoded
}
