package documents

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/backups"
	"github.com/dortanes/ravenpass/packages/app/storage"
)

// vaultMediaType is the type of a backup document: a vault file has no MIME type of its own.
const vaultMediaType = "application/octet-stream"

var (
	_ api.BackupFolders   = BackupFolders{}
	_ backups.Destination = BackupFolders{}
)

// BackupFolders writes each backup in a new document of a folder chosen with the system folder picker.
type BackupFolders struct {
	Provider Provider
}

// Choose shows the system folder picker, which has no place for the prompt.
func (folders BackupFolders) Choose(string) (string, storage.Label, bool, error) {
	return folders.Provider.PickFolder()
}

// Save writes data to a new document in folder and returns its address; the provider may rename it.
func (folders BackupFolders) Save(folder, name string, data []byte) (string, error) {
	address, err := folders.Provider.CreateIn(folder, name, vaultMediaType)
	if err != nil {
		return "", fmt.Errorf("create backup document: %w", err)
	}
	backup := savedDocument{provider: folders.Provider, address: address}
	if err := backup.Write(data); err != nil {
		backup.Discard()
		return "", err
	}
	return address, nil
}

// Remove deletes the backup document; one its folder no longer holds fails with fs.ErrNotExist, as backups count
// removed.
func (folders BackupFolders) Remove(address string) error {
	err := folders.Provider.Delete(address)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return fmt.Errorf("remove backup document: %w", fs.ErrNotExist)
	case err != nil:
		return fmt.Errorf("remove backup document: %w", err)
	}
	return nil
}
