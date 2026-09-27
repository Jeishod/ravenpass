package vaultservice

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dortanes/ravenpass/packages/app/devicerecords"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	exportRecordVersion byte = 1
	exportRecordSize    int  = 57
)

var errExportRecordCorrupt = errors.New("export record is invalid")

// ExportState is whether a verified export matches the open vault's head.
type ExportState uint8

// Export states reported by Service.ExportState.
const (
	ExportUnknown ExportState = iota + 1
	ExportCurrent
	ExportStale
)

// exportRecord proves an export of one head was written; digest covers the file bytes, not the session.
type exportRecord struct {
	vaultID  vault.ID
	revision uint64
	digest   [32]byte
}

func (r exportRecord) encode() []byte {
	data := make([]byte, exportRecordSize)
	data[0] = exportRecordVersion
	copy(data[1:17], r.vaultID[:])
	binary.BigEndian.PutUint64(data[17:25], r.revision)
	copy(data[25:exportRecordSize], r.digest[:])
	return data
}

func decodeExportRecord(data []byte, vaultID string) (exportRecord, error) {
	if len(data) != exportRecordSize || data[0] != exportRecordVersion {
		return exportRecord{}, errExportRecordCorrupt
	}
	var record exportRecord
	copy(record.vaultID[:], data[1:17])
	record.revision = binary.BigEndian.Uint64(data[17:25])
	copy(record.digest[:], data[25:exportRecordSize])
	if record.vaultID.String() != vaultID || record.revision == 0 {
		return exportRecord{}, errExportRecordCorrupt
	}
	return record, nil
}

// RecordExport stores proof that head was exported as bytes whose on-disk digest is fileDigest.
func (s *Service) RecordExport(head vault.Head, fileDigest [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	current, err := s.session.Head()
	if err != nil {
		return err
	}
	if current != head {
		return ErrStaleExport
	}
	record := exportRecord{vaultID: head.VaultID, revision: head.Revision, digest: fileDigest}
	name := head.VaultID.String()
	if err := s.keys.SaveExportRecord(name, record.encode()); err != nil {
		return fmt.Errorf("store export record: %w", err)
	}
	stored, err := s.readExportRecord(name)
	if err != nil {
		return err
	}
	if stored == nil || *stored != record {
		return fmt.Errorf("verify export record: %w", errExportRecordCorrupt)
	}
	s.device.rememberExport(stored)
	return nil
}

// ExportState reports ExportUnknown, never ExportCurrent, for a missing or unreadable record.
func (s *Service) ExportState() (ExportState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return 0, ErrNotReady
	}
	head, err := s.session.Head()
	if err != nil {
		return 0, err
	}
	record, err := s.loadExportRecord(head.VaultID.String())
	if err != nil || record == nil {
		return ExportUnknown, nil
	}
	if record.vaultID == head.VaultID && record.revision == head.Revision && record.digest == head.Hash {
		return ExportCurrent, nil
	}
	return ExportStale, nil
}

func (s *Service) loadExportRecord(vaultID string) (*exportRecord, error) {
	if s.device.exportLoaded {
		return s.device.export, nil
	}
	record, err := s.readExportRecord(vaultID)
	if err != nil {
		return nil, err
	}
	s.device.rememberExport(record)
	return record, nil
}

func (s *Service) readExportRecord(vaultID string) (*exportRecord, error) {
	data, err := s.keys.LoadExportRecord(vaultID)
	if errors.Is(err, devicerecords.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record, err := decodeExportRecord(data, vaultID)
	if err != nil {
		return nil, err
	}
	return &record, nil
}
