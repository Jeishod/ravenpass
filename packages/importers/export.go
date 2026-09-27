// Package importers holds what every import source shares; each source reads in its own package.
package importers

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/vault"
)

// Format names how an export file is written.
type Format string

// The formats a reader finds.
const (
	FormatJSON          Format = "json"
	FormatEncryptedJSON Format = "encrypted-json"
	FormatZIP           Format = "zip"
	FormatEncryptedZIP  Format = "encrypted-zip"
	FormatCSV           Format = "csv"
)

// Origin is the kind an item had in the source.
type Origin string

// The kinds sources hold.
const (
	OriginLogin          Origin = "login"
	OriginAlias          Origin = "alias"
	OriginCard           Origin = "card"
	OriginIdentity       Origin = "identity"
	OriginPassport       Origin = "passport"
	OriginDriversLicense Origin = "drivers-license"
	OriginNote           Origin = "note"
	OriginSSHKey         Origin = "ssh-key"
	OriginBankAccount    Origin = "bank-account"
)

// Reason says why an item the export holds is not imported.
type Reason string

const (
	// ReasonTooLong is an item the vault refuses, by then only for a value over a bound.
	ReasonTooLong Reason = "too-long"
	// ReasonUnnamed is an item with no name and nothing to name it after.
	ReasonUnnamed Reason = "unnamed"
	// ReasonUnsupported is an item of a type the reader does not know.
	ReasonUnsupported Reason = "unsupported"
)

// Item is one item read from an export, with the folder and collection names it was filed under; Content.Groups is empty.
type Item struct {
	Content vault.NewItem
	Origin  Origin
	Folders []string
}

// Skip is an item the import cannot add; Origin is empty for a type the reader does not know.
type Skip struct {
	Label  string
	Origin Origin
	Reason Reason
}

// Export is what a reader found in a file.
type Export struct {
	Format      Format
	Items       []Item
	Skipped     []Skip
	Attachments int // files an archive export carries beside its data
	Passkeys    int // passkeys no item's content takes
}

// File is one opened export, independent of its source and not safe for concurrent use.
type File interface {
	Format() Format
	Locked() bool
	// Unlock opens a locked file; ErrWrongPassword leaves it locked for another attempt.
	Unlock(password string) error
	// Read fails with ErrLocked until a locked file is unlocked.
	Read(labels Labels) (Export, error)
	// Close clears what the file holds.
	Close()
}

// Errors a reader reports; compare with errors.Is.
var (
	ErrUnrecognized          = errors.New("file is not an export this source reads")
	ErrAccountBound          = errors.New("export is encrypted with a key only the source account holds")
	ErrWrongPassword         = errors.New("password does not open the export")
	ErrUnsupportedEncryption = errors.New("export encryption is unsupported")
	ErrTooLarge              = errors.New("export exceeds supported limits")
	ErrLocked                = errors.New("export is locked")
)

const (
	// MaxExportBytes bounds an unencrypted document, before or after decryption.
	MaxExportBytes = 64 << 20
	// MaxArchiveEntries bounds the entries an archive export holds.
	MaxArchiveEntries = 100_000
	// MaxSealedArchiveBytes bounds a password-protected archive, decrypted whole.
	MaxSealedArchiveBytes = 128 << 20
)
