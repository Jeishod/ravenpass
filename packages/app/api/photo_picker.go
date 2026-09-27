package api

import (
	"path/filepath"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// PickedPhoto is a photo the owner picked: its file name, without a folder, and its content.
type PickedPhoto struct {
	Name string
	Data []byte
}

// PhotoPicker is how the owner picks a photo on a host.
type PhotoPicker interface {
	// Pick asks for one photo, false on cancel; one over limit bytes fails with photos.ErrTooLarge.
	Pick(limit int64) (PickedPhoto, bool, error)
}

// dialogPhotos picks a photo with the file dialog.
type dialogPhotos struct {
	dialog  fileDialog
	dialogs func() preferences.Dialogs
}

func (picker dialogPhotos) Pick(limit int64) (PickedPhoto, bool, error) {
	dialogs := picker.dialogs()
	picked, err := picker.dialog.pick(dialogs.ChoosePhoto, dialogs.PhotoFilter, photoExtensions)
	if err != nil || picked.path == "" {
		return PickedPhoto{}, false, err
	}
	defer picked.discard()
	data, err := readChosenFile(picked.path, limit)
	if err != nil {
		return PickedPhoto{}, false, err
	}
	return PickedPhoto{Name: filepath.Base(picked.path), Data: data}, true, nil
}
