package api

import (
	"github.com/dortanes/ravenpass/packages/app/storage"
)

func noHold() (release func()) { return func() {} }

// heldFiles holds the host while its vault file picker shows.
type heldFiles struct {
	VaultFiles
	hold func() (release func())
}

func (files heldFiles) Create(kind storage.Kind, name string) (storage.Target, bool, error) {
	defer files.hold()()
	return files.VaultFiles.Create(kind, name)
}

func (files heldFiles) Existing() (storage.Target, bool, error) {
	defer files.hold()()
	return files.VaultFiles.Existing()
}

// heldPhotos holds the host while its photo picker shows.
type heldPhotos struct {
	PhotoPicker
	hold func() (release func())
}

func (photos heldPhotos) Pick(limit int64) (PickedPhoto, bool, error) {
	defer photos.hold()()
	return photos.PhotoPicker.Pick(limit)
}
