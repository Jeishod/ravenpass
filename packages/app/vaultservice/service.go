// Package vaultservice holds the open vault, its storage and this device's records behind one lock.
package vaultservice

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/changecount"
	"github.com/dortanes/ravenpass/packages/app/devicerecords"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/vault"
)

const witnessSize = 57

// Errors returned by Service.
var (
	ErrNotReady           = errors.New("vault is not unlocked")
	ErrSetupInProgress    = errors.New("vault setup is in progress")
	ErrAlreadyInitialized = errors.New("vault already exists")
	ErrNoPendingSetup     = errors.New("vault setup has not started")
	ErrRecoveryChanged    = errors.New("vault changed during recovery")
	ErrStaleExport        = errors.New("vault changed; unlock again before exporting")
	ErrConfirmationNeeded = errors.New("recovery may lose newer changes and requires explicit confirmation")
	ErrWitnessCorrupt     = errors.New("vault witness is invalid")
	ErrMoveVerification   = errors.New("vault at the new location could not be verified")
	ErrSecondCopy         = errors.New("another known vault holds the same vault identity")
	ErrOlderCopy          = errors.New("this file is older than the vault this device used")
	ErrDiverged           = errors.New("this vault was changed on another device at the same time")
	ErrNoOtherVault       = errors.New("no other vault to return to")
)

// CiphertextStore is one bound storage location.
type CiphertextStore = storage.Ciphertext

// Storage is the selected vault location, which it can report, rebind, relocate and erase.
type Storage interface {
	CiphertextStore
	Status() storage.Status
	Open() error
	Bind(target storage.Target) error
	// Identify records which vault the bound location holds.
	Identify(vault string) error
	Relocate(target storage.Target, write func(storage.Ciphertext) error) (storage.Relocation, error)
	Forget(target storage.Target) error
	Erase(target storage.Target) error
	LoadFrom(target storage.Target) ([]byte, error)
}

// KeyStore keeps this device's records of each vault; a missing record loads as devicerecords.ErrNotFound.
type KeyStore interface {
	SaveHeadWitness(vaultID string, witness []byte) error
	LoadHeadWitness(vaultID string) ([]byte, error)
	DeleteHeadWitness(vaultID string) error
	SaveUsageRecord(vaultID string, usage []byte) error
	LoadUsageRecord(vaultID string) ([]byte, error)
	DeleteUsageRecord(vaultID string) error
	SaveExportRecord(vaultID string, record []byte) error
	LoadExportRecord(vaultID string) ([]byte, error)
	DeleteExportRecord(vaultID string) error
	SaveUnlockPolicy(vaultID string, policy []byte) error
	LoadUnlockPolicy(vaultID string) ([]byte, error)
	DeleteUnlockPolicy(vaultID string) error
}

// Owner reports, without prompting, whether this device can authenticate its owner.
type Owner interface {
	DeviceOwnerAvailable() bool
}

// Device is the owner check and the hardware bindings for a PIN and device authentication.
type Device struct {
	Owner    Owner
	PIN      unlock.Binding
	Platform unlock.PresenceBinding
	// PINKey derives a PIN's wrapping key; nil uses vault.DerivePINKey.
	PINKey unlock.PINKey
}

// Phase is the stage of the vault's lifecycle the service is in.
type Phase uint8

// Phases reported by Service.State.
const (
	PhaseLocked Phase = iota + 1
	PhaseCreating
	PhaseRecovering
	PhaseReady
	// PhaseDiverged holds a vault opened from a file that diverged from this device's acknowledged version.
	PhaseDiverged
)

// State is the current phase and, where known, the vault's head.
type State struct {
	Phase       Phase
	VaultExists bool
	// VaultMissing reports a locked location that no longer holds the vault this device opened there.
	VaultMissing bool
	Head         vault.Head
}

// RecoveryPreview describes a staged recovery before its owner confirms it.
type RecoveryPreview struct {
	Head                    vault.Head
	MayLoseNewerCredentials bool
	// NeedsWayIn reports that the device holds no usable way into the vault, so confirming needs a choice.
	NeedsWayIn bool
}

type stagedCreation struct {
	session   *vault.Session
	container []byte
}

type stagedRecovery struct {
	session       *vault.Session
	head          vault.Head
	potentialLoss bool
}

// Service owns the open vault session and serializes every operation on it through mu.
type Service struct {
	mu         sync.Mutex
	files      Storage
	keys       KeyStore
	owner      Owner
	pins       *unlock.PINs
	platforms  *unlock.PlatformCredentials
	session    *vault.Session
	creating   *stagedCreation
	recovering *stagedRecovery
	diverged   *stagedDivergence
	rekeying   *stagedRekey
	// returnTo is the vault that was current when setting up another one began.
	returnTo    storage.Target
	device      deviceState
	pinThrottle *unlock.Throttle
	// changes counts extension and autofill saves plus adopted versions from other devices.
	changes changecount.Counter
	// states counts every save, every open or lock, and every change of current location.
	states changecount.Counter
}

// New returns a locked Service over files, keys and device.
func New(files Storage, keys KeyStore, device Device) (*Service, error) {
	if files == nil || keys == nil || device.Owner == nil {
		return nil, errors.New("vault dependencies are required")
	}
	pins, err := unlock.NewPINs(device.PIN, device.PINKey)
	if err != nil {
		return nil, err
	}
	platforms, err := unlock.NewPlatformCredentials(device.Platform)
	if err != nil {
		return nil, err
	}
	return &Service{files: files, keys: keys, owner: device.Owner, pins: pins, platforms: platforms, pinThrottle: unlock.NewThrottle(time.Now)}, nil
}

// State reports the current phase and whether the bound location holds a vault.
func (s *Service) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		head, err := s.session.Head()
		if err != nil {
			return State{}, err
		}
		return State{Phase: PhaseReady, VaultExists: true, Head: head}, nil
	}
	if s.creating != nil {
		return State{Phase: PhaseCreating}, nil
	}
	if s.recovering != nil {
		return State{Phase: PhaseRecovering, VaultExists: true, Head: s.recovering.head}, nil
	}
	if s.diverged != nil {
		return State{Phase: PhaseDiverged, VaultExists: true}, nil
	}
	_, err := s.files.LoadCiphertext()
	if errors.Is(err, storage.ErrNotFound) {
		missing, err := s.lostVault()
		if err != nil {
			return State{}, err
		}
		return State{Phase: PhaseLocked, VaultMissing: missing}, nil
	}
	if err != nil {
		return State{}, err
	}
	return State{Phase: PhaseLocked, VaultExists: true}, nil
}

// VaultMissing reports whether the bound location no longer holds the vault this device opened there and keeps an
// unlock record for.
func (s *Service) VaultMissing() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files.Status().Current.Vault == "" {
		return false, nil
	}
	container, err := s.files.LoadCiphertext()
	clear(container)
	if !errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return s.lostVault()
}

// lostVault reports whether this device keeps an unlock record for the vault last opened at the bound location,
// which the caller found empty. The caller holds s.mu.
func (s *Service) lostVault() (bool, error) {
	recorded := s.files.Status().Current.Vault
	if recorded == "" {
		return false, nil
	}
	id, err := vault.ParseID(recorded)
	if err != nil {
		return false, nil
	}
	record, err := s.keys.LoadUnlockPolicy(id.String())
	clear(record)
	switch {
	case errors.Is(err, devicerecords.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// openedHere reports whether the selection records a vault opened at the bound location; its file can read as absent
// while it still syncs, so no new vault is written there. The caller holds s.mu.
func (s *Service) openedHere() bool {
	return s.files.Status().Current.Vault != ""
}

// identifyBound records the vault just opened as the one its location holds; failing loses only that record.
func (s *Service) identifyBound(id vault.ID) {
	if err := s.files.Identify(id.String()); err != nil {
		slog.Warn("record which vault the location holds", "err", err)
	}
}

// BeginCreation stages a new vault in memory and returns its recovery phrase.
func (s *Service) BeginCreation() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil || s.creating != nil || s.recovering != nil {
		return "", ErrSetupInProgress
	}
	if _, err := s.files.LoadCiphertext(); err == nil {
		return "", ErrAlreadyInitialized
	} else if !errors.Is(err, storage.ErrNotFound) {
		return "", err
	}
	if s.openedHere() {
		return "", ErrAlreadyInitialized
	}
	created, err := vault.Create()
	if err != nil {
		return "", err
	}
	s.creating = &stagedCreation{session: created.Session, container: created.Container}
	return created.RecoveryPhrase, nil
}

// MethodChoice is how a created or recovered vault opens on the device; PIN is empty when not chosen.
type MethodChoice struct {
	Biometry bool
	PIN      string
}

// Empty reports a choice that names no way in.
func (c MethodChoice) Empty() bool { return !c.Biometry && c.PIN == "" }

// checkChoice refuses an empty choice, an invalid PIN, or device authentication the device lacks.
func (s *Service) checkChoice(choice MethodChoice) error {
	switch {
	case choice.Empty():
		return unlock.ErrNoMethodLeft
	case choice.PIN != "" && !vault.ValidPIN(choice.PIN):
		return vault.ErrInvalidPIN
	case choice.Biometry && !s.owner.DeviceOwnerAvailable():
		return unlock.ErrNotAvailable
	}
	return nil
}

// bindChoice makes the hardware keys a checked choice names and wraps the vault key for each.
func (s *Service) bindChoice(id vault.ID, choice MethodChoice, wrap func(key []byte) ([]byte, error)) (unlock.Policy, error) {
	var policy unlock.Policy
	var err error
	if choice.Biometry {
		if policy.Platform, err = s.platforms.Set(id, wrap); err != nil {
			return unlock.Policy{}, err
		}
	}
	if choice.PIN != "" {
		if policy.PIN, err = s.pins.Set(choice.PIN, id, wrap); err != nil {
			return unlock.Policy{}, err
		}
	}
	return policy, nil
}

// ConfirmCreation commits the staged vault, bound to choice, once phrase matches its recovery key.
func (s *Service) ConfirmCreation(phrase string, choice MethodChoice) (vault.Head, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stage := s.creating
	if stage == nil {
		return vault.Head{}, ErrNoPendingSetup
	}
	if err := s.verifyCreationPhrase(phrase); err != nil {
		return vault.Head{}, err
	}
	if err := s.checkChoice(choice); err != nil {
		return vault.Head{}, err
	}
	if s.openedHere() {
		s.discardStaging()
		return vault.Head{}, ErrAlreadyInitialized
	}
	head, err := stage.session.Head()
	if err != nil {
		return vault.Head{}, err
	}
	id := head.VaultID.String()
	policy, err := s.bindChoice(head.VaultID, choice, stage.session.WrapDeviceKey)
	if err != nil {
		s.discardStaging()
		return vault.Head{}, err
	}
	if err := s.savePolicy(head.VaultID, policy); err != nil {
		s.discardStaging()
		return vault.Head{}, err
	}
	if err := s.files.CommitCiphertext(nil, stage.container, func() error {
		previous, err := s.loadWitness(id)
		if err != nil {
			return err
		}
		if previous != nil {
			return vault.ErrWitnessMismatch
		}
		return s.saveVerifiedWitness(head)
	}); err != nil {
		s.discardStaging()
		return vault.Head{}, fmt.Errorf("create vault: %w", err)
	}
	s.session = stage.session
	s.states.Record()
	s.identifyBound(head.VaultID)
	clear(stage.container)
	s.creating = nil
	return head, nil
}

// Unlock opens the bound vault with device authentication, showing reason in the system prompt.
func (s *Service) Unlock(reason string) (vault.Head, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil || s.creating != nil || s.recovering != nil {
		return vault.Head{}, ErrSetupInProgress
	}
	s.discardDivergence()
	container, err := s.files.LoadCiphertext()
	if err != nil {
		return vault.Head{}, err
	}
	id, err := vault.InspectUntrustedVaultID(container)
	if err != nil {
		return vault.Head{}, err
	}
	name := id.String()
	policy, err := s.loadPolicy(id)
	if err != nil {
		return vault.Head{}, err
	}
	if !policy.HasPlatform() {
		return vault.Head{}, unlock.ErrDisabled
	}
	if err := vault.CheckDeviceEnvelope(container, policy.Platform.Envelope); err != nil {
		if errors.Is(err, vault.ErrKeyReplaced) {
			return vault.Head{}, err
		}
		policy.Platform = nil
		s.device.rememberPolicy(name, policy)
		return vault.Head{}, fmt.Errorf("%w: %w", unlock.ErrUnbound, err)
	}
	key, err := s.platforms.Key(policy.Platform, id, reason)
	// A credential whose hardware key the device no longer accepts is treated as turned off.
	if errors.Is(err, unlock.ErrUnbound) {
		policy.Platform = nil
		s.device.rememberPolicy(name, policy)
		return vault.Head{}, err
	}
	if err != nil {
		return vault.Head{}, err
	}
	defer clear(key[:])
	opened, err := s.openWithKey(container, name, key[:], policy.Platform.Envelope)
	if err != nil {
		return vault.Head{}, err
	}
	return s.adoptSession(opened)
}

// openWithKey opens the bound vault for every unlock method, failing with ErrOlderCopy or ErrDiverged.
func (s *Service) openWithKey(container []byte, name string, key, envelope []byte) (*vault.Session, error) {
	witness, err := s.loadWitness(name)
	if err != nil {
		return nil, err
	}
	opened, divergence, err := vault.OpenWithDevice(container, key, envelope, witness, s.advanceWitness(name, witness))
	switch {
	case divergence != nil:
		s.diverged = &stagedDivergence{divergence: divergence, name: name, witness: witness}
		return nil, ErrDiverged
	case errors.Is(err, vault.ErrWitnessOlder):
		return nil, fmt.Errorf("%w: %w", ErrOlderCopy, err)
	case err != nil:
		return nil, err
	}
	return opened, nil
}

// advanceWitness saves a newer head as acknowledged only while the file holds it and the witness is still from.
func (s *Service) advanceWitness(name string, from *vault.Witness) func(vault.Witness) error {
	return func(advanced vault.Witness) error {
		return s.files.ReconcileCiphertext(advanced.Hash, func() error {
			current, err := s.loadWitness(name)
			if err != nil {
				return err
			}
			if current == nil || from == nil || *current != *from {
				return vault.ErrWitnessMismatch
			}
			return s.saveVerifiedWitness(vault.Head{VaultID: advanced.VaultID, Revision: advanced.Revision, Hash: advanced.Hash})
		})
	}
}

func (s *Service) adoptSession(opened *vault.Session) (vault.Head, error) {
	head, err := opened.Head()
	if err != nil {
		opened.Lock()
		return vault.Head{}, err
	}
	s.session = opened
	s.states.Record()
	s.identifyBound(head.VaultID)
	return head, nil
}

// BeginRecovery opens the bound vault with its recovery phrase and stages it for confirmation.
func (s *Service) BeginRecovery(phrase string) (RecoveryPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil || s.creating != nil || s.recovering != nil {
		return RecoveryPreview{}, ErrSetupInProgress
	}
	container, err := s.files.LoadCiphertext()
	if err != nil {
		return RecoveryPreview{}, err
	}
	opened, err := vault.OpenWithRecovery(container, phrase)
	if err != nil {
		return RecoveryPreview{}, err
	}
	head, err := opened.Head()
	if err != nil {
		opened.Lock()
		return RecoveryPreview{}, err
	}
	witness, err := s.loadWitness(head.VaultID.String())
	if err != nil {
		opened.Lock()
		return RecoveryPreview{}, err
	}
	potentialLoss := witness == nil
	if witness != nil {
		_, err = opened.ReconcileWitness(witness)
		potentialLoss = err != nil
	}
	s.recovering = &stagedRecovery{session: opened, head: head, potentialLoss: potentialLoss}
	return RecoveryPreview{
		Head:                    head,
		MayLoseNewerCredentials: potentialLoss,
		NeedsWayIn:              !s.keptPolicy(opened).Usable(s.owner.DeviceOwnerAvailable()),
	}, nil
}

// ConfirmRecovery commits the staged recovery; an empty choice keeps the device's usable ways in.
func (s *Service) ConfirmRecovery(acceptPossibleDataLoss bool, choice MethodChoice) (vault.Head, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stage := s.recovering
	if stage == nil {
		return vault.Head{}, ErrNoPendingSetup
	}
	if choice.Empty() {
		if !s.keptPolicy(stage.session).Usable(s.owner.DeviceOwnerAvailable()) {
			return vault.Head{}, unlock.ErrNoMethodLeft
		}
	} else if err := s.checkChoice(choice); err != nil {
		return vault.Head{}, err
	}
	currentContainer, err := s.files.LoadCiphertext()
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		s.discardStaging()
		return vault.Head{}, err
	}
	if err != nil || sha256.Sum256(currentContainer) != stage.head.Hash {
		s.discardStaging()
		return vault.Head{}, ErrRecoveryChanged
	}
	id := stage.head.VaultID.String()
	witness, err := s.loadWitness(id)
	if err != nil {
		s.discardStaging()
		return vault.Head{}, err
	}
	potentialLoss := witness == nil
	if witness != nil {
		_, err = stage.session.ReconcileWitness(witness)
		potentialLoss = err != nil
	}
	if potentialLoss && !acceptPossibleDataLoss {
		stage.potentialLoss = true
		return vault.Head{}, ErrConfirmationNeeded
	}
	if potentialLoss && !stage.potentialLoss {
		stage.potentialLoss = true
		return vault.Head{}, ErrConfirmationNeeded
	}
	finalize := func() error {
		current, loadErr := s.loadWitness(id)
		if loadErr != nil {
			return loadErr
		}
		if !equalWitness(current, witness) {
			return vault.ErrWitnessMismatch
		}
		return s.enrollRecoveredKey(stage, choice)
	}
	if err := s.files.ReconcileCiphertext(stage.head.Hash, finalize); err != nil {
		s.discardStaging()
		return vault.Head{}, err
	}
	s.session = stage.session
	s.states.Record()
	s.identifyBound(stage.head.VaultID)
	s.recovering = nil
	return stage.head, nil
}

// enrollRecoveredKey binds choice, or rebinds the kept device authentication, for the recovered vault.
func (s *Service) enrollRecoveredKey(stage *stagedRecovery, choice MethodChoice) error {
	id := stage.head.VaultID.String()
	if err := s.keys.DeleteUsageRecord(id); err != nil {
		return fmt.Errorf("remove device usage record: %w", err)
	}
	if err := s.keys.DeleteExportRecord(id); err != nil {
		return fmt.Errorf("remove export record: %w", err)
	}
	s.device.forget()
	policy := s.keptPolicy(stage.session)
	if !choice.Empty() {
		var err error
		if policy, err = s.bindChoice(stage.head.VaultID, choice, stage.session.WrapDeviceKey); err != nil {
			return err
		}
	} else if policy.HasPlatform() {
		platform, err := s.platforms.Set(stage.head.VaultID, stage.session.WrapDeviceKey)
		if err != nil {
			return err
		}
		policy.Platform = platform
	}
	if err := s.savePolicy(stage.head.VaultID, policy); err != nil {
		return err
	}
	return s.saveVerifiedWitness(stage.head)
}

// keptPolicy is the ways in the device holds for the vault opened shows, or none where its record cannot be read.
// A PIN holding a replaced vault key is dropped; device authentication is bound again on recovery.
func (s *Service) keptPolicy(opened *vault.Session) unlock.Policy {
	head, err := opened.Head()
	if err != nil {
		return unlock.Policy{}
	}
	policy, err := s.loadPolicy(head.VaultID)
	if err != nil {
		return unlock.Policy{}
	}
	if policy.HasPIN() && opened.CheckDeviceEnvelope(policy.PIN.Envelope) != nil {
		policy.PIN = nil
	}
	return policy
}

func equalWitness(a, b *vault.Witness) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// List returns the entries of the open vault.
func (s *Service) List() ([]vault.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, ErrNotReady
	}
	return s.session.List()
}

// Select issues a selection ticket for reading the item id.
func (s *Service) Select(id vault.ID) (vault.Selection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.Selection{}, ErrNotReady
	}
	return s.session.BeginSelection(id)
}

// ReadSelected returns the credential ticket selects.
func (s *Service) ReadSelected(ticket vault.Selection) (vault.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.Credential{}, ErrNotReady
	}
	return s.session.ReadSelected(ticket)
}

// ClearSelection invalidates the current selection ticket.
func (s *Service) ClearSelection() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		s.session.ClearSelection()
	}
}

// CreateCredential saves a new credential in groups and returns its ID.
func (s *Service) CreateCredential(input vault.CredentialInput, groups []vault.ID) (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createCredential(input, groups)
}

// createCredential is CreateCredential for a caller that holds s.mu.
func (s *Service) createCredential(input vault.CredentialInput, groups []vault.ID) (vault.ID, error) {
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	pending, id, err := s.session.PrepareCreate(input, groups)
	if err != nil {
		return vault.ID{}, err
	}
	if err := s.commit(pending); err != nil {
		return vault.ID{}, err
	}
	return id, nil
}

// EditCredential applies patch to the credential id.
func (s *Service) EditCredential(id vault.ID, patch vault.CredentialPatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.editCredential(id, patch)
}

// editCredential is EditCredential for a caller that holds s.mu.
func (s *Service) editCredential(id vault.ID, patch vault.CredentialPatch) error {
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareEdit(id, patch)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// SetPinned pins or unpins the item id.
func (s *Service) SetPinned(id vault.ID, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareSetPinned(id, pinned)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// DeleteItem removes an item of either kind.
func (s *Service) DeleteItem(id vault.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return ErrNotReady
	}
	pending, err := s.session.PrepareDelete(id)
	if err != nil {
		return err
	}
	return s.commit(pending)
}

// Export returns the open vault's container once the file and device witness still match its head.
func (s *Service) Export() ([]byte, vault.Head, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, vault.Head{}, ErrNotReady
	}
	container, head, err := s.session.Export()
	if err != nil {
		s.lockSession()
		return nil, vault.Head{}, err
	}
	err = s.files.ReconcileCiphertext(head.Hash, func() error {
		witness, loadErr := s.loadWitness(head.VaultID.String())
		if loadErr != nil {
			return loadErr
		}
		if witness == nil || *witness != vault.WitnessFor(head) {
			return vault.ErrWitnessMismatch
		}
		return nil
	})
	if err != nil {
		s.lockSession()
		if errors.Is(err, storage.ErrStaleHead) || errors.Is(err, vault.ErrWitnessMismatch) || errors.Is(err, ErrWitnessCorrupt) {
			return nil, vault.Head{}, fmt.Errorf("%w: %w", ErrStaleExport, err)
		}
		return nil, vault.Head{}, err
	}
	return container, head, nil
}

// Unlocked reports whether a vault is open, without reading storage.
func (s *Service) Unlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session != nil
}

// Lock closes the open vault and discards any staged setup.
func (s *Service) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockSession()
	s.discardStaging()
}

// Storage reports where the vault is kept current.
func (s *Service) Storage() storage.Status {
	return s.files.Status()
}

// OpenStorage binds the recorded location again after it could not be reached.
func (s *Service) OpenStorage() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.files.Open()
}

// BindStorage points at a location without moving a vault, and is refused while one is open or staged.
func (s *Service) BindStorage(target storage.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil || s.creating != nil || s.recovering != nil {
		return ErrSetupInProgress
	}
	if err := s.files.Bind(target); err != nil {
		return err
	}
	s.states.Record()
	return nil
}

// MoveStorage verifies the open vault against its head and witness, then relocates it to target.
func (s *Service) MoveStorage(target storage.Target) (storage.Relocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return storage.Relocation{}, ErrNotReady
	}
	head, err := s.session.Head()
	if err != nil {
		s.lockSession()
		return storage.Relocation{}, err
	}
	container, err := s.files.LoadCiphertext()
	if errors.Is(err, storage.ErrNotFound) {
		s.lockSession()
		return storage.Relocation{}, fmt.Errorf("%w: %w", storage.ErrStaleHead, storage.ErrVaultMissing)
	}
	if err != nil {
		return storage.Relocation{}, err
	}
	defer clear(container)
	if sha256.Sum256(container) != head.Hash {
		s.lockSession()
		return storage.Relocation{}, storage.ErrStaleHead
	}
	witness, err := s.loadWitness(head.VaultID.String())
	if err != nil {
		return storage.Relocation{}, err
	}
	if witness == nil || *witness != vault.WitnessFor(head) {
		s.lockSession()
		return storage.Relocation{}, vault.ErrWitnessMismatch
	}
	return s.files.Relocate(target, func(destination storage.Ciphertext) error {
		if err := destination.CommitCiphertext(nil, container, func() error { return nil }); err != nil {
			return err
		}
		written, err := destination.LoadCiphertext()
		if err != nil {
			return err
		}
		defer clear(written)
		if !bytes.Equal(written, container) {
			return ErrMoveVerification
		}
		return nil
	})
}

func (s *Service) commit(pending *vault.Pending) error {
	return s.commitThen(pending, nil)
}

// commitThen stores pending and, while the storage still holds it, records its witness and then runs after when set.
func (s *Service) commitThen(pending *vault.Pending, after func() error) error {
	previous, err := s.session.Head()
	if err != nil {
		s.lockSession()
		return err
	}
	id := previous.VaultID.String()
	witness, err := s.loadWitness(id)
	if err != nil || witness == nil || *witness != vault.WitnessFor(previous) {
		s.lockSession()
		if err != nil {
			return err
		}
		return vault.ErrWitnessMismatch
	}
	next := pending.Head()
	err = s.files.CommitCiphertext(&previous.Hash, pending.Container(), func() error {
		current, loadErr := s.loadWitness(id)
		if loadErr != nil {
			return loadErr
		}
		if current == nil || *current != *witness {
			return vault.ErrWitnessMismatch
		}
		if err := s.saveVerifiedWitness(next); err != nil || after == nil {
			return err
		}
		return after()
	})
	if errors.Is(err, storage.ErrTooLarge) {
		// ErrTooLarge is returned before any write, so the file and session still agree.
		if abortErr := s.session.Abort(pending); abortErr != nil {
			s.lockSession()
			return abortErr
		}
		return err
	}
	if err != nil {
		s.lockSession()
		return err
	}
	if err := s.session.Commit(pending); err != nil {
		s.lockSession()
		return err
	}
	s.states.Record()
	return nil
}

func (s *Service) loadWitness(id string) (*vault.Witness, error) {
	data, err := s.keys.LoadHeadWitness(id)
	if errors.Is(err, devicerecords.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) != witnessSize || data[0] != 1 {
		return nil, ErrWitnessCorrupt
	}
	var witness vault.Witness
	copy(witness.VaultID[:], data[1:17])
	witness.Revision = binary.BigEndian.Uint64(data[17:25])
	copy(witness.Hash[:], data[25:57])
	if witness.VaultID.String() != id || witness.Revision == 0 {
		return nil, ErrWitnessCorrupt
	}
	return &witness, nil
}

func (s *Service) saveVerifiedWitness(head vault.Head) error {
	data := make([]byte, witnessSize)
	data[0] = 1
	copy(data[1:17], head.VaultID[:])
	binary.BigEndian.PutUint64(data[17:25], head.Revision)
	copy(data[25:57], head.Hash[:])
	id := head.VaultID.String()
	if err := s.keys.SaveHeadWitness(id, data); err != nil {
		return fmt.Errorf("store vault witness: %w", err)
	}
	verified, err := s.loadWitness(id)
	if err != nil {
		return err
	}
	if verified == nil || *verified != vault.WitnessFor(head) {
		return ErrWitnessCorrupt
	}
	return nil
}

// lockSession locks the open vault; callers that rebind storage must do so under the same s.mu hold.
func (s *Service) lockSession() {
	s.discardRekey()
	if s.session != nil {
		s.session.Lock()
		s.session = nil
	}
	s.device.forget()
	s.states.Record()
}

func (s *Service) discardStaging() {
	if s.creating != nil {
		s.creating.session.Lock()
		clear(s.creating.container)
		s.creating = nil
	}
	if s.recovering != nil {
		s.recovering.session.Lock()
		s.recovering = nil
	}
	s.discardDivergence()
}
