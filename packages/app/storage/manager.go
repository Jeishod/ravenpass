package storage

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
)

// Manager keeps the known vaults, binds one as the live Ciphertext store, and moves, forgets or erases them.
type Manager struct {
	mu            sync.Mutex
	record        selectionFile
	backends      []Backend
	defaultTarget Target
	maxBytes      int64
	bound         Store
	list          known
	reason        Reason
}

// NewManager keeps its selection record at selectionPath and offers the kinds of backends, in order.
func NewManager(selectionPath string, defaultTarget Target, maxBytes int64, backends ...Backend) (*Manager, error) {
	if selectionPath == "" {
		return nil, errors.New("storage selection path is required")
	}
	if maxBytes <= 0 {
		return nil, errors.New("vault size limit is invalid")
	}
	for i, backend := range backends {
		if slices.ContainsFunc(backends[:i], func(other Backend) bool { return other.Kind() == backend.Kind() }) {
			return nil, fmt.Errorf("storage type %q is offered twice", backend.Kind())
		}
	}
	m := &Manager{
		record:        selectionFile{path: selectionPath},
		backends:      backends,
		defaultTarget: defaultTarget,
		maxBytes:      maxBytes,
		list:          known{vaults: []Target{defaultTarget}, current: defaultTarget},
	}
	if m.check(defaultTarget) != nil {
		return nil, ErrUnsupportedKind
	}
	return m, nil
}

// Open binds the recorded current vault, else the default; an unopenable location leaves the manager unbound.
func (m *Manager) Open() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bound != nil {
		return nil
	}

	recorded, found, err := m.record.load(m.check)
	if err != nil {
		m.release(ReasonSelectionUnusable)
		m.list = known{}
		return err
	}
	if found {
		m.list = recorded
	}
	if err := m.bindCurrent(); err != nil {
		return err
	}
	if tidy := m.withoutEmpty(m.list); len(tidy.vaults) != len(m.list.vaults) {
		if err := m.record.save(tidy); err != nil {
			return err
		}
		m.list = tidy
	}
	return nil
}

// Bind makes a location current and known without writing a vault; other empty locations no vault was opened at leave
// the list.
func (m *Manager) Bind(target Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	opened, err := m.open(target)
	if err != nil {
		return err
	}
	next := m.list
	next.vaults = slices.Clone(next.vaults)
	switch at := slices.IndexFunc(next.vaults, target.Same); {
	case at < 0:
		if len(next.vaults) >= maxKnownVaults {
			return fmt.Errorf("%w: too many vaults are listed", ErrSelectionInvalid)
		}
		next.vaults = append(next.vaults, target)
	case target.Label == Label{}:
		target = next.vaults[at]
	default:
		target.Vault = next.vaults[at].Vault
		next.vaults[at] = target
	}
	next.current = target
	next = m.withoutEmpty(next)
	if err := m.record.save(next); err != nil {
		return err
	}
	m.list = next
	m.adopt(opened)
	return nil
}

// withoutEmpty drops non-current locations holding no vault, keeping unreadable ones and every one a vault was opened
// at: a synced or evicted file can read as absent while it still exists. The caller holds m.mu.
func (m *Manager) withoutEmpty(list known) known {
	kept := known{current: list.current}
	for _, target := range list.vaults {
		if target.Vault == "" && !target.Same(list.current) && m.discardEmpty(target) {
			continue
		}
		kept.vaults = append(kept.vaults, target)
	}
	return kept
}

// DiscardEmpty deletes the placeholder at target, such as the document a picker created for a move that failed, while
// it holds no vault and is not a known location.
func (m *Manager) DiscardEmpty(target Target) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if target.Same(m.list.current) || slices.ContainsFunc(m.list.vaults, target.Same) {
		return
	}
	m.discardEmpty(target)
}

// discardEmpty reports whether target holds no vault file, discarding its placeholder.
func (m *Manager) discardEmpty(target Target) bool {
	opened, err := m.open(target)
	if err != nil {
		return false
	}
	ciphertext, err := opened.LoadCiphertext()
	clear(ciphertext)
	if !errors.Is(err, ErrNotFound) {
		return false
	}
	if placeholder, ok := opened.(Placeholder); ok {
		if err := placeholder.DiscardEmpty(); err != nil {
			slog.Warn("discard empty vault placeholder", "err", err)
		}
	}
	return true
}

// Relocate writes the vault to target and adopts it with its recorded identity; only then is the previous file
// removed, so a write or check that fails leaves the previous file current and untouched.
func (m *Manager) Relocate(target Target, write func(Ciphertext) error) (Relocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bound == nil {
		return Relocation{}, ErrUnavailable
	}
	if target.Same(m.list.current) {
		return Relocation{}, ErrSameLocation
	}
	if slices.ContainsFunc(m.list.vaults, target.Same) {
		return Relocation{}, ErrTargetOccupied
	}
	if write == nil {
		return Relocation{}, errors.New("relocation writer is required")
	}

	opened, err := m.open(target)
	if err != nil {
		return Relocation{}, err
	}
	switch _, err := opened.LoadCiphertext(); {
	case err == nil:
		return Relocation{}, ErrTargetOccupied
	case !errors.Is(err, ErrNotFound):
		return Relocation{}, fmt.Errorf("%w: %w", ErrTargetOccupied, err)
	}
	if err := write(opened); err != nil {
		removeQuietly(opened)
		return Relocation{}, err
	}

	target.Vault = m.list.current.Vault
	next := m.list
	next.vaults = slices.Clone(next.vaults)
	if at := slices.IndexFunc(next.vaults, next.current.Same); at >= 0 {
		next.vaults[at] = target
	} else {
		next.vaults = append(next.vaults, target)
	}
	next.current = target
	if err := m.record.save(next); err != nil {
		removeQuietly(opened)
		return Relocation{}, err
	}

	previous, previousTarget := m.bound, m.list.current
	m.list = next
	m.adopt(opened)
	removeErr := previous.Remove()
	if removeErr != nil {
		slog.Warn("remove the previous vault file", "err", removeErr)
	}
	return Relocation{Target: target, Previous: previousTarget, PreviousRemoved: removeErr == nil}, nil
}

// Identify records vault as the one the bound location holds, writing the selection only when it changes.
func (m *Manager) Identify(vault string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bound == nil {
		return ErrUnavailable
	}
	if m.list.current.Vault == vault {
		return nil
	}
	next := m.list
	next.vaults = slices.Clone(next.vaults)
	next.current.Vault = vault
	if at := slices.IndexFunc(next.vaults, next.current.Same); at >= 0 {
		next.vaults[at].Vault = vault
	}
	if err := m.record.save(next); err != nil {
		return err
	}
	m.list = next
	return nil
}

// Forget drops a vault from the known list without touching its file; the first remaining one becomes current.
func (m *Manager) Forget(target Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.forget(target)
}

// Erase removes a vault's file and forgets it; the caller deletes its device records.
func (m *Manager) Erase(target Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	opened, err := m.open(target)
	if err != nil {
		return err
	}
	if err := opened.Remove(); err != nil {
		return err
	}
	return m.forget(target)
}

// LoadFrom reads a vault's ciphertext without binding it.
func (m *Manager) LoadFrom(target Target) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if target.Same(m.list.current) && m.bound != nil {
		return m.bound.LoadCiphertext()
	}
	opened, err := m.open(target)
	if err != nil {
		return nil, err
	}
	return opened.LoadCiphertext()
}

// Status reports the offered kinds, the known vaults, the current location and why none is bound.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := Status{
		Kinds:   make([]Kind, len(m.backends)),
		Current: m.list.current,
		Default: m.defaultTarget,
		Vaults:  slices.Clone(m.list.vaults),
		Reason:  m.reason,
	}
	for i, backend := range m.backends {
		status.Kinds[i] = backend.Kind()
	}
	if m.bound != nil {
		status.Available = true
		status.Restricted = m.bound.Restricted()
	}
	return status
}

// LoadCiphertext reads the bound vault, or fails with ErrUnavailable.
func (m *Manager) LoadCiphertext() ([]byte, error) {
	bound, err := m.current()
	if err != nil {
		return nil, err
	}
	return bound.LoadCiphertext()
}

// CommitCiphertext commits candidate to the bound vault, or fails with ErrUnavailable.
func (m *Manager) CommitCiphertext(expectedHash *[sha256.Size]byte, candidate []byte, finalize func() error) error {
	bound, err := m.current()
	if err != nil {
		return err
	}
	return bound.CommitCiphertext(expectedHash, candidate, finalize)
}

// ReconcileCiphertext reconciles the bound vault, or fails with ErrUnavailable.
func (m *Manager) ReconcileCiphertext(expectedHash [sha256.Size]byte, finalize func() error) error {
	bound, err := m.current()
	if err != nil {
		return err
	}
	return bound.ReconcileCiphertext(expectedHash, finalize)
}

func (m *Manager) forget(target Target) error {
	at := slices.IndexFunc(m.list.vaults, target.Same)
	if at < 0 {
		return ErrUnknownVault
	}
	wasCurrent := target.Same(m.list.current)
	next := known{vaults: slices.Delete(slices.Clone(m.list.vaults), at, at+1)}
	if len(next.vaults) == 0 {
		next.vaults = []Target{m.defaultTarget}
	}
	next.current = m.list.current
	if wasCurrent {
		next.current = next.vaults[0]
	}
	if err := m.record.save(next); err != nil {
		return err
	}
	m.list = next
	if wasCurrent {
		m.release(ReasonNone)
		// An unreachable location leaves the manager unbound, which Status reports.
		if err := m.bindCurrent(); err != nil {
			slog.Warn("bind the next vault location", "err", err)
		}
	}
	return nil
}

func (m *Manager) bindCurrent() error {
	opened, err := m.open(m.list.current)
	if err != nil {
		m.release(ReasonUnreachable)
		return err
	}
	m.adopt(opened)
	return nil
}

func (m *Manager) current() (Store, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bound == nil {
		return nil, ErrUnavailable
	}
	return m.bound, nil
}

func (m *Manager) backend(kind Kind) Backend {
	for _, backend := range m.backends {
		if backend.Kind() == kind {
			return backend
		}
	}
	return nil
}

// check reports whether target names a location of an offered kind.
func (m *Manager) check(target Target) error {
	backend := m.backend(target.Kind)
	if backend == nil {
		return ErrUnsupportedKind
	}
	return backend.Check(target.Path)
}

func (m *Manager) open(target Target) (Store, error) {
	if err := m.check(target); err != nil {
		return nil, err
	}
	return m.backend(target.Kind).Open(target, m.maxBytes)
}

func (m *Manager) adopt(opened Store) {
	m.bound = opened
	m.reason = ReasonNone
}

func (m *Manager) release(reason Reason) {
	m.bound = nil
	m.reason = reason
}

func removeQuietly(opened Store) {
	if err := opened.Remove(); err != nil {
		slog.Warn("remove unadopted vault copy", "err", err)
	}
}
