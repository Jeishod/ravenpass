// Package aliasvault reads AliasVault exports: .avux, password-protected .avex, and CSV.
package aliasvault

import (
	"bytes"
	"io"

	"github.com/dortanes/ravenpass/packages/importers"
)

// Open recognises an AliasVault export by content in r[:size] and keeps no reference to r.
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
		return openArchive(source, source.Size())
	}
	document, err := importers.ReadDocument(source, importers.MaxSealedArchiveBytes)
	if err != nil {
		return nil, err
	}
	return openText(document)
}

// openText owns document: the returned file holds it, or it is cleared on failure.
func openText(document []byte) (*importers.ExportFile, error) {
	text := importers.TrimByteOrderMark(document)
	head, payload, delimited := bytes.Cut(text, sealedDelimiter)
	var opened *importers.ExportFile
	var err error
	switch {
	case delimited && isObject(head):
		opened, err = openSealed(head, payload)
	case len(document) > importers.MaxExportBytes:
		err = importers.ErrTooLarge
	default:
		opened, err = openCSV(text)
	}
	if err != nil {
		clear(document)
		return nil, err
	}
	return opened, nil
}

func isObject(text []byte) bool {
	content := bytes.TrimLeft(text, " \t\r\n")
	return len(content) > 0 && content[0] == '{'
}
