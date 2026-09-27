package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dortanes/ravenpass/packages/app/privatefile"
)

const (
	maxSelectionBytes = 64 << 10
	maxKnownVaults    = 64
	selectionVersion  = 2
)

// selection lists the known vaults and the current one, unencrypted; it holds no vault data.
type selection struct {
	Version int             `json:"version"`
	Current string          `json:"current"`
	Vaults  []selectedVault `json:"vaults"`
}

type selectedVault struct {
	Kind  Kind   `json:"kind"`
	Path  string `json:"path"`
	Name  string `json:"name,omitempty"`
	Place string `json:"place,omitempty"`
	Vault string `json:"vault,omitempty"`
}

// known is an ordered list of vaults with one of them current.
type known struct {
	vaults  []Target
	current Target
}

// selectionFile reads and writes the selection record at a fixed path.
type selectionFile struct {
	path string
}

// load reads the record, accepting only entries check accepts.
func (file selectionFile) load(check func(Target) error) (known, bool, error) {
	data, err := privatefile.Read(file.path, maxSelectionBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return known{}, false, nil
	}
	if err != nil {
		return known{}, false, fmt.Errorf("%w: %w", ErrSelectionInvalid, err)
	}
	var recorded selection
	if err := json.Unmarshal(data, &recorded); err != nil {
		return known{}, false, fmt.Errorf("%w: %w", ErrSelectionInvalid, err)
	}
	if recorded.Version != selectionVersion {
		return known{}, false, fmt.Errorf("%w: unsupported record", ErrSelectionInvalid)
	}
	return readList(recorded, check)
}

func readList(recorded selection, check func(Target) error) (known, bool, error) {
	if len(recorded.Vaults) == 0 || len(recorded.Vaults) > maxKnownVaults {
		return known{}, false, fmt.Errorf("%w: vault list is empty or too long", ErrSelectionInvalid)
	}
	list := known{vaults: make([]Target, 0, len(recorded.Vaults))}
	seen := make(map[string]bool, len(recorded.Vaults))
	for _, entry := range recorded.Vaults {
		target := Target{Kind: entry.Kind, Path: entry.Path, Label: Label{Name: entry.Name, Place: entry.Place}, Vault: entry.Vault}
		if err := check(target); err != nil {
			return known{}, false, fmt.Errorf("%w: %w", ErrSelectionInvalid, err)
		}
		if seen[target.Path] {
			return known{}, false, fmt.Errorf("%w: vault listed twice", ErrSelectionInvalid)
		}
		seen[target.Path] = true
		list.vaults = append(list.vaults, target)
		if target.Path == recorded.Current {
			list.current = target
		}
	}
	if list.current == (Target{}) {
		return known{}, false, fmt.Errorf("%w: current vault is not listed", ErrSelectionInvalid)
	}
	return list, true, nil
}

func (file selectionFile) save(list known) error {
	record := selection{Version: selectionVersion, Current: list.current.Path}
	record.Vaults = make([]selectedVault, len(list.vaults))
	for i, target := range list.vaults {
		record.Vaults[i] = selectedVault{
			Kind:  target.Kind,
			Path:  target.Path,
			Name:  target.Label.Name,
			Place: target.Label.Place,
			Vault: target.Vault,
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := privatefile.Write(file.path, data); err != nil {
		return fmt.Errorf("write storage selection: %w", err)
	}
	return nil
}
