package aliasvault

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/importers/importerstest"
)

var english = importerstest.English()

const noteItem = `{"id": "n1", "name": "Router", "itemType": "Note", "folderId": null,
	"fieldValues": [{"fieldKey": "notes.content", "fieldDefinitionId": null, "value": "admin panel", "weight": 0}],
	"totpCodes": [], "passkeys": [], "attachments": []}`

const csvHeader = "ServiceName,FolderPath,ServiceUrl,Username,CurrentPassword,AliasEmail,TwoFactorSecret,AliasGender,AliasFirstName,AliasLastName,AliasNickName,AliasBirthDate,CardholderName,CardNumber,CardExpiryMonth,CardExpiryYear,CardCvv,CardPin,Notes,CreatedAt,UpdatedAt\r\n"

const csvNote = csvHeader + "Router,,,,,,,,,,,,,,,,,,admin panel,03/19/2026 08:59:05,03/19/2026 08:59:05\r\n"

func manifestOf(items ...string) string {
	return `{"version": "1.0.0", "exportedAt": "2026-03-19T08:59:16.938Z", "exportedBy": "alex@example.test",
		"items": [` + strings.Join(items, ",") + `], "folders": [], "tags": [], "itemTags": [], "fieldDefinitions": [], "logos": []}`
}

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

func avuxOf(t *testing.T, manifest string, entries ...archiveEntry) []byte {
	t.Helper()
	return archiveOf(t, append([]archiveEntry{{archiveManifest, manifest}}, entries...)...)
}

func openContent(t *testing.T, content []byte) importers.File {
	t.Helper()
	opened, err := Open(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(opened.Close)
	return opened
}

func readContent(t *testing.T, content []byte) importers.Export {
	t.Helper()
	export, err := openContent(t, content).Read(english)
	if err != nil {
		t.Fatal(err)
	}
	return export
}

func openError(content []byte) error {
	_, err := Open(bytes.NewReader(content), int64(len(content)))
	return err
}

// padded serves text followed by filler, standing in for a file of any size.
type padded struct {
	text   []byte
	filler byte
}

func (p padded) ReadAt(buffer []byte, offset int64) (int, error) {
	for i := range buffer {
		if at := offset + int64(i); at < int64(len(p.text)) {
			buffer[i] = p.text[at]
		} else {
			buffer[i] = p.filler
		}
	}
	return len(buffer), nil
}

func TestOpenRecognisesAFileByItsContentAlone(t *testing.T) {
	avux := avuxOf(t, manifestOf(noteItem),
		archiveEntry{"attachments/", ""},
		archiveEntry{"attachments/n1_a1_manual.pdf", "pdf"},
		archiveEntry{"attachments/n1_a2_photo.jpg", "jpg"},
		archiveEntry{"logos/example.com_l1.png", "png"},
	)
	tests := []struct {
		name        string
		content     []byte
		format      importers.Format
		locked      bool
		attachments int
	}{
		{"avux", avux, importers.FormatZIP, false, 2},
		{"avux with a byte-order mark in its manifest", avuxOf(t, "\xef\xbb\xbf"+manifestOf(noteItem)), importers.FormatZIP, false, 0},
		{"avex", fastSealing.seal(t, "correct horse", avux), importers.FormatEncryptedZIP, true, 2},
		{"avex with a byte-order mark", append([]byte("\xef\xbb\xbf"), fastSealing.seal(t, "correct horse", avux)...), importers.FormatEncryptedZIP, true, 2},
		{"csv", []byte(csvNote), importers.FormatCSV, false, 0},
		{"csv with a byte-order mark", []byte("\xef\xbb\xbf" + csvNote), importers.FormatCSV, false, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opened := openContent(t, test.content)
			if opened.Format() != test.format || opened.Locked() != test.locked {
				t.Fatalf("opened as %q, locked %v", opened.Format(), opened.Locked())
			}
			if err := opened.Unlock("correct horse"); err != nil {
				t.Fatal(err)
			}
			export, err := opened.Read(english)
			if err != nil {
				t.Fatal(err)
			}
			if export.Format != test.format || export.Attachments != test.attachments || len(export.Items) != 1 {
				t.Fatalf("read %+v", export)
			}
			if note := export.Items[0].Content.Note; note == nil || note.Label != "Router" || note.Body != "admin panel" {
				t.Fatalf("read %+v", export.Items[0].Content)
			}
		})
	}
}

func TestOpenReadsAnArchiveWhateverItsNameSays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.csv")
	if err := os.WriteFile(path, avuxOf(t, manifestOf(noteItem)), 0o600); err != nil {
		t.Fatal(err)
	}
	renamed, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer renamed.Close()
	info, err := renamed.Stat()
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(renamed, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if opened.Format() != importers.FormatZIP {
		t.Fatalf("opened as %q", opened.Format())
	}
}

func TestOpenRefusesWhatItCannotRead(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    error
	}{
		{"empty", nil, importers.ErrUnrecognized},
		{"binary", []byte{0x00, 0x01, 0x02, 0x03, 0xff}, importers.ErrUnrecognized},
		{"json without a sealed payload", []byte(`{"format": "avex", "version": "1.0.0"}`), importers.ErrUnrecognized},
		{"csv without a password column", []byte("ServiceName,Username\nRouter,admin\n"), importers.ErrUnrecognized},
		{"csv without a name column", []byte("Username,CurrentPassword\nadmin,secret\n"), importers.ErrUnrecognized},
		{"csv that is not UTF-8", []byte("ServiceName,CurrentPassword\nRouter,\xff\xfe\n"), importers.ErrUnrecognized},
		{"archive without manifest.json", archiveOf(t, archiveEntry{"data.json", manifestOf(noteItem)}, archiveEntry{"attachments/a.txt", "a"}), importers.ErrUnrecognized},
		{"archive with manifest.json in a folder", archiveOf(t, archiveEntry{"export/manifest.json", manifestOf(noteItem)}), importers.ErrUnrecognized},
		{"manifest version 2", avuxOf(t, strings.Replace(manifestOf(noteItem), `"1.0.0"`, `"2.0.0"`, 1)), importers.ErrUnrecognized},
		{"manifest without a version", avuxOf(t, `{"items": []}`), importers.ErrUnrecognized},
		{"manifest version as a number", avuxOf(t, `{"version": 1, "items": []}`), importers.ErrUnrecognized},
		{"manifest that is not json", avuxOf(t, "ServiceName,CurrentPassword\n"), importers.ErrUnrecognized},
		{"truncated archive", avuxOf(t, manifestOf(noteItem))[:40], importers.ErrUnrecognized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := openError(test.content); !errors.Is(err, test.want) {
				t.Fatalf("Open = %v, want %v", err, test.want)
			}
		})
	}
	later := avuxOf(t, strings.Replace(manifestOf(noteItem), `"1.0.0"`, `"1.7.2"`, 1))
	if err := openError(later); err != nil {
		t.Fatalf("a later manifest of version 1: %v", err)
	}
	delimited := []byte(csvHeader + "Router,,,,,,,,,,,,,,,,,,\"a" + string(sealedDelimiter) + "b\",,\r\n")
	if opened := openContent(t, delimited); opened.Format() != importers.FormatCSV {
		t.Fatalf("a CSV holding the delimiter opened as %q", opened.Format())
	}
}

func TestOpenBoundsWhatItReads(t *testing.T) {
	sealedHead := fastSealing.around(t, nil)
	if _, err := Open(bytes.NewReader(sealedHead), importers.MaxSealedArchiveBytes+1); !errors.Is(err, importers.ErrTooLarge) {
		t.Fatalf("an avex over its bound: %v", err)
	}
	if _, err := Open(padded{text: sealedHead, filler: 'a'}, importers.MaxExportBytes+1); err != nil {
		t.Fatalf("an avex over the bound of a CSV: %v", err)
	}
	csvHead := []byte("ServiceName,CurrentPassword\nRouter,")
	if _, err := Open(padded{text: csvHead, filler: 'a'}, importers.MaxExportBytes+1); !errors.Is(err, importers.ErrTooLarge) {
		t.Fatalf("a CSV over its bound: %v", err)
	}
	if _, err := Open(bytes.NewReader([]byte(csvNote)), -1); !errors.Is(err, importers.ErrUnrecognized) {
		t.Fatalf("a negative size: %v", err)
	}

	var claiming bytes.Buffer
	writer := zip.NewWriter(&claiming)
	manifest := manifestOf(noteItem)
	raw, err := writer.CreateRaw(&zip.FileHeader{Name: archiveManifest, Method: zip.Store, CompressedSize64: uint64(len(manifest)), UncompressedSize64: importers.MaxExportBytes + 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := openError(claiming.Bytes()); !errors.Is(err, importers.ErrTooLarge) {
		t.Fatalf("a manifest claiming more than the bound: %v", err)
	}
}

func TestSealedArchiveOpensWithItsPassword(t *testing.T) {
	avux := avuxOf(t, manifestOf(noteItem, loginItem), archiveEntry{"attachments/l1_a1_notes.txt", "txt"})
	opened := openContent(t, fastSealing.seal(t, "correct horse", avux))
	if _, err := opened.Read(english); !errors.Is(err, importers.ErrLocked) {
		t.Fatalf("read before unlock: %v", err)
	}
	if err := opened.Unlock("wrong horse"); !errors.Is(err, importers.ErrWrongPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	if !opened.Locked() {
		t.Fatal("a wrong password unlocked the file")
	}
	if err := opened.Unlock("correct horse"); err != nil {
		t.Fatalf("the right password after a wrong one: %v", err)
	}
	if err := opened.Unlock("anything"); err != nil || opened.Locked() {
		t.Fatalf("unlocking an unlocked file: %v, locked %v", err, opened.Locked())
	}
	export, err := opened.Read(english)
	if err != nil {
		t.Fatal(err)
	}
	want := readContent(t, avux)
	want.Format = importers.FormatEncryptedZIP
	if !reflect.DeepEqual(export, want) || export.Attachments != 1 {
		t.Fatalf("read %+v, want %+v", export, want)
	}
}

func TestSealedArchiveAcceptsTheKeyDerivationInAnyLetterCase(t *testing.T) {
	for _, name := range []string{"Argon2Id", "Argon2id", "argon2id", "ARGON2ID"} {
		scheme := fastSealing
		scheme.kdf = name
		opened := openContent(t, scheme.seal(t, "password", avuxOf(t, manifestOf(noteItem))))
		if err := opened.Unlock("password"); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestSealedArchiveRefusesAnUnsupportedHeader(t *testing.T) {
	with := func(edit func(*sealing)) []byte {
		scheme := fastSealing
		edit(&scheme)
		return scheme.around(t, make([]byte, nonceLength+tagLength))
	}
	tests := []struct {
		name    string
		content []byte
		want    error
	}{
		{"another format", with(func(s *sealing) { s.format = "avux" }), importers.ErrUnrecognized},
		{"no format", with(func(s *sealing) { s.format = "" }), importers.ErrUnrecognized},
		{"another version", with(func(s *sealing) { s.version = "1.1.0" }), importers.ErrUnsupportedEncryption},
		{"another key derivation", with(func(s *sealing) { s.kdf = "PBKDF2" }), importers.ErrUnsupportedEncryption},
		{"iterations below the range", with(func(s *sealing) { s.iterations = 0 }), importers.ErrUnsupportedEncryption},
		{"iterations above the range", with(func(s *sealing) { s.iterations = 11 }), importers.ErrUnsupportedEncryption},
		{"memory below 8 KiB a lane", with(func(s *sealing) { s.parallelism, s.memory = 4, 31 }), importers.ErrUnsupportedEncryption},
		{"memory above the range", with(func(s *sealing) { s.memory = 1<<20 + 1 }), importers.ErrUnsupportedEncryption},
		{"parallelism below the range", with(func(s *sealing) { s.parallelism = 0 }), importers.ErrUnsupportedEncryption},
		{"parallelism above the range", with(func(s *sealing) { s.parallelism, s.memory = 17, 1024 }), importers.ErrUnsupportedEncryption},
		{"a blank salt", with(func(s *sealing) { s.salt = " " }), importers.ErrUnrecognized},
		{"another cipher", with(func(s *sealing) { s.algorithm = "AES-256-CBC" }), importers.ErrUnsupportedEncryption},
		{"a payload short of a nonce and a tag", fastSealing.around(t, make([]byte, nonceLength+tagLength-1)), importers.ErrUnrecognized},
		{"a header that is not json", []byte("{\"format\": \"avex\"," + string(sealedDelimiter) + strings.Repeat("a", 64)), importers.ErrUnrecognized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := openError(test.content); !errors.Is(err, test.want) {
				t.Fatalf("Open = %v, want %v", err, test.want)
			}
		})
	}
	accepted := []func(*sealing){
		func(s *sealing) { s.iterations = minIterations },
		func(s *sealing) { s.iterations = maxIterations },
		func(s *sealing) { s.parallelism, s.memory = 4, 32 },
		func(s *sealing) { s.memory = maxMemoryKiB },
		func(s *sealing) { s.parallelism, s.memory = maxParallelism, 128 },
	}
	for i, edit := range accepted {
		if err := openError(with(edit)); err != nil {
			t.Errorf("parameters at the edge of their range %d: %v", i, err)
		}
	}
}

func TestSealedArchiveRefusesAlteredData(t *testing.T) {
	altered := fastSealing.seal(t, "password", avuxOf(t, manifestOf(noteItem)))
	altered[len(altered)-tagLength-1] ^= 1
	tests := []struct {
		name    string
		content []byte
		want    error
	}{
		{"an altered payload", altered, importers.ErrWrongPassword},
		{"a payload that is not an archive", fastSealing.seal(t, "password", []byte(csvNote)), importers.ErrUnrecognized},
		{"an archive without manifest.json", fastSealing.seal(t, "password", archiveOf(t, archiveEntry{"data.json", "{}"})), importers.ErrUnrecognized},
		{"an archive of manifest version 2", fastSealing.seal(t, "password", avuxOf(t, `{"version": "2.0.0", "items": []}`)), importers.ErrUnrecognized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opened := openContent(t, test.content)
			if err := opened.Unlock("password"); !errors.Is(err, test.want) {
				t.Fatalf("Unlock = %v, want %v", err, test.want)
			}
			if !opened.Locked() {
				t.Fatal("the file unlocked")
			}
		})
	}
}

func TestCloseClearsWhatTheFileHolds(t *testing.T) {
	avux := avuxOf(t, manifestOf(noteItem))
	unlocked := openContent(t, fastSealing.seal(t, "password", avux))
	if err := unlocked.Unlock("password"); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		opened importers.File
	}{
		{"avux", openContent(t, avux)},
		{"unlocked avex", unlocked},
		{"csv", openContent(t, []byte(csvNote))},
	}
	for _, test := range tests {
		test.opened.Close()
		if _, err := test.opened.Read(english); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("%s: Read after Close = %v", test.name, err)
		}
		if err := test.opened.Unlock("password"); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("%s: Unlock after Close = %v", test.name, err)
		}
	}
	locked := openContent(t, fastSealing.seal(t, "password", avux))
	locked.Close()
	if err := locked.Unlock("password"); !errors.Is(err, fs.ErrClosed) || locked.Locked() {
		t.Fatalf("Unlock of a file closed while locked = %v", err)
	}
}

func TestWipeClearsTheTextEachReaderHolds(t *testing.T) {
	document := manifestText(manifestOf(noteItem))
	rows := csvText{importers.CSV(csvNote)}
	for name, test := range map[string]struct {
		readable importers.Readable
		text     []byte
	}{"manifest": {document, document}, "csv": {rows, rows.CSV}} {
		test.readable.Wipe()
		if !bytes.Equal(test.text, make([]byte, len(test.text))) {
			t.Fatalf("%s: the text survives Wipe", name)
		}
	}
}
