// Package backups writes and prunes encrypted copies of the open vault in the folder the owner chose.
package backups

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	checkEvery = time.Minute
	// retryAfter spaces the keeper's own retries after a failed backup.
	retryAfter = time.Hour
	extension  = ".rpv"
)

// Vault is the open vault as backups read it. *vaultservice.Service satisfies it.
type Vault interface {
	Unlocked() bool
	State() (vaultservice.State, error)
	Export() ([]byte, vault.Head, error)
	RecordExport(head vault.Head, fileDigest [32]byte) error
	Storage() storage.Status
}

// Destination reaches the folder backups go to.
type Destination interface {
	// Save writes data to a new file named name in folder and returns its address; it never replaces a file.
	Save(folder, name string, data []byte) (address string, err error)
	// Remove deletes the file at address; a file already gone fails with fs.ErrNotExist.
	Remove(address string) error
}

// Status is how the open vault's backups stand.
type Status struct {
	// Last is when the latest backup in the chosen folder was written; zero for none.
	Last time.Time
	// Failed reports that the last attempt to back up the open vault failed.
	Failed bool
}

// Keeper backs up the open vault on its own.
type Keeper struct {
	vault       Vault
	settings    func() preferences.AutoBackup
	destination Destination
	now         func() time.Time
	poke        chan struct{}

	mu      sync.Mutex
	journal *journal
	// failedAt holds each vault's last failed attempt until one succeeds.
	failedAt map[string]time.Time
}

// New makes a keeper that journals the backups it writes in the private file at journalPath.
func New(journalPath string, open Vault, settings func() preferences.AutoBackup, destination Destination) (*Keeper, error) {
	if journalPath == "" || open == nil || settings == nil || destination == nil {
		return nil, errors.New("the journal path, vault, settings and destination are required")
	}
	return &Keeper{
		vault: open, settings: settings, destination: destination, now: time.Now,
		poke: make(chan struct{}, 1), journal: openJournal(journalPath), failedAt: map[string]time.Time{},
	}, nil
}

// Run checks every minute and on Poke until ctx ends.
func (k *Keeper) Run(ctx context.Context) {
	ticker := time.NewTicker(checkEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-k.poke:
		}
		k.check(false)
	}
}

// Poke asks Run for a check now, without waiting.
func (k *Keeper) Poke() {
	select {
	case k.poke <- struct{}{}:
	default:
	}
}

// Check checks now and returns after any backup, ignoring the wait after a failure.
func (k *Keeper) Check() { k.check(true) }

// Status reports how the open vault's backups stand; zero while no vault is open.
func (k *Keeper) Status() Status {
	head, open := k.openHead()
	if !open {
		return Status{}
	}
	folder := k.settings().Folder.Address
	id := head.VaultID.String()
	k.mu.Lock()
	defer k.mu.Unlock()
	var status Status
	if latest, found := k.journal.latest(id, folder); found {
		status.Last = time.Unix(latest.At, 0)
	}
	_, status.Failed = k.failedAt[id]
	return status
}

// openHead reports the open vault's head. It reads no vault file while none is open.
func (k *Keeper) openHead() (vault.Head, bool) {
	if !k.vault.Unlocked() {
		return vault.Head{}, false
	}
	state, err := k.vault.State()
	if err != nil || state.Phase != vaultservice.PhaseReady {
		return vault.Head{}, false
	}
	return state.Head, true
}

// check backs up the open vault when it is due. asked skips the wait after a failure.
func (k *Keeper) check(asked bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	settings := k.settings()
	if !settings.Enabled || settings.Folder.Address == "" {
		return
	}
	head, open := k.openHead()
	if !open {
		return
	}
	id := head.VaultID.String()
	at := k.now()
	if latest, found := k.journal.latest(id, settings.Folder.Address); found {
		if latest.Hash == hex.EncodeToString(head.Hash[:]) || at.Before(time.Unix(latest.At, 0).Add(settings.Interval.Every())) {
			return
		}
	}
	if failed, found := k.failedAt[id]; found && !asked && at.Before(failed.Add(retryAfter)) {
		return
	}
	if err := k.backUp(id, settings, at); err != nil {
		slog.Warn("back up vault", "err", err)
		k.failedAt[id] = at
		return
	}
	delete(k.failedAt, id)
}

// backUp writes, journals and records a backup of the open vault, then prunes beyond the chosen count.
func (k *Keeper) backUp(vaultID string, settings preferences.AutoBackup, at time.Time) error {
	data, head, err := k.vault.Export()
	if err != nil {
		return err
	}
	defer clear(data)
	folder := settings.Folder.Address
	name := fileName(k.vault.Storage().Current, at)
	address, err := k.destination.Save(folder, name, data)
	if err != nil {
		return err
	}
	written := entry{
		Folder: folder, Address: address, Name: name,
		Revision: head.Revision, Hash: hex.EncodeToString(head.Hash[:]), At: at.Unix(),
	}
	journaled := k.journal.record(vaultID, append(k.journal.backups(vaultID), written))
	if state, err := k.vault.State(); err == nil && state.Head == head {
		if err := k.vault.RecordExport(head, sha256.Sum256(data)); err != nil {
			slog.Warn("record backup as the latest encrypted copy", "err", err)
		}
	}
	return errors.Join(journaled, k.prune(vaultID, folder, settings.Keep))
}

// prune removes the oldest backups of vaultID in folder beyond keep; other folders' backups leave the journal only.
func (k *Keeper) prune(vaultID, folder string, keep int) error {
	var inFolder []entry
	for _, listed := range k.journal.backups(vaultID) {
		if listed.Folder == folder {
			inFolder = append(inFolder, listed)
		}
	}
	excess := len(inFolder) - keep
	kept := make([]entry, 0, len(inFolder))
	for i, listed := range inFolder {
		if i < excess {
			err := k.destination.Remove(listed.Address)
			if err == nil || errors.Is(err, fs.ErrNotExist) {
				continue
			}
		}
		kept = append(kept, listed)
	}
	return k.journal.record(vaultID, kept)
}

// fileName names a backup as "Personal 2026-09-25 10-30-00.rpv" from the vault's name and local time.
func fileName(target storage.Target, at time.Time) string {
	name := target.Label.Name
	if name == "" {
		name = filepath.Base(target.Path)
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || unicode.IsControl(r) {
			return '-'
		}
		return r
	}, name)
	return strings.TrimSpace(name+" "+at.Local().Format("2006-01-02 15-04-05")) + extension
}
