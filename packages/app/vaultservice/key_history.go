package vaultservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/dortanes/ravenpass/packages/app/devicerecords"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	keyRecordVersion = 1
	// maxReplacedKeys bounds the replaced vault keys a device remembers per vault.
	maxReplacedKeys = 16
)

// keyHistory is the vault key this device last used for a vault and the keys it saw replaced, newest first. A file
// sealed under a replaced key is what someone holding an old recovery key could write.
type keyHistory struct {
	Version  int      `json:"version"`
	Current  []byte   `json:"current"`
	Replaced [][]byte `json:"replaced,omitempty"`
}

// replaced reports whether identity names a key this device saw replaced.
func (h keyHistory) replaced(identity [32]byte) bool {
	return slices.ContainsFunc(h.Replaced, func(held []byte) bool { return [32]byte(held) == identity })
}

// movedTo is the history once the vault uses identity: the key used before joins the replaced ones.
func (h keyHistory) movedTo(identity [32]byte) keyHistory {
	next := keyHistory{Current: identity[:]}
	if len(h.Current) > 0 && [32]byte(h.Current) != identity {
		next.Replaced = append(next.Replaced, h.Current)
	}
	for _, held := range h.Replaced {
		if [32]byte(held) != identity && len(next.Replaced) < maxReplacedKeys {
			next.Replaced = append(next.Replaced, held)
		}
	}
	return next
}

// loadKeyHistory reads a vault's key record; a vault this device holds no readable record for has an empty history,
// which the next recorded key replaces.
func (s *Service) loadKeyHistory(id vault.ID) (keyHistory, error) {
	data, err := s.keys.LoadKeyRecord(id.String())
	if errors.Is(err, devicerecords.ErrNotFound) {
		return keyHistory{}, nil
	}
	if err != nil {
		return keyHistory{}, err
	}
	var history keyHistory
	if json.Unmarshal(data, &history) != nil || history.Version != keyRecordVersion || len(history.Current) != 32 ||
		slices.ContainsFunc(history.Replaced, func(held []byte) bool { return len(held) != 32 }) {
		slog.Warn("ignore a malformed vault key record")
		return keyHistory{}, nil
	}
	return history, nil
}

// saveKeyHistory stores history as the vault's key record, or removes the record for an empty history.
func (s *Service) saveKeyHistory(id vault.ID, history keyHistory) error {
	if len(history.Current) == 0 {
		return s.keys.DeleteKeyRecord(id.String())
	}
	history.Version = keyRecordVersion
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}
	if err := s.keys.SaveKeyRecord(id.String(), data); err != nil {
		return fmt.Errorf("store vault key record: %w", err)
	}
	return nil
}

// recordKey records that vault id uses the key identity names now.
func (s *Service) recordKey(id vault.ID, identity [32]byte) error {
	history, err := s.loadKeyHistory(id)
	if err != nil {
		return err
	}
	if len(history.Current) > 0 && [32]byte(history.Current) == identity {
		return nil
	}
	return s.saveKeyHistory(id, history.movedTo(identity))
}

// noteSessionKey records the key opened is sealed under, for an open that already succeeded; failing loses only the
// record.
func (s *Service) noteSessionKey(opened *vault.Session) {
	id, identity, err := sessionKey(opened)
	if err == nil {
		err = s.recordKey(id, identity)
	}
	if err != nil {
		slog.Warn("record the vault key", "err", err)
	}
}

// recordRecoveredKey records the key a recovered vault is sealed under. A device that kept no record, such as one
// that last opened the vault with an earlier release, still learns the key its ways in held, held, as replaced.
func (s *Service) recordRecoveredKey(opened *vault.Session, held *[32]byte) error {
	id, identity, err := sessionKey(opened)
	if err != nil {
		return err
	}
	history, err := s.loadKeyHistory(id)
	if err != nil {
		return err
	}
	if len(history.Current) == 0 && held != nil {
		history = history.movedTo(*held)
	}
	return s.saveKeyHistory(id, history.movedTo(identity))
}

// heldKey names the vault key this device's ways in hold for vault id, nil where they hold none it can read.
func (s *Service) heldKey(id vault.ID) *[32]byte {
	policy, err := s.loadPolicy(id)
	if err != nil {
		return nil
	}
	var envelopes [][]byte
	if policy.HasPlatform() {
		envelopes = append(envelopes, policy.Platform.Envelope)
	}
	if policy.HasPIN() {
		envelopes = append(envelopes, policy.PIN.Envelope)
	}
	for _, envelope := range envelopes {
		if held, err := vault.EnvelopeKeyIdentity(envelope, id); err == nil {
			return &held
		}
	}
	return nil
}

// sealedUnderReplacedKey reports whether opened is sealed under a key this device saw replaced.
func (s *Service) sealedUnderReplacedKey(opened *vault.Session) (bool, error) {
	id, identity, err := sessionKey(opened)
	if err != nil {
		return false, err
	}
	history, err := s.loadKeyHistory(id)
	if err != nil {
		return false, err
	}
	return history.replaced(identity), nil
}

// sessionKey is the vault opened shows and the key it is sealed under.
func sessionKey(opened *vault.Session) (vault.ID, [32]byte, error) {
	head, err := opened.Head()
	if err != nil {
		return vault.ID{}, [32]byte{}, err
	}
	identity, err := opened.KeyIdentity()
	if err != nil {
		return vault.ID{}, [32]byte{}, err
	}
	return head.VaultID, identity, nil
}
