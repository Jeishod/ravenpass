package importers

import (
	"bytes"
	"fmt"
	"io"
)

var byteOrderMark = []byte("\xef\xbb\xbf")

// ReadDocument reads the whole of r from its start, refusing a document over limit bytes before reading it.
func ReadDocument(r *io.SectionReader, limit int64) ([]byte, error) {
	if r.Size() > limit {
		return nil, ErrTooLarge
	}
	document := make([]byte, r.Size())
	if _, err := io.ReadFull(io.NewSectionReader(r, 0, r.Size()), document); err != nil {
		clear(document)
		return nil, Unrecognized(err)
	}
	return document, nil
}

// TrimByteOrderMark drops a leading UTF-8 byte-order mark; the result shares text's bytes.
func TrimByteOrderMark(text []byte) []byte {
	return bytes.TrimPrefix(text, byteOrderMark)
}

// Unrecognized wraps the cause of a file that cannot be read in ErrUnrecognized.
func Unrecognized(err error) error {
	return fmt.Errorf("%w: %w", ErrUnrecognized, err)
}

// Unsupported wraps the cause of an encryption that cannot be opened in ErrUnsupportedEncryption.
func Unsupported(err error) error {
	return fmt.Errorf("%w: %w", ErrUnsupportedEncryption, err)
}
