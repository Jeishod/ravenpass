package importers

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"testing"
)

// archiveEntry is one entry of a test archive; a name ending in a slash is a folder.
type archiveEntry struct {
	name    string
	content string
}

func archiveOf(t *testing.T, entries ...archiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		created, err := writer.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := created.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func readZIP(archive []byte) ([]byte, int, error) {
	return ReadZIP(bytes.NewReader(archive), int64(len(archive)), "data.json", "attachments/")
}

// failingReader fails every read with cause.
type failingReader struct {
	cause error
}

func (r failingReader) ReadAt([]byte, int64) (int, error) {
	return 0, r.cause
}

func TestIsZIPReadsTheSignature(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{"an archive", archiveOf(t, archiveEntry{"data.json", "{}"}), true},
		{"the signature alone", []byte("PK\x03\x04"), true},
		{"text", []byte("ServiceName,CurrentPassword\n"), false},
		{"shorter than the signature", []byte("PK"), false},
		{"empty", nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := IsZIP(bytes.NewReader(test.content))
			if err != nil || got != test.want {
				t.Fatalf("IsZIP = %v, %v; want %v", got, err, test.want)
			}
		})
	}
	cause := errors.New("disk failed")
	if _, err := IsZIP(failingReader{cause}); !errors.Is(err, ErrUnrecognized) || !errors.Is(err, cause) {
		t.Fatalf("a failed read: %v", err)
	}
}

func TestReadZIPReadsItsDataAndCountsTheFolder(t *testing.T) {
	document, files, err := readZIP(archiveOf(t,
		archiveEntry{"data.json", "\xef\xbb\xbf{}"},
		archiveEntry{"attachments/", ""},
		archiveEntry{"attachments/a/", ""},
		archiveEntry{"attachments/a/manual.pdf", "pdf"},
		archiveEntry{"attachments/b.txt", "txt"},
		archiveEntry{"attachmentsnot/c.txt", "c"},
		archiveEntry{"readme.txt", "not an attachment"},
		archiveEntry{"data.json", "a second entry of the name"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if string(document) != "\xef\xbb\xbf{}" || files != 2 {
		t.Fatalf("read %q and %d files", document, files)
	}
}

func TestReadZIPRefusesWhatItCannotRead(t *testing.T) {
	tests := []struct {
		name    string
		archive []byte
	}{
		{"an archive without its data", archiveOf(t, archiveEntry{"export/data.json", "{}"}, archiveEntry{"attachments/a.txt", "a"})},
		{"a truncated archive", archiveOf(t, archiveEntry{"data.json", "{}"})[:40]},
		{"text", []byte("not an archive")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := readZIP(test.archive); !errors.Is(err, ErrUnrecognized) {
				t.Fatalf("ReadZIP = %v", err)
			}
		})
	}
}

func TestReadZIPBoundsWhatItReads(t *testing.T) {
	var crowded bytes.Buffer
	writer := zip.NewWriter(&crowded)
	for i := range MaxArchiveEntries + 1 {
		if _, err := writer.CreateRaw(&zip.FileHeader{Name: fmt.Sprintf("attachments/%d", i), Method: zip.Store}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readZIP(crowded.Bytes()); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("an archive over the entry bound: %v", err)
	}

	claims := func(claimed uint64, content string) []byte {
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		raw, err := writer.CreateRaw(&zip.FileHeader{Name: "data.json", Method: zip.Store, CompressedSize64: uint64(len(content)), UncompressedSize64: claimed})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := raw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	if _, _, err := readZIP(claims(MaxExportBytes+1, "{}")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("data claiming more than the bound: %v", err)
	}
	if _, _, err := readZIP(claims(4, `{"longer": "than it claims"}`)); !errors.Is(err, ErrUnrecognized) {
		t.Fatalf("data longer than it claims: %v", err)
	}
}
