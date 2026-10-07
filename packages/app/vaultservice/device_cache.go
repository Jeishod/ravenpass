package vaultservice

import (
	"log/slog"

	"github.com/dortanes/ravenpass/packages/vault"
)

// OpenVaultID reports the identifier of the open vault.
func (s *Service) OpenVaultID() (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	head, err := s.session.Head()
	return head.VaultID, err
}

// SealDeviceCache seals device cache data with the open vault, or fails with ErrNotReady unless it is id.
func (s *Service) SealDeviceCache(id vault.ID, name string, plaintext []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(id); err != nil {
		return nil, err
	}
	return s.session.SealDeviceCache(name, plaintext)
}

// OpenDeviceCache opens device cache data with the open vault, or fails with ErrNotReady unless it is id.
func (s *Service) OpenDeviceCache(id vault.ID, name string, sealed []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(id); err != nil {
		return nil, err
	}
	return s.session.OpenDeviceCache(name, sealed)
}

func (s *Service) requireOpen(id vault.ID) error {
	if s.session == nil {
		return ErrNotReady
	}
	head, err := s.session.Head()
	if err != nil {
		return err
	}
	if head.VaultID != id {
		return ErrNotReady
	}
	return nil
}

// DeviceData holds the owner's data sealed with the open vault's key on this device; a key change seals it again.
type DeviceData interface {
	// ResealDeviceData opens what it holds for the vault id with open and seals it again with seal, and returns the write
	// that puts the result in place once the new key is saved. It runs while the Service is busy and must not call it.
	ResealDeviceData(id vault.ID, open, seal func(name string, data []byte) ([]byte, error)) (write func() error, err error)
}

// KeepDeviceData has a key change seal holder's data again.
func (s *Service) KeepDeviceData(holder DeviceData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceData = append(s.deviceData, holder)
}

// SealDeviceData seals the owner's device data with the open vault, or fails with ErrNotReady unless it is id.
func (s *Service) SealDeviceData(id vault.ID, name string, plaintext []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(id); err != nil {
		return nil, err
	}
	return s.session.SealDeviceData(name, plaintext)
}

// OpenDeviceData opens the owner's device data with the open vault, or fails with ErrNotReady unless it is id.
func (s *Service) OpenDeviceData(id vault.ID, name string, sealed []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(id); err != nil {
		return nil, err
	}
	return s.session.OpenDeviceData(name, sealed)
}

// resealDeviceData seals every holder's data for the vault id again under rekey's key and returns the writes to run
// once the new key is saved; a holder that fails keeps its data under the old key. The caller holds s.mu.
func (s *Service) resealDeviceData(id vault.ID, rekey *vault.Rekey) []func() error {
	var writes []func() error
	for _, holder := range s.deviceData {
		write, err := holder.ResealDeviceData(id, s.session.OpenDeviceData, rekey.SealDeviceData)
		if err != nil {
			slog.Warn("seal device data for a new vault key", "err", err)
			continue
		}
		if write != nil {
			writes = append(writes, write)
		}
	}
	return writes
}

func writeDeviceData(writes []func() error) {
	for _, write := range writes {
		if err := write(); err != nil {
			slog.Warn("write device data sealed for a new vault key", "err", err)
		}
	}
}
