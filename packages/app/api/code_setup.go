package api

import (
	"cmp"
	"strconv"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// codeSetupLifetime is how long a received one-time code setup waits, counted from its arrival.
const codeSetupLifetime = 5 * time.Minute

// CodeSetup is the one-time code setup waiting for the owner, without its secret.
type CodeSetup struct {
	Pending bool `json:"pending"`
	// Token names the setup to the calls acting on it.
	Token   string `json:"token"`
	Issuer  string `json:"issuer"`
	Account string `json:"account"`
}

// heldCodeSetup keeps the last received otpauth link in memory until used, dismissed, replaced or expired; forgetting it cannot wipe its strings.
type heldCodeSetup struct {
	mu   sync.Mutex
	link string
	face vault.SetupFace
	// generation tells the expiry of the link held now from the expiry of one it replaced.
	generation uint64
	expiry     *time.Timer
}

func (h *heldCodeSetup) keep(link string, face vault.SetupFace) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.drop()
	h.generation++
	generation := h.generation
	h.link, h.face = link, face
	h.expiry = time.AfterFunc(codeSetupLifetime, func() { h.expire(generation) })
}

// held reports the face of the held link and the token that names it, if one is held.
func (h *heldCodeSetup) held() (vault.SetupFace, string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.face, h.token(), h.link != ""
}

// apply runs add with the link token names and forgets it on success; a stale token fails with code-setup-expired.
func (h *heldCodeSetup) apply(token string, add func(link string, face vault.SetupFace) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.link == "" || token != h.token() {
		return fail(failureCodeSetupExpired)
	}
	if err := add(h.link, h.face); err != nil {
		return err
	}
	h.drop()
	return nil
}

// forget forgets the link token names, and keeps a newer one.
func (h *heldCodeSetup) forget(token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if token == h.token() {
		h.drop()
	}
}

// token names the link held now. The caller holds h.mu.
func (h *heldCodeSetup) token() string { return strconv.FormatUint(h.generation, 10) }

func (h *heldCodeSetup) expire(generation uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.generation == generation {
		h.drop()
	}
}

// drop forgets the held link. The caller holds h.mu.
func (h *heldCodeSetup) drop() {
	if h.expiry != nil {
		h.expiry.Stop()
	}
	h.link, h.face, h.expiry = "", vault.SetupFace{}, nil
}

// GetCodeSetup reports the one-time code setup waiting for the owner.
func (s *Service) GetCodeSetup() (CodeSetup, error) {
	if err := s.requireOpenVault(); err != nil {
		return CodeSetup{}, err
	}
	face, token, pending := s.codeSetup.held()
	if !pending {
		return CodeSetup{}, nil
	}
	return CodeSetup{Pending: true, Token: token, Issuer: face.Issuer, Account: face.Account}, nil
}

// AddCodeSetup makes the waiting setup token names the one-time code of credential credentialID.
func (s *Service) AddCodeSetup(token, credentialID string) error {
	parsed, err := vault.ParseID(credentialID)
	if err != nil {
		return fail(failureItemUnreadable)
	}
	return s.applyCodeSetup(token, func(link string, _ vault.SetupFace) error {
		return present(s.vault.EditCredential(parsed, vault.CredentialPatch{TOTP: &link}))
	})
}

// CreateCredentialFromCodeSetup creates a credential in the default group from the waiting setup token names.
func (s *Service) CreateCredentialFromCodeSetup(token string) (string, error) {
	var created vault.ID
	err := s.applyCodeSetup(token, func(link string, face vault.SetupFace) error {
		group, held, err := s.vault.DefaultGroup(s.preferences.DefaultGroup())
		if err != nil {
			return present(err)
		}
		var groups []vault.ID
		if held {
			groups = []vault.ID{group}
		}
		input := vault.CredentialInput{Label: cmp.Or(face.Issuer, face.Account), Login: face.Account, TOTP: link}
		created, err = s.vault.CreateCredential(input, groups)
		return present(err)
	})
	if err != nil {
		return "", err
	}
	return created.String(), nil
}

// DismissCodeSetup forgets the waiting setup token names.
func (s *Service) DismissCodeSetup(token string) error {
	if err := s.requireOpenVault(); err != nil {
		return err
	}
	s.codeSetup.forget(token)
	return nil
}

// applyCodeSetup runs add with the waiting setup token names while the vault is open.
func (s *Service) applyCodeSetup(token string, add func(link string, face vault.SetupFace) error) error {
	if err := s.requireOpenVault(); err != nil {
		return err
	}
	return s.codeSetup.apply(token, add)
}

func (s *Service) requireOpenVault() error {
	if !s.vault.Unlocked() {
		return fail(failureVaultLocked)
	}
	return nil
}
