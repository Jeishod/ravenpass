// Package storage binds the store that keeps the current vault ciphertext and tracks the known vaults.
package storage

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// Kind names a way of keeping the vault current.
type Kind string

const (
	// LocalFile is a file the device's own file system holds at an absolute path.
	LocalFile Kind = "local-file"
	// Document is a file a document provider holds, reached through the address the file picker granted.
	Document Kind = "document"
)

// Failures the Manager reports.
var (
	ErrUnavailable      = errors.New("vault storage is unavailable")
	ErrUnsupportedKind  = errors.New("unsupported storage type")
	ErrTargetOccupied   = errors.New("chosen location already holds a file")
	ErrSameLocation     = errors.New("vault is already kept at that location")
	ErrSelectionInvalid = errors.New("storage selection is invalid")
	ErrUnknownVault     = errors.New("vault is not in the known list")
)

// Failures every store reports with these values, whatever its kind.
var (
	ErrNotFound            = errors.New("vault file not found")
	ErrEmptyCiphertext     = errors.New("ciphertext is empty")
	ErrTooLarge            = errors.New("vault file exceeds size limit")
	ErrInvalidPath         = errors.New("vault location is invalid")
	ErrDurabilityUncertain = errors.New("vault replaced but its durability is unconfirmed")
	ErrStaleHead           = errors.New("vault head changed")
	// ErrVaultMissing wraps ErrStaleHead where the location no longer holds the vault a commit expects.
	ErrVaultMissing      = errors.New("vault file is no longer at its location")
	ErrFinalizerRequired = errors.New("vault commit finalizer is required")
	ErrFinalizerFailed   = errors.New("vault finalizer failed")
)

// Reason explains why no storage is bound.
type Reason string

// Reasons no storage is bound.
const (
	ReasonNone              Reason = ""
	ReasonUnreachable       Reason = "unreachable"
	ReasonSelectionUnusable Reason = "selection-unusable"
)

// Target is where the vault lives, identified by Kind and Path; Label is display only.
type Target struct {
	Kind  Kind
	Path  string
	Label Label
	// Vault is the unverified identity of the vault this device last opened there; empty until one opens.
	Vault string
}

// Label is how the interface shows a target: the file's name and the place holding it.
type Label struct {
	Name  string
	Place string
}

// Same reports whether two targets are one location, however each is labelled.
func (target Target) Same(other Target) bool {
	return target.Kind == other.Kind && target.Path == other.Path
}

// Status reports the storage the interface should present.
type Status struct {
	Kinds []Kind
	// Current is the bound location, or the one that could not be bound.
	Current Target
	// Default is where a device that chose nothing keeps its vault.
	Default    Target
	Vaults     []Target
	Available  bool
	Restricted bool
	Reason     Reason
}

// Ciphertext is the whole contract a store owes the vault service.
type Ciphertext interface {
	LoadCiphertext() ([]byte, error)
	CommitCiphertext(expectedHash *[sha256.Size]byte, candidate []byte, finalize func() error) error
	ReconcileCiphertext(expectedHash [sha256.Size]byte, finalize func() error) error
}

// Store keeps one location's vault file; LoadCiphertext reports ErrNotFound where it holds none.
type Store interface {
	Ciphertext
	// Restricted reports whether the location keeps the file readable by its owner alone.
	Restricted() bool
	// Remove deletes the vault file and whatever the store keeps beside it; a missing file is no error.
	Remove() error
}

// Placeholder is a store whose file exists before a vault is written, as a picker-created document is.
type Placeholder interface {
	// DiscardEmpty deletes the file while it holds no vault and none was opened there.
	DiscardEmpty() error
}

// Backend keeps vaults of one kind.
type Backend interface {
	Kind() Kind
	// Check reports whether path can name a location of this kind. It does no I/O.
	Check(path string) error
	// Open prepares the store for target without writing a vault.
	Open(target Target, maxBytes int64) (Store, error)
}

// Relocation reports what a completed move left behind; an unremoved previous file goes stale.
type Relocation struct {
	Target          Target
	Previous        Target
	PreviousRemoved bool
}

// CheckHead compares a location's read result with the head a commit expects; nil expects no vault.
func CheckHead(expected *[sha256.Size]byte, current []byte, readErr error) error {
	switch {
	case readErr == nil:
		if expected == nil || sha256.Sum256(current) != *expected {
			return ErrStaleHead
		}
	case errors.Is(readErr, ErrNotFound):
		if expected != nil {
			return fmt.Errorf("%w: %w", ErrStaleHead, ErrVaultMissing)
		}
	default:
		return readErr
	}
	return nil
}

// Finish runs a commit's finalizer once the ciphertext is in place.
func Finish(finalize func() error) error {
	if err := finalize(); err != nil {
		return fmt.Errorf("%w: %w", ErrFinalizerFailed, err)
	}
	return nil
}
