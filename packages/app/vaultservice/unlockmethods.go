package vaultservice

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/dortanes/ravenpass/packages/app/devicerecords"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/vault"
)

// Methods reports how the bound vault may be opened on the device.
type Methods struct {
	BiometryAvailable bool
	BiometryEnabled   bool
	PINSet            bool
	// PINAttemptsLeft counts the wrong PINs left before the PIN is removed; zero when no PIN is set.
	PINAttemptsLeft int
}

// UnlockMethods reads the ways in for the bound vault, or only device availability when none exists.
func (s *Service) UnlockMethods() (Methods, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	available := s.owner.DeviceOwnerAvailable()
	id, err := s.vaultIdentity()
	if errors.Is(err, storage.ErrNotFound) {
		return Methods{BiometryAvailable: available}, nil
	}
	if err != nil {
		return Methods{}, err
	}
	policy, err := s.loadPolicy(id)
	if err != nil {
		return Methods{}, err
	}
	methods := Methods{
		BiometryAvailable: available,
		BiometryEnabled:   policy.HasPlatform(),
		PINSet:            policy.HasPIN(),
	}
	if policy.PIN != nil {
		methods.PINAttemptsLeft = policy.PIN.Remaining()
	}
	return methods, nil
}

// UnlockWithPIN opens the bound vault with its PIN; each wrong PIN throttles the next and the tenth removes it.
func (s *Service) UnlockWithPIN(pin string) (vault.Head, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil || s.creating != nil || s.recovering != nil {
		return vault.Head{}, ErrSetupInProgress
	}
	s.discardDivergence()
	if !vault.ValidPIN(pin) {
		return vault.Head{}, vault.ErrInvalidPIN
	}
	container, err := s.files.LoadCiphertext()
	if err != nil {
		return vault.Head{}, err
	}
	id, err := vault.InspectUntrustedVaultID(container)
	if err != nil {
		return vault.Head{}, err
	}
	policy, err := s.loadPolicy(id)
	if err != nil {
		return vault.Head{}, err
	}
	if policy.HasPIN() {
		if err := vault.CheckDeviceEnvelope(container, policy.PIN.Envelope); err != nil {
			if errors.Is(err, vault.ErrKeyReplaced) {
				return vault.Head{}, err
			}
			s.forgetPIN(id, policy)
			return vault.Head{}, unlock.ErrNoPIN
		}
	}
	var opened *vault.Session
	err = s.attemptPIN(id, policy, pin, func(key []byte) (err error) {
		opened, err = s.openWithKey(container, id.String(), key, policy.PIN.Envelope)
		return err
	})
	if err != nil {
		if opened != nil {
			opened.Lock()
		}
		return vault.Head{}, err
	}
	return s.adoptSession(opened)
}

// VerifyPIN checks pin against the open vault's PIN, counting failures as UnlockWithPIN does; the wrong PIN that
// removes the PIN also locks the vault, as lockOnPINRemoved says.
func (s *Service) VerifyPIN(pin string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !vault.ValidPIN(pin) {
		return vault.ErrInvalidPIN
	}
	id, policy, err := s.openVaultPolicy()
	if err != nil {
		return err
	}
	err = s.attemptPIN(id, policy, pin, func(key []byte) error {
		return s.session.VerifyDeviceKey(key, policy.PIN.Envelope)
	})
	s.lockOnPINRemoved(err)
	return err
}

// lockOnPINRemoved locks the open vault once err is the wrong PIN that removed its PIN, so whoever typed it cannot set a
// way in of their own. The caller holds s.mu.
func (s *Service) lockOnPINRemoved(err error) {
	if errors.Is(err, unlock.ErrPINRemoved) {
		s.lockSession()
		s.discardStaging()
	}
}

// attemptPIN counts a try that fails with vault.ErrAuthentication as a wrong PIN. The caller holds s.mu.
// The attempt is stored as wrong before it is tried, so a count that cannot be stored refuses the attempt.
func (s *Service) attemptPIN(id vault.ID, policy unlock.Policy, pin string, try func(key []byte) error) error {
	if !policy.HasPIN() {
		return unlock.ErrNoPIN
	}
	if err := s.pinThrottle.Allow(id.String()); err != nil {
		return err
	}
	key, err := s.pins.Key(policy.PIN, pin, id)
	if errors.Is(err, unlock.ErrUnbound) {
		s.forgetPIN(id, policy)
		return unlock.ErrNoPIN
	}
	if err != nil {
		return err
	}
	defer clear(key[:])
	charged, err := s.chargePIN(id, policy)
	if err != nil {
		return err
	}
	if err := try(key[:]); err != nil {
		if errors.Is(err, vault.ErrAuthentication) {
			return s.wrongPIN(id, charged)
		}
		if restoreErr := s.savePolicy(id, policy); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	s.pinThrottle.Record(id.String(), 0)
	cleared := *policy.PIN
	cleared.Failures = 0
	policy.PIN = &cleared
	return s.savePolicy(id, policy)
}

// chargePIN stores policy with one more wrong PIN, or without its PIN when that one is the last allowed.
func (s *Service) chargePIN(id vault.ID, policy unlock.Policy) (unlock.Policy, error) {
	charged := *policy.PIN
	charged.Failures++
	policy.PIN = &charged
	if charged.Failures >= unlock.MaxPINFailures {
		policy.PIN = nil
	}
	if err := s.savePolicy(id, policy); err != nil {
		return unlock.Policy{}, err
	}
	return policy, nil
}

// wrongPIN reports a wrong PIN that chargePIN already stored and starts the delay it earns.
func (s *Service) wrongPIN(id vault.ID, charged unlock.Policy) error {
	if !charged.HasPIN() {
		s.pinThrottle.Record(id.String(), 0)
		return unlock.ErrPINRemoved
	}
	s.pinThrottle.Record(id.String(), charged.PIN.Failures)
	return unlock.ErrWrongPIN
}

// SetPIN binds a new PIN to a new hardware key; the digits derive the wrapping key and are never stored.
func (s *Service) SetPIN(pin string) error {
	return s.changePolicy(func(id vault.ID, policy *unlock.Policy) error {
		setup, err := s.pins.Set(pin, id, s.session.WrapDeviceKey)
		if err != nil {
			return err
		}
		s.pinThrottle.Record(id.String(), 0)
		policy.PIN = setup
		return nil
	})
}

// RemovePIN drops the PIN's copy of the vault key unless the PIN is the device's only way in.
func (s *Service) RemovePIN() error {
	return s.changePolicy(func(_ vault.ID, policy *unlock.Policy) error {
		if !policy.HasPIN() {
			return unlock.ErrNoPIN
		}
		if !policy.HasPlatform() || !s.owner.DeviceOwnerAvailable() {
			return unlock.ErrNoMethodLeft
		}
		policy.PIN = nil
		return nil
	})
}

// SetBiometryUnlock turns device authentication on or off; off deletes the wrapped vault key, not just the option.
func (s *Service) SetBiometryUnlock(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rekeying != nil {
		return ErrSetupInProgress
	}
	id, policy, err := s.openVaultPolicy()
	if err != nil {
		return err
	}
	if policy.HasPlatform() == enabled {
		return nil
	}
	if !enabled {
		if !policy.HasPIN() {
			return unlock.ErrNoMethodLeft
		}
		policy.Platform = nil
		return s.savePolicy(id, policy)
	}
	if !s.owner.DeviceOwnerAvailable() {
		return unlock.ErrNotAvailable
	}
	opened := s.session
	// A StrongBox key takes about 16 s to create on a Pixel 9 Pro XL; the vault stays lockable meanwhile.
	s.mu.Unlock()
	platform, err := s.platforms.Set(id, opened.WrapDeviceKey)
	s.mu.Lock()
	if err != nil {
		return err
	}
	switch {
	case s.session != opened:
		return ErrNotReady
	case s.rekeying != nil:
		return ErrSetupInProgress
	}
	if _, policy, err = s.openVaultPolicy(); err != nil {
		return err
	}
	if policy.HasPlatform() {
		return nil
	}
	policy.Platform = platform
	return s.savePolicy(id, policy)
}

// changePolicy applies one change to the open vault's record and stores the result.
func (s *Service) changePolicy(apply func(id vault.ID, policy *unlock.Policy) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rekeying != nil {
		return ErrSetupInProgress
	}
	id, policy, err := s.openVaultPolicy()
	if err != nil {
		return err
	}
	if err := apply(id, &policy); err != nil {
		return err
	}
	return s.savePolicy(id, policy)
}

func (s *Service) openVaultPolicy() (vault.ID, unlock.Policy, error) {
	if s.session == nil {
		return vault.ID{}, unlock.Policy{}, ErrNotReady
	}
	head, err := s.session.Head()
	if err != nil {
		return vault.ID{}, unlock.Policy{}, err
	}
	policy, err := s.loadPolicy(head.VaultID)
	if err != nil {
		return vault.ID{}, unlock.Policy{}, err
	}
	return head.VaultID, policy, nil
}

func (s *Service) vaultIdentity() (vault.ID, error) {
	if s.session != nil {
		head, err := s.session.Head()
		if err != nil {
			return vault.ID{}, err
		}
		return head.VaultID, nil
	}
	container, err := s.files.LoadCiphertext()
	if err != nil {
		return vault.ID{}, err
	}
	id, err := vault.InspectUntrustedVaultID(container)
	clear(container)
	if err != nil {
		return vault.ID{}, err
	}
	return id, nil
}

// loadPolicy reads a vault's record once and caches it; a PIN whose hardware key is gone is dropped from it.
func (s *Service) loadPolicy(id vault.ID) (unlock.Policy, error) {
	name := id.String()
	if held, known := s.device.knownPolicy(name); known {
		return held, nil
	}
	data, err := s.keys.LoadUnlockPolicy(name)
	if errors.Is(err, devicerecords.ErrNotFound) {
		var policy unlock.Policy
		s.device.rememberPolicy(name, policy)
		return policy, nil
	}
	if err != nil {
		return unlock.Policy{}, err
	}
	policy, err := unlock.Decode(data)
	if err != nil {
		return unlock.Policy{}, err
	}
	if policy.PIN != nil && !s.pins.Bound(policy.PIN, id) {
		s.forgetPIN(id, policy)
		policy.PIN = nil
		return policy, nil
	}
	// The delay the stored wrong PINs earned starts again from the first read after a restart.
	if policy.PIN != nil {
		s.pinThrottle.Record(name, policy.PIN.Failures)
	}
	s.device.rememberPolicy(name, policy)
	return policy, nil
}

// forgetPIN stores policy without its PIN, which this device can no longer use, so the PIN's wrapped vault key and
// hardware key reference are not kept; where the record cannot be stored the PIN is dropped until the next restart.
func (s *Service) forgetPIN(id vault.ID, policy unlock.Policy) {
	policy.PIN = nil
	if err := s.savePolicy(id, policy); err != nil {
		slog.Warn("drop an unusable PIN from the unlock record", "err", err)
		s.device.rememberPolicy(id.String(), policy)
	}
}

func (s *Service) savePolicy(id vault.ID, policy unlock.Policy) error {
	data, err := policy.Encode()
	if err != nil {
		return err
	}
	name := id.String()
	if err := s.keys.SaveUnlockPolicy(name, data); err != nil {
		return fmt.Errorf("store unlock record: %w", err)
	}
	s.device.rememberPolicy(name, policy)
	return nil
}
