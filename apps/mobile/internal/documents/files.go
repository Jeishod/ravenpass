package documents

import "github.com/dortanes/ravenpass/packages/app/storage"

// Files picks vault documents with the system file picker.
type Files struct {
	Provider Provider
}

func (Files) Kinds() []storage.Kind { return []storage.Kind{storage.Document} }

// Create asks where a new vault document goes, proposing name.
func (files Files) Create(kind storage.Kind, name string) (storage.Target, bool, error) {
	if kind != storage.Document {
		return storage.Target{}, false, storage.ErrUnsupportedKind
	}
	return files.pick(true, name)
}

// Existing asks for a vault document that already exists.
func (files Files) Existing() (storage.Target, bool, error) {
	return files.pick(false, "")
}

func (files Files) pick(create bool, name string) (storage.Target, bool, error) {
	address, label, picked, err := files.Provider.Pick(create, name)
	if err != nil || !picked {
		return storage.Target{}, false, err
	}
	return storage.Target{Kind: storage.Document, Path: address, Label: label}, true, nil
}
