// Package devicerecords keeps the per-vault witness, usage, export, unlock and vault key records this device holds.
package devicerecords

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/dortanes/ravenpass/packages/app/privatefile"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	recordVersion = 1
	maxFileBytes  = 1 << 20
	maxFieldSize  = 64 << 10
)

var (
	// ErrMalformed reports a records file that cannot be read.
	ErrMalformed = errors.New("device records are malformed")
	// ErrNotFound reports a vault without the requested record.
	ErrNotFound = errors.New("no such device record")
	// ErrInvalidIdentifier reports a vault identifier that is not 32 lowercase hex digits.
	ErrInvalidIdentifier = errors.New("invalid vault identifier")
	// ErrInvalidData reports a record that is empty or too large.
	ErrInvalidData = errors.New("invalid device record")
)

type vaultRecord struct {
	Witness []byte `json:"witness,omitempty"`
	Usage   []byte `json:"usage,omitempty"`
	Export  []byte `json:"export,omitempty"`
	Unlock  []byte `json:"unlock,omitempty"`
	Keys    []byte `json:"keys,omitempty"`
}

func (r vaultRecord) empty() bool {
	return len(r.Witness) == 0 && len(r.Usage) == 0 && len(r.Export) == 0 && len(r.Unlock) == 0 && len(r.Keys) == 0
}

type file struct {
	Version int                    `json:"version"`
	Vaults  map[string]vaultRecord `json:"vaults"`
}

// Store keeps the device records in a private file excluded from backups: a backup must not carry wrapped vault keys beside their bound keys.
type Store struct {
	mu      sync.Mutex
	path    string
	backups privatefile.BackupExclusion
	vaults  map[string]vaultRecord
	loaded  bool
}

// New returns a store for the records file at path.
func New(path string, backups privatefile.BackupExclusion) (*Store, error) {
	if path == "" {
		return nil, errors.New("device records path is required")
	}
	if backups == nil {
		return nil, errors.New("a backup exclusion is required")
	}
	return &Store{path: path, backups: backups}, nil
}

// SaveHeadWitness records the vault head witness for vaultID.
func (s *Store) SaveHeadWitness(vaultID string, witness []byte) error {
	return s.save(vaultID, kindWitness, witness)
}

// LoadHeadWitness returns a copy the caller may wipe; ErrNotFound means vaultID has no witness.
func (s *Store) LoadHeadWitness(vaultID string) ([]byte, error) {
	return s.load(vaultID, kindWitness)
}

// DeleteHeadWitness removes the vault head witness for vaultID.
func (s *Store) DeleteHeadWitness(vaultID string) error {
	return s.remove(vaultID, kindWitness)
}

// SaveUsageRecord records the usage record for vaultID.
func (s *Store) SaveUsageRecord(vaultID string, usage []byte) error {
	return s.save(vaultID, kindUsage, usage)
}

// LoadUsageRecord returns a copy the caller may wipe; ErrNotFound means vaultID has no usage record.
func (s *Store) LoadUsageRecord(vaultID string) ([]byte, error) {
	return s.load(vaultID, kindUsage)
}

// DeleteUsageRecord removes the usage record for vaultID.
func (s *Store) DeleteUsageRecord(vaultID string) error {
	return s.remove(vaultID, kindUsage)
}

// SaveExportRecord records the export record for vaultID.
func (s *Store) SaveExportRecord(vaultID string, record []byte) error {
	return s.save(vaultID, kindExport, record)
}

// LoadExportRecord returns a copy the caller may wipe; ErrNotFound means vaultID has no export record.
func (s *Store) LoadExportRecord(vaultID string) ([]byte, error) {
	return s.load(vaultID, kindExport)
}

// DeleteExportRecord removes the export record for vaultID.
func (s *Store) DeleteExportRecord(vaultID string) error {
	return s.remove(vaultID, kindExport)
}

// SaveUnlockPolicy records the unlock policy for vaultID.
func (s *Store) SaveUnlockPolicy(vaultID string, policy []byte) error {
	return s.save(vaultID, kindUnlock, policy)
}

// LoadUnlockPolicy returns a copy the caller may wipe; ErrNotFound means vaultID has no unlock policy.
func (s *Store) LoadUnlockPolicy(vaultID string) ([]byte, error) {
	return s.load(vaultID, kindUnlock)
}

// DeleteUnlockPolicy removes the unlock policy for vaultID.
func (s *Store) DeleteUnlockPolicy(vaultID string) error {
	return s.remove(vaultID, kindUnlock)
}

// SaveKeyRecord records the vault key record for vaultID.
func (s *Store) SaveKeyRecord(vaultID string, keys []byte) error {
	return s.save(vaultID, kindKeys, keys)
}

// LoadKeyRecord returns a copy the caller may wipe; ErrNotFound means vaultID has no vault key record.
func (s *Store) LoadKeyRecord(vaultID string) ([]byte, error) {
	return s.load(vaultID, kindKeys)
}

// DeleteKeyRecord removes the vault key record for vaultID.
func (s *Store) DeleteKeyRecord(vaultID string) error {
	return s.remove(vaultID, kindKeys)
}

type kind int

const (
	kindWitness kind = iota
	kindUsage
	kindExport
	kindUnlock
	kindKeys
)

func (k kind) of(record *vaultRecord) *[]byte {
	switch k {
	case kindWitness:
		return &record.Witness
	case kindUsage:
		return &record.Usage
	case kindExport:
		return &record.Export
	case kindKeys:
		return &record.Keys
	default:
		return &record.Unlock
	}
}

func (s *Store) save(vaultID string, which kind, data []byte) error {
	if !validVaultID(vaultID) {
		return ErrInvalidIdentifier
	}
	if len(data) == 0 || len(data) > maxFieldSize {
		return ErrInvalidData
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadFile(); err != nil {
		return err
	}
	record := s.vaults[vaultID]
	*which.of(&record) = append([]byte(nil), data...)
	s.vaults[vaultID] = record
	return s.write()
}

func (s *Store) load(vaultID string, which kind) ([]byte, error) {
	if !validVaultID(vaultID) {
		return nil, ErrInvalidIdentifier
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadFile(); err != nil {
		return nil, err
	}
	record := s.vaults[vaultID]
	held := *which.of(&record)
	if len(held) == 0 {
		return nil, ErrNotFound
	}
	return append([]byte(nil), held...), nil
}

func (s *Store) remove(vaultID string, which kind) error {
	if !validVaultID(vaultID) {
		return ErrInvalidIdentifier
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadFile(); err != nil {
		return err
	}
	record, known := s.vaults[vaultID]
	if !known {
		return nil
	}
	held := which.of(&record)
	if len(*held) == 0 {
		return nil
	}
	clear(*held)
	*held = nil
	if record.empty() {
		delete(s.vaults, vaultID)
	} else {
		s.vaults[vaultID] = record
	}
	return s.write()
}

func (s *Store) loadFile() error {
	if s.loaded {
		return nil
	}
	s.vaults = make(map[string]vaultRecord)
	s.loaded = true
	data, err := privatefile.Read(s.path, maxFileBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if errors.Is(err, fs.ErrInvalid) {
		return ErrMalformed
	}
	if err != nil {
		return fmt.Errorf("read device records: %w", err)
	}
	var stored file
	if err := json.Unmarshal(data, &stored); err != nil {
		return ErrMalformed
	}
	if stored.Version != recordVersion {
		return ErrMalformed
	}
	for vaultID, record := range stored.Vaults {
		if validVaultID(vaultID) {
			s.vaults[vaultID] = record
		}
	}
	return nil
}

func (s *Store) write() error {
	data, err := json.Marshal(file{Version: recordVersion, Vaults: s.vaults})
	if err != nil {
		return err
	}
	if err := privatefile.Write(s.path, data, privatefile.ExcludedFromBackups(s.backups)); err != nil {
		return fmt.Errorf("write device records: %w", err)
	}
	return nil
}

func validVaultID(value string) bool {
	id, err := vault.ParseID(value)
	return err == nil && id.String() == value
}
