package backups

import (
	"os"
	"path/filepath"

	"github.com/dortanes/ravenpass/packages/app/savedfile"
	"github.com/dortanes/ravenpass/packages/vault"
)

// Directory is the Destination whose folders and addresses are file system paths.
type Directory struct{}

// Save writes data to a new file named name in folder and returns its path.
func (Directory) Save(folder, name string, data []byte) (string, error) {
	path := filepath.Join(folder, name)
	if err := savedfile.Create(path, data, vault.MaxContainerBytes); err != nil {
		return "", err
	}
	return path, nil
}

// Remove deletes the file at address.
func (Directory) Remove(address string) error { return os.Remove(address) }
