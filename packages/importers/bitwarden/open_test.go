package bitwarden

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/importers/importerstest"
)

var english = importerstest.English()

func jsonExportOf(items ...string) []byte {
	return []byte(`{"encrypted":false,"folders":[],"items":[` + strings.Join(items, ",") + `]}`)
}

const noteJSON = `{"type":2,"name":"Router","notes":"admin panel","secureNote":{"type":0}}`

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

// archiveEntry is one entry of a test archive; a name ending in a slash is a directory.
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

func TestOpenRecognisesAFileByItsContentAlone(t *testing.T) {
	csvExport := "folder,favorite,type,name,notes,fields,reprompt,login_uri,login_username,login_password,login_totp\n,,note,Router,admin panel,,0,,,,\n"
	archive := archiveOf(t,
		archiveEntry{"data.json", string(jsonExportOf(noteJSON))},
		archiveEntry{"attachments/", ""},
		archiveEntry{"attachments/Router/", ""},
		archiveEntry{"attachments/Router/manual.pdf", "pdf"},
		archiveEntry{"attachments/Router/photo.jpg", "jpg"},
		archiveEntry{"attachments/Other/notes.txt", "txt"},
		archiveEntry{"readme.txt", "not an attachment"},
	)
	tests := []struct {
		name        string
		content     []byte
		format      importers.Format
		attachments int
	}{
		{"json", jsonExportOf(noteJSON), importers.FormatJSON, 0},
		{"json with a byte-order mark", append([]byte("\xef\xbb\xbf"), jsonExportOf(noteJSON)...), importers.FormatJSON, 0},
		{"json after blank lines", append([]byte("\r\n \t\n"), jsonExportOf(noteJSON)...), importers.FormatJSON, 0},
		{"csv", []byte(csvExport), importers.FormatCSV, 0},
		{"csv with a byte-order mark", []byte("\xef\xbb\xbf" + csvExport), importers.FormatCSV, 0},
		{"zip", archive, importers.FormatZIP, 3},
		{"zip with a byte-order mark in data.json", archiveOf(t, archiveEntry{"data.json", "\xef\xbb\xbf" + string(jsonExportOf(noteJSON))}), importers.FormatZIP, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opened := openContent(t, test.content)
			if opened.Format() != test.format || opened.Locked() {
				t.Fatalf("opened as %q, locked %v", opened.Format(), opened.Locked())
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

func TestOpenRefusesWhatItCannotRead(t *testing.T) {
	encString := "2." + strings.Repeat("A", 22) + "==|" + strings.Repeat("A", 22) + "==|" + strings.Repeat("A", 43) + "="
	protected := func(kdf string) []byte {
		return []byte(`{"encrypted":true,"passwordProtected":true,"salt":"salt",` + kdf + `,"encKeyValidation_DO_NOT_EDIT":"` + encString + `","data":"` + encString + `"}`)
	}
	tests := []struct {
		name    string
		content []byte
		want    error
	}{
		{"account-restricted", []byte(`{"encrypted":true,"encKeyValidation_DO_NOT_EDIT":"` + encString + `","folders":[],"items":[]}`), importers.ErrAccountBound},
		{"encrypted without a password", []byte(`{"encrypted":true,"passwordProtected":false}`), importers.ErrAccountBound},
		{"json without encrypted", []byte(`{"folders":[],"items":[]}`), importers.ErrUnrecognized},
		{"encrypted of the wrong type", []byte(`{"encrypted":"false","items":[]}`), importers.ErrUnrecognized},
		{"truncated json", []byte(`{"encrypted":false,"items":[`), importers.ErrUnrecognized},
		{"json array", []byte(`[{"encrypted":false}]`), importers.ErrUnrecognized},
		{"csv without a name column", []byte("type,title\nlogin,Example\n"), importers.ErrUnrecognized},
		{"csv that is not UTF-8", []byte("type,name\nlogin,\xff\xfe\n"), importers.ErrUnrecognized},
		{"empty", nil, importers.ErrUnrecognized},
		{"binary", []byte{0x00, 0x01, 0x02, 0x03, 0xff}, importers.ErrUnrecognized},
		{"zip without data.json", archiveOf(t, archiveEntry{"export/data.json", string(jsonExportOf())}, archiveEntry{"attachments/a/b.txt", "b"}), importers.ErrUnrecognized},
		{"zip with data.json that is not json", archiveOf(t, archiveEntry{"data.json", "type,name\nlogin,Example\n"}), importers.ErrUnrecognized},
		{"zip with an account-restricted data.json", archiveOf(t, archiveEntry{"data.json", `{"encrypted":true}`}), importers.ErrAccountBound},
		{"truncated zip", archiveOf(t, archiveEntry{"data.json", string(jsonExportOf())})[:40], importers.ErrUnrecognized},
		{"PBKDF2 iterations below the range", protected(`"kdfType":0,"kdfIterations":4999`), importers.ErrUnsupportedEncryption},
		{"Argon2id memory above the range", protected(`"kdfType":1,"kdfIterations":3,"kdfMemory":2048,"kdfParallelism":4`), importers.ErrUnsupportedEncryption},
		{"an unknown key derivation", protected(`"kdfType":7,"kdfIterations":600000`), importers.ErrUnsupportedEncryption},
		{"another envelope type", bytes.ReplaceAll(protected(`"kdfType":0,"kdfIterations":600000`), []byte(`"2.`), []byte(`"0.`)), importers.ErrUnsupportedEncryption},
		{"a malformed envelope", bytes.ReplaceAll(protected(`"kdfType":0,"kdfIterations":600000`), []byte(`=|`), []byte(`=`)), importers.ErrUnrecognized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := openError(test.content); !errors.Is(err, test.want) {
				t.Fatalf("Open = %v, want %v", err, test.want)
			}
		})
	}
}

func TestOpenBoundsWhatItReads(t *testing.T) {
	if _, err := Open(bytes.NewReader(jsonExportOf()), importers.MaxExportBytes+1); !errors.Is(err, importers.ErrTooLarge) {
		t.Fatalf("a document over the bound: %v", err)
	}
	if _, err := Open(bytes.NewReader(jsonExportOf()), -1); !errors.Is(err, importers.ErrUnrecognized) {
		t.Fatalf("a negative size: %v", err)
	}
	if _, err := Open(bytes.NewReader(jsonExportOf()), 10); !errors.Is(err, importers.ErrUnrecognized) {
		t.Fatalf("a size short of the document: %v", err)
	}

	var crowded bytes.Buffer
	writer := zip.NewWriter(&crowded)
	for i := range importers.MaxArchiveEntries + 1 {
		if _, err := writer.CreateRaw(&zip.FileHeader{Name: fmt.Sprintf("attachments/%d", i), Method: zip.Store}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := openError(crowded.Bytes()); !errors.Is(err, importers.ErrTooLarge) {
		t.Fatalf("an archive over the entry bound: %v", err)
	}

	claims := func(claimed uint64, content string) []byte {
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		raw, err := writer.CreateRaw(&zip.FileHeader{Name: archiveData, Method: zip.Store, CompressedSize64: uint64(len(content)), UncompressedSize64: claimed})
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
	document := string(jsonExportOf(noteJSON))
	if err := openError(claims(importers.MaxExportBytes+1, document)); !errors.Is(err, importers.ErrTooLarge) {
		t.Fatalf("data.json claiming more than the bound: %v", err)
	}
	if err := openError(claims(4, document)); !errors.Is(err, importers.ErrUnrecognized) {
		t.Fatalf("data.json longer than it claims: %v", err)
	}
}

func TestPasswordProtectedExportOpensWithItsPassword(t *testing.T) {
	for name, scheme := range map[string]derivation{"PBKDF2": fastPBKDF2, "Argon2id": fastArgon2id} {
		t.Run(name, func(t *testing.T) {
			opened := openContent(t, protectedExport(t, "correct horse", scheme, jsonExportOf(noteJSON)))
			if opened.Format() != importers.FormatEncryptedJSON || !opened.Locked() {
				t.Fatalf("opened as %q, locked %v", opened.Format(), opened.Locked())
			}
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
			if export.Format != importers.FormatEncryptedJSON || len(export.Items) != 1 || export.Items[0].Content.Note.Label != "Router" {
				t.Fatalf("read %+v", export)
			}
		})
	}
}

func TestPasswordProtectedExportRefusesAlteredData(t *testing.T) {
	sealedExport := func(data []byte) []byte {
		return protectedExport(t, "password", fastPBKDF2, data)
	}
	tampered := func(content []byte) []byte {
		var document map[string]any
		if err := json.Unmarshal(content, &document); err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(document["data"].(string), "|")
		parts[2] = base64.StdEncoding.EncodeToString(make([]byte, sha256.Size))
		document["data"] = strings.Join(parts, "|")
		altered, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		return altered
	}
	tests := []struct {
		name    string
		content []byte
	}{
		{"tampered data", tampered(sealedExport(jsonExportOf(noteJSON)))},
		{"data that is not json", sealedExport([]byte("not json"))},
		{"data that is itself encrypted", sealedExport([]byte(`{"encrypted":true,"passwordProtected":true}`))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opened := openContent(t, test.content)
			if err := opened.Unlock("password"); !errors.Is(err, importers.ErrUnrecognized) {
				t.Fatalf("Unlock = %v", err)
			}
			if !opened.Locked() {
				t.Fatal("an altered file unlocked")
			}
		})
	}
}

func TestCloseClearsWhatTheFileHolds(t *testing.T) {
	plain := openContent(t, jsonExportOf(noteJSON))
	protected := openContent(t, protectedExport(t, "password", fastPBKDF2, jsonExportOf(noteJSON)))
	if err := protected.Unlock("password"); err != nil {
		t.Fatal(err)
	}
	for name, opened := range map[string]importers.File{"plain": plain, "unlocked": protected} {
		opened.Close()
		if _, err := opened.Read(english); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("%s: Read after Close = %v", name, err)
		}
		if err := opened.Unlock("password"); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("%s: Unlock after Close = %v", name, err)
		}
	}
}

func TestWipeClearsTheTextEachReaderHolds(t *testing.T) {
	document := jsonText(jsonExportOf(noteJSON))
	rows := csvText{importers.CSV("type,name\nnote,Router\n")}
	for name, test := range map[string]struct {
		readable importers.Readable
		text     []byte
	}{"json": {document, document}, "csv": {rows, rows.CSV}} {
		test.readable.Wipe()
		if !bytes.Equal(test.text, make([]byte, len(test.text))) {
			t.Fatalf("%s: the text survives Wipe", name)
		}
	}
}
