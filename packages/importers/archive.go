package importers

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
)

var zipSignature = []byte("PK\x03\x04")

// IsZIP reports whether r starts with the signature of a ZIP local file header.
func IsZIP(r io.ReaderAt) (bool, error) {
	prefix := make([]byte, len(zipSignature))
	read, err := r.ReadAt(prefix, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, Unrecognized(err)
	}
	return bytes.Equal(prefix[:read], zipSignature), nil
}

// ReadZIP reads entry data from the ZIP in r's first size bytes and counts the files under folder, which ends in a slash.
func ReadZIP(r io.ReaderAt, size int64, data, folder string) ([]byte, int, error) {
	archive, err := zip.NewReader(r, size)
	// An entry path that would escape a folder is harmless: nothing is ever extracted.
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, 0, Unrecognized(err)
	}
	if len(archive.File) > MaxArchiveEntries {
		return nil, 0, ErrTooLarge
	}
	var found *zip.File
	files := 0
	for _, entry := range archive.File {
		switch {
		case entry.Name == data && found == nil:
			found = entry
		case strings.HasPrefix(entry.Name, folder) && !entry.Mode().IsDir():
			files++
		}
	}
	if found == nil {
		return nil, 0, ErrUnrecognized
	}
	document, err := readEntry(found)
	if err != nil {
		return nil, 0, err
	}
	return document, files, nil
}

// readEntry reads into one buffer a byte past the claimed size, which archive/zip never exceeds, leaving no partial copy of the export.
func readEntry(entry *zip.File) ([]byte, error) {
	if entry.UncompressedSize64 > MaxExportBytes {
		return nil, ErrTooLarge
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, Unrecognized(err)
	}
	defer reader.Close()
	document := make([]byte, entry.UncompressedSize64+1)
	read, err := io.ReadFull(reader, document)
	switch {
	case errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF):
		return document[:read], nil
	case err != nil:
		clear(document)
		return nil, Unrecognized(err)
	default:
		clear(document)
		return nil, ErrUnrecognized
	}
}
