package importers

import "io/fs"

// Readable is export text a source maps into items.
type Readable interface {
	// Read maps the text's items, writing unmapped values under labels.
	Read(labels Labels) (Export, error)
	// Wipe zeroes the text in memory; Read is not called afterwards.
	Wipe()
}

// Sealed is an export that needs its file password.
type Sealed interface {
	// Unseal opens the export with password; ErrWrongPassword leaves it sealed for another attempt.
	Unseal(password string) (*ExportFile, error)
}

// ExportFile is a File holding readable export text or, while locked, a sealed export.
type ExportFile struct {
	format      Format
	readable    Readable
	sealed      Sealed
	attachments int
}

// NewExportFile returns a readable file of format carrying attachments files beside its data.
func NewExportFile(format Format, readable Readable, attachments int) *ExportFile {
	return &ExportFile{format: format, readable: readable, attachments: attachments}
}

// NewLockedExportFile returns a file of format that stays locked until its password unseals it.
func NewLockedExportFile(format Format, sealed Sealed, attachments int) *ExportFile {
	return &ExportFile{format: format, sealed: sealed, attachments: attachments}
}

// Format reports the export's format.
func (f *ExportFile) Format() Format {
	return f.format
}

// Locked reports whether the export still needs its file password.
func (f *ExportFile) Locked() bool {
	return f.sealed != nil
}

// Unlock unseals a locked export; unsealed content that is itself locked is ErrUnrecognized.
func (f *ExportFile) Unlock(password string) error {
	switch {
	case f.sealed == nil && f.readable == nil:
		return fs.ErrClosed
	case f.sealed == nil:
		return nil
	}
	unsealed, err := f.sealed.Unseal(password)
	if err != nil {
		return err
	}
	if unsealed.Locked() {
		return ErrUnrecognized
	}
	f.readable, f.sealed = unsealed.readable, nil
	f.attachments += unsealed.attachments
	return nil
}

// Read maps the export's items, writing unmapped values under labels.
func (f *ExportFile) Read(labels Labels) (Export, error) {
	switch {
	case f.sealed != nil:
		return Export{}, ErrLocked
	case f.readable == nil:
		return Export{}, fs.ErrClosed
	}
	export, err := f.readable.Read(labels)
	if err != nil {
		return Export{}, err
	}
	export.Format = f.format
	export.Attachments = f.attachments
	return export, nil
}

// Close wipes the export text; the file cannot be read or unlocked afterwards.
func (f *ExportFile) Close() {
	if f.readable != nil {
		f.readable.Wipe()
	}
	f.readable, f.sealed = nil, nil
}
