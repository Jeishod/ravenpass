package api

import (
	"path/filepath"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

// VaultPlaces is how a host names folders it knows local vault files in, such as a synced cloud drive.
type VaultPlaces interface {
	// Place is folder in the owner's words, false for a folder the host does not name.
	Place(folder string) (string, bool)
}

// noPlaces names no folder.
type noPlaces struct{}

func (noPlaces) Place(string) (string, bool) { return "", false }

// label is target's provider label, else its file name and its folder's named place or base name.
func (s *Service) label(target storage.Target) storage.Label {
	if target.Label != (storage.Label{}) {
		return target.Label
	}
	folder := filepath.Dir(target.Path)
	place, named := s.places.Place(folder)
	if !named {
		place = filepath.Base(folder)
	}
	return storage.Label{Name: filepath.Base(target.Path), Place: place}
}
