package vaultservice

import (
	"errors"
	"slices"

	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

// Deletion reports what a permanent deletion left behind on the device.
type Deletion struct {
	Target               storage.Target
	VaultID              vault.ID
	DeviceRecordsRemoved bool
	// SharedIdentity reports that another listed file holds the same vault, so its device records were kept.
	SharedIdentity bool
}

// Vaults lists the vaults this device knows, with the current one.
func (s *Service) Vaults() (vaults []storage.Target, current storage.Target) {
	status := s.files.Status()
	return status.Vaults, status.Current
}

// HoldsAnyVault reports whether any known location holds a vault file; an unreadable one counts as holding one.
func (s *Service) HoldsAnyVault() bool {
	known, _ := s.Vaults()
	for _, location := range known {
		container, err := s.files.LoadFrom(location)
		clear(container)
		if !errors.Is(err, storage.ErrNotFound) {
			return true
		}
	}
	return false
}

// SwitchVault closes the open vault and makes target current, still locked.
func (s *Service) SwitchVault(target storage.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refuseSecondCopy(target); err != nil {
		return err
	}
	s.lockSession()
	s.discardStaging()
	return s.files.Bind(target)
}

// refuseSecondCopy rejects a file carrying the identity of a vault already known at another location.
func (s *Service) refuseSecondCopy(target storage.Target) error {
	known, _ := s.Vaults()
	if slices.ContainsFunc(known, target.Same) {
		return nil
	}
	container, err := s.files.LoadFrom(target)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	id, err := vault.InspectUntrustedVaultID(container)
	clear(container)
	if err != nil {
		return err
	}
	if s.identityIsShared(target, id) {
		return ErrSecondCopy
	}
	return nil
}

// identityIsShared reports whether a known vault other than target holds the same vault.
func (s *Service) identityIsShared(target storage.Target, id vault.ID) bool {
	known, _ := s.Vaults()
	for _, other := range known {
		if other.Same(target) {
			continue
		}
		data, err := s.files.LoadFrom(other)
		if err != nil {
			continue
		}
		otherID, err := vault.InspectUntrustedVaultID(data)
		clear(data)
		if err == nil && otherID == id {
			return true
		}
	}
	return false
}

// PrepareVaultCreation binds an empty location no vault was opened at for setup and remembers the current vault to
// return to.
func (s *Service) PrepareVaultCreation(target storage.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	container, err := s.files.LoadFrom(target)
	switch {
	case err == nil:
		clear(container)
		return ErrAlreadyInitialized
	case !errors.Is(err, storage.ErrNotFound):
		return err
	}
	known, previous := s.Vaults()
	if at := slices.IndexFunc(known, target.Same); at >= 0 && known[at].Vault != "" {
		return ErrAlreadyInitialized
	}
	s.lockSession()
	s.discardStaging()
	if err := s.files.Bind(target); err != nil {
		return err
	}
	s.returnTo = previous
	return nil
}

// FreeLocation reports whether target is unlisted and holds no vault file.
func (s *Service) FreeLocation(target storage.Target) (bool, error) {
	known, _ := s.Vaults()
	if slices.ContainsFunc(known, target.Same) {
		return false, nil
	}
	container, err := s.files.LoadFrom(target)
	clear(container)
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, storage.ErrNotFound):
		return true, nil
	default:
		return false, err
	}
}

// CancelVaultCreation abandons an unwritten setup and rebinds the vault chosen by vaultToReturnTo.
func (s *Service) CancelVaultCreation() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.files.LoadCiphertext(); err == nil {
		return ErrAlreadyInitialized
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	known, _ := s.Vaults()
	back, found := s.vaultToReturnTo(known)
	if !found {
		return ErrNoOtherVault
	}
	s.lockSession()
	s.discardStaging()
	if err := s.files.Bind(back); err != nil {
		return err
	}
	s.returnTo = storage.Target{}
	return nil
}

// vaultToReturnTo prefers s.returnTo, else the first readable known vault. The caller holds s.mu.
func (s *Service) vaultToReturnTo(known []storage.Target) (storage.Target, bool) {
	candidates := known
	if slices.ContainsFunc(known, s.returnTo.Same) {
		candidates = append([]storage.Target{s.returnTo}, known...)
	}
	for _, location := range candidates {
		container, err := s.files.LoadFrom(location)
		clear(container)
		if err == nil {
			return location, true
		}
	}
	return storage.Target{}, false
}

// ForgetVault drops a vault from the known list, keeping its file and device records.
func (s *Service) ForgetVault(target storage.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isCurrent(target) {
		s.lockSession()
		s.discardStaging()
	}
	return s.files.Forget(target)
}

// DeleteVault erases a readable vault's file and the device records named by its identity.
func (s *Service) DeleteVault(target storage.Target) (Deletion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	container, err := s.files.LoadFrom(target)
	if err != nil {
		return Deletion{}, err
	}
	id, err := vault.InspectUntrustedVaultID(container)
	clear(container)
	if err != nil {
		return Deletion{}, err
	}
	// Device records are keyed by vault identity; erasing one of two copies must keep them for the other.
	shared := s.identityIsShared(target, id)
	current := s.isCurrent(target)
	if current {
		s.lockSession()
		s.discardStaging()
	}
	if err := s.files.Erase(target); err != nil {
		return Deletion{}, err
	}
	if shared {
		return Deletion{Target: target, VaultID: id, SharedIdentity: true}, nil
	}

	name := id.String()
	removed := true
	for _, remove := range []func(string) error{
		s.keys.DeleteHeadWitness,
		s.keys.DeleteUsageRecord,
		s.keys.DeleteExportRecord,
		s.keys.DeleteUnlockPolicy,
	} {
		if remove(name) != nil {
			removed = false
		}
	}
	if current {
		s.device.forget()
	}
	return Deletion{Target: target, VaultID: id, DeviceRecordsRemoved: removed}, nil
}

func (s *Service) isCurrent(target storage.Target) bool {
	return s.files.Status().Current.Same(target)
}
