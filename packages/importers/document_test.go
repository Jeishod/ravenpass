package importers

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestReadDocumentReadsTheWholeFile(t *testing.T) {
	content := []byte("ServiceName,CurrentPassword\n")
	source := io.NewSectionReader(bytes.NewReader(content), 0, int64(len(content)))
	if _, err := source.Seek(5, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	document, err := ReadDocument(source, int64(len(content)))
	if err != nil || !bytes.Equal(document, content) {
		t.Fatalf("ReadDocument = %q, %v", document, err)
	}
	if _, err := ReadDocument(source, int64(len(content))-1); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("a document over the limit: %v", err)
	}
	short := io.NewSectionReader(bytes.NewReader(content), 0, int64(len(content))+10)
	if _, err := ReadDocument(short, MaxExportBytes); !errors.Is(err, ErrUnrecognized) {
		t.Fatalf("a file shorter than its size: %v", err)
	}
	empty, err := ReadDocument(io.NewSectionReader(bytes.NewReader(nil), 0, 0), MaxExportBytes)
	if err != nil || len(empty) != 0 {
		t.Fatalf("an empty file: %q, %v", empty, err)
	}
}

func TestTrimByteOrderMarkDropsOneLeadingMark(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"\xef\xbb\xbfServiceName", "ServiceName"},
		{"\xef\xbb\xbf\xef\xbb\xbfServiceName", "\xef\xbb\xbfServiceName"},
		{"ServiceName\xef\xbb\xbf", "ServiceName\xef\xbb\xbf"},
		{" \xef\xbb\xbfServiceName", " \xef\xbb\xbfServiceName"},
		{"\xef\xbbServiceName", "\xef\xbbServiceName"},
		{"", ""},
	}
	for _, test := range tests {
		if got := TrimByteOrderMark([]byte(test.text)); string(got) != test.want {
			t.Errorf("TrimByteOrderMark(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestUnrecognizedAndUnsupportedKeepTheirCause(t *testing.T) {
	cause := errors.New("cause")
	tests := []struct {
		err      error
		sentinel error
	}{
		{Unrecognized(cause), ErrUnrecognized},
		{Unsupported(cause), ErrUnsupportedEncryption},
	}
	for _, test := range tests {
		if !errors.Is(test.err, test.sentinel) || !errors.Is(test.err, cause) {
			t.Errorf("%v does not wrap %v and its cause", test.err, test.sentinel)
		}
	}
}
