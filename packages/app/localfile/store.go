//go:build darwin || linux

// Package localfile keeps a vault in a local file, replaced atomically under a directory lock.
package localfile

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/dortanes/ravenpass/packages/app/storage"
	"golang.org/x/sys/unix"
)

const (
	// DefaultVaultName is the file name Ravenpass proposes when the user has not chosen one.
	DefaultVaultName = "vault.rpv"
	maxNameLength    = 128
	tempSuffixLength = 32
)

// ErrUnsafePermissions reports a vault path that is not a regular file or cannot be kept private.
var ErrUnsafePermissions = errors.New("vault path has unsafe permissions")

// Backend keeps vaults in local files; only Home is created on demand, so an absent disk never reads as empty.
type Backend struct {
	Home string
}

// Kind identifies this backend among those a storage.Manager offers.
func (Backend) Kind() storage.Kind { return storage.LocalFile }

// Check accepts an absolute, clean path.
func (Backend) Check(path string) error {
	if !filepath.IsAbs(path) || path != filepath.Clean(path) {
		return fmt.Errorf("%w: path is not absolute", storage.ErrInvalidPath)
	}
	return nil
}

// Open creates Home when target lies directly in it; any other missing directory fails.
func (backend Backend) Open(target storage.Target, maxBytes int64) (storage.Store, error) {
	if backend.Home != "" && filepath.Dir(target.Path) == backend.Home {
		if err := os.MkdirAll(backend.Home, 0700); err != nil {
			return nil, fmt.Errorf("create vault directory: %w", err)
		}
	}
	return New(target.Path, maxBytes)
}

// Store keeps a vault in a chosen file, touching only the lock and interrupted-write files beside it.
type Store struct {
	directory  string
	name       string
	lockName   string
	tempBase   string
	maxBytes   int64
	restricted bool
}

// New opens the store for the vault file at path, cleaning up interrupted writes.
func New(path string, maxBytes int64) (*Store, error) {
	directory, name, err := splitVaultPath(path)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 || maxBytes == math.MaxInt64 {
		return nil, errors.New("vault size limit is invalid")
	}
	store := &Store{
		directory: directory,
		name:      name,
		lockName:  "." + name + ".lock",
		tempBase:  "." + name + ".tmp-",
		maxBytes:  maxBytes,
	}
	directoryFD, err := store.openDirectory()
	if err != nil {
		return nil, err
	}
	defer unix.Close(directoryFD)

	var existing unix.Stat_t
	switch err := unix.Fstatat(directoryFD, name, &existing, unix.AT_SYMLINK_NOFOLLOW); {
	case err == nil && existing.Mode&unix.S_IFMT != unix.S_IFREG:
		return nil, fmt.Errorf("%w: vault path must name a regular file", storage.ErrInvalidPath)
	case err != nil && !errors.Is(err, unix.ENOENT):
		return nil, fmt.Errorf("inspect vault file: %w", err)
	}

	restricted, err := probeRestriction(directoryFD, store.tempBase)
	if err != nil {
		return nil, fmt.Errorf("prepare vault location: %w", err)
	}
	store.restricted = restricted

	if err := store.withLock(directoryFD, func() error {
		return store.cleanupTemps(directoryFD)
	}); err != nil {
		return nil, fmt.Errorf("clean up interrupted writes: %w", err)
	}
	return store, nil
}

// Restricted reports whether the file system keeps the vault file readable by its owner alone.
func (store *Store) Restricted() bool { return store.restricted }

// LoadCiphertext reads the vault file; storage.ErrNotFound when there is none, storage.ErrEmptyFile when it is empty.
func (store *Store) LoadCiphertext() ([]byte, error) {
	directoryFD, err := store.openDirectory()
	if err != nil {
		return nil, err
	}
	defer unix.Close(directoryFD)
	return store.loadCiphertext(directoryFD)
}

func (store *Store) loadCiphertext(directoryFD int) ([]byte, error) {
	fd, err := unix.Openat(directoryFD, store.name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open vault file: %w", err)
	}
	file := os.NewFile(uintptr(fd), store.name)
	defer file.Close()

	var before unix.Stat_t
	if err := unix.Fstat(fd, &before); err != nil {
		return nil, fmt.Errorf("inspect vault file: %w", err)
	}
	if before.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, ErrUnsafePermissions
	}
	if store.restricted && before.Mode&0777 != 0600 {
		if err := restrict(fd, &before); err != nil {
			return nil, err
		}
	}
	// An empty file holds no vault, as an empty document does: a new vault may replace it, and one opened there is lost.
	if before.Size <= 0 {
		return nil, storage.ErrEmptyFile
	}
	if before.Size > store.maxBytes {
		return nil, storage.ErrTooLarge
	}

	data, err := io.ReadAll(io.LimitReader(file, store.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read vault file: %w", err)
	}
	if int64(len(data)) > store.maxBytes {
		return nil, storage.ErrTooLarge
	}

	var after unix.Stat_t
	if err := unix.Fstat(fd, &after); err != nil {
		return nil, fmt.Errorf("inspect read vault file: %w", err)
	}
	if int64(len(data)) != before.Size || before.Size != after.Size || before.Mtim != after.Mtim {
		return nil, errors.New("vault file changed during read")
	}
	return data, nil
}

// CommitCiphertext replaces the vault file under the lock while it still holds expectedHash, then runs finalize.
func (store *Store) CommitCiphertext(expectedHash *[sha256.Size]byte, ciphertext []byte, finalize func() error) error {
	if len(ciphertext) == 0 {
		return storage.ErrEmptyCiphertext
	}
	if int64(len(ciphertext)) > store.maxBytes {
		return storage.ErrTooLarge
	}
	if finalize == nil {
		return storage.ErrFinalizerRequired
	}

	directoryFD, err := store.openDirectory()
	if err != nil {
		return err
	}
	defer unix.Close(directoryFD)

	return store.withLock(directoryFD, func() error {
		if err := store.cleanupTemps(directoryFD); err != nil {
			return fmt.Errorf("clean up interrupted writes: %w", err)
		}
		if err := store.checkExpectedHash(directoryFD, expectedHash); err != nil {
			return err
		}
		if err := store.save(directoryFD, ciphertext); err != nil {
			return err
		}
		return storage.Finish(finalize)
	})
}

// ReconcileCiphertext runs finalize under the lock while the vault file holds expectedHash.
func (store *Store) ReconcileCiphertext(expectedHash [sha256.Size]byte, finalize func() error) error {
	if finalize == nil {
		return storage.ErrFinalizerRequired
	}
	directoryFD, err := store.openDirectory()
	if err != nil {
		return err
	}
	defer unix.Close(directoryFD)

	return store.withLock(directoryFD, func() error {
		if err := store.checkExpectedHash(directoryFD, &expectedHash); err != nil {
			return err
		}
		return storage.Finish(finalize)
	})
}

// Remove deletes the vault file with its lock and interrupted-write files; a missing file is not an error.
func (store *Store) Remove() error {
	directoryFD, err := store.openDirectory()
	if err != nil {
		return err
	}
	defer unix.Close(directoryFD)

	return store.withLock(directoryFD, func() error {
		if err := store.cleanupTemps(directoryFD); err != nil {
			return fmt.Errorf("clean up interrupted writes: %w", err)
		}
		// The lock file goes last: its lock is held until the descriptor closes.
		for _, name := range []string{store.name, store.lockName} {
			if err := unix.Unlinkat(directoryFD, name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
				return fmt.Errorf("remove vault file: %w", err)
			}
		}
		if err := unix.Fsync(directoryFD); err != nil {
			return fmt.Errorf("sync vault removal: %w", err)
		}
		return nil
	})
}

func (store *Store) checkExpectedHash(directoryFD int, expectedHash *[sha256.Size]byte) error {
	current, err := store.loadCiphertext(directoryFD)
	return storage.CheckHead(expectedHash, current, err)
}

func splitVaultPath(path string) (string, string, error) {
	if !filepath.IsAbs(path) {
		return "", "", fmt.Errorf("%w: vault path must be absolute", storage.ErrInvalidPath)
	}
	cleaned := filepath.Clean(path)
	directory, name := filepath.Split(cleaned)
	directory = filepath.Clean(directory)
	if name == "" || name == "." || name == ".." || directory == cleaned {
		return "", "", fmt.Errorf("%w: vault path must name a file", storage.ErrInvalidPath)
	}
	// Dot names are reserved for the lock and interrupted-write files.
	if strings.HasPrefix(name, ".") || len(name) > maxNameLength {
		return "", "", fmt.Errorf("%w: vault file name is not supported", storage.ErrInvalidPath)
	}
	return directory, name, nil
}

func (store *Store) openDirectory() (int, error) {
	fd, err := unix.Open(store.directory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, fmt.Errorf("open vault directory: %w", err)
	}
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("inspect vault directory: %w", err)
	}
	if info.Mode&unix.S_IFMT != unix.S_IFDIR {
		unix.Close(fd)
		return -1, ErrUnsafePermissions
	}
	return fd, nil
}

func (store *Store) withLock(directoryFD int, work func() error) error {
	fd, err := unix.Openat(
		directoryFD,
		store.lockName,
		unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0600,
	)
	if err != nil {
		return fmt.Errorf("open vault lock: %w", err)
	}
	defer unix.Close(fd)

	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return fmt.Errorf("inspect vault lock: %w", err)
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG {
		return ErrUnsafePermissions
	}
	if store.restricted && info.Mode&0777 != 0600 {
		if err := restrict(fd, &info); err != nil {
			return err
		}
	}
	if err := unix.Flock(fd, unix.LOCK_EX); err != nil {
		return fmt.Errorf("lock vault directory: %w", err)
	}
	return work()
}

func (store *Store) save(directoryFD int, ciphertext []byte) error {
	tempName, err := store.temporaryName()
	if err != nil {
		return err
	}
	fd, err := unix.Openat(
		directoryFD,
		tempName,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0600,
	)
	if err != nil {
		return fmt.Errorf("create temporary vault file: %w", err)
	}
	// Fails with ENOENT once the rename took the name; cleanupTemps removes any other leftover.
	defer unix.Unlinkat(directoryFD, tempName, 0)
	if err := unix.Fchmod(fd, 0600); err != nil && store.restricted {
		return errors.Join(fmt.Errorf("protect temporary vault file: %w", err), unix.Close(fd))
	}

	file := os.NewFile(uintptr(fd), tempName)
	writeErr := writeDurably(file, ciphertext)
	closeErr := file.Close()
	if writeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close temporary vault file: %w", closeErr)
	}
	if err := unix.Renameat(directoryFD, tempName, directoryFD, store.name); err != nil {
		return fmt.Errorf("replace vault file: %w", err)
	}
	if err := unix.Fsync(directoryFD); err != nil {
		return fmt.Errorf("%w: %v", storage.ErrDurabilityUncertain, err)
	}
	return nil
}

func writeDurably(file *os.File, ciphertext []byte) error {
	written, err := file.Write(ciphertext)
	if err != nil {
		return fmt.Errorf("write temporary vault file: %w", err)
	}
	if written != len(ciphertext) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temporary vault file: %w", err)
	}
	if err := flushDevice(file.Fd()); err != nil {
		return fmt.Errorf("fully sync temporary vault file: %w", err)
	}
	return nil
}

func (store *Store) temporaryName() (string, error) {
	var random [tempSuffixLength / 2]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("name temporary vault file: %w", err)
	}
	return store.tempBase + hex.EncodeToString(random[:]), nil
}

// probeRestriction reports whether the location keeps a 0600 mode; exFAT and many network volumes do not.
func probeRestriction(directoryFD int, tempBase string) (bool, error) {
	var random [tempSuffixLength / 2]byte
	if _, err := rand.Read(random[:]); err != nil {
		return false, fmt.Errorf("name permission probe: %w", err)
	}
	name := tempBase + hex.EncodeToString(random[:])
	fd, err := unix.Openat(
		directoryFD,
		name,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0600,
	)
	if err != nil {
		return false, fmt.Errorf("write to vault location: %w", err)
	}
	defer unix.Unlinkat(directoryFD, name, 0)
	defer unix.Close(fd)

	if err := unix.Fchmod(fd, 0600); err != nil {
		return false, nil
	}
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return false, fmt.Errorf("inspect permission probe: %w", err)
	}
	return info.Mode&0777 == 0600, nil
}

func unsupportedOperation(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EINVAL)
}

func restrict(fd int, info *unix.Stat_t) error {
	if err := unix.Fchmod(fd, 0600); err != nil {
		return ErrUnsafePermissions
	}
	if err := unix.Fstat(fd, info); err != nil {
		return fmt.Errorf("inspect restricted file: %w", err)
	}
	if info.Mode&0777 != 0600 {
		return ErrUnsafePermissions
	}
	return nil
}

func (store *Store) cleanupTemps(directoryFD int) error {
	duplicate, err := unix.Openat(directoryFD, ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open vault directory entries: %w", err)
	}
	directory := os.NewFile(uintptr(duplicate), "vault directory")
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return fmt.Errorf("read vault directory entries: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close vault directory entries: %w", closeErr)
	}

	removed := false
	for _, entry := range entries {
		name := entry.Name()
		if !store.isTempName(name) {
			continue
		}
		var info unix.Stat_t
		if err := unix.Fstatat(directoryFD, name, &info, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			if errors.Is(err, unix.ENOENT) {
				continue
			}
			return fmt.Errorf("inspect interrupted write: %w", err)
		}
		if info.Mode&unix.S_IFMT != unix.S_IFREG {
			continue
		}
		if err := unix.Unlinkat(directoryFD, name, 0); err != nil {
			return fmt.Errorf("remove interrupted write: %w", err)
		}
		removed = true
	}
	if removed {
		if err := unix.Fsync(directoryFD); err != nil {
			return fmt.Errorf("sync interrupted write cleanup: %w", err)
		}
	}
	return nil
}

func (store *Store) isTempName(name string) bool {
	if !strings.HasPrefix(name, store.tempBase) || len(name) != len(store.tempBase)+tempSuffixLength {
		return false
	}
	for _, char := range name[len(store.tempBase):] {
		isDigit := char >= '0' && char <= '9'
		isHexLetter := char >= 'a' && char <= 'f'
		if !isDigit && !isHexLetter {
			return false
		}
	}
	return true
}
