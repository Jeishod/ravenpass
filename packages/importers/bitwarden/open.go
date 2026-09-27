// Package bitwarden reads Bitwarden individual vault exports: JSON, encrypted JSON, ZIP and CSV.
package bitwarden

import (
	"bytes"
	"io"

	"github.com/dortanes/ravenpass/packages/importers"
)

// A ZIP export holds data.json at its root and attached files under attachments/.
const (
	archiveData        = "data.json"
	archiveAttachments = "attachments/"
)

// Open recognises a Bitwarden export by content in r[:size] and keeps no reference to r.
func Open(r io.ReaderAt, size int64) (importers.File, error) {
	if size < 0 {
		return nil, importers.ErrUnrecognized
	}
	opened, err := open(io.NewSectionReader(r, 0, size))
	if err != nil {
		return nil, err
	}
	return opened, nil
}

func open(source *io.SectionReader) (*importers.ExportFile, error) {
	archive, err := importers.IsZIP(source)
	if err != nil {
		return nil, err
	}
	if archive {
		return openArchive(source)
	}
	document, err := importers.ReadDocument(source, importers.MaxExportBytes)
	if err != nil {
		return nil, err
	}
	return openText(document)
}

// openText owns document: the returned file holds it, or it is cleared.
func openText(document []byte) (*importers.ExportFile, error) {
	text := importers.TrimByteOrderMark(document)
	if content := bytes.TrimLeft(text, " \t\r\n"); len(content) > 0 && content[0] == '{' {
		return openJSON(text)
	}
	return openCSV(text)
}

func openArchive(source *io.SectionReader) (*importers.ExportFile, error) {
	document, attachments, err := importers.ReadZIP(source, source.Size(), archiveData, archiveAttachments)
	if err != nil {
		return nil, err
	}
	plain, locked, err := readJSON(importers.TrimByteOrderMark(document))
	switch {
	case err != nil:
		return nil, err
	case locked != nil:
		return importers.NewLockedExportFile(importers.FormatZIP, locked, attachments), nil
	}
	return importers.NewExportFile(importers.FormatZIP, plain, attachments), nil
}
