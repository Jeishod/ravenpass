// Package appfiles names the app's own files directory, which holds the vaults not picked with the file picker.
package appfiles

import (
	"path/filepath"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

// Places names the app's files directory, private to the app and kept out of backups, as this device.
type Places struct {
	Directory string
	Dialogs   func() preferences.Dialogs
}

// Place names folder when it is the app's files directory, false for any other folder.
func (places Places) Place(folder string) (string, bool) {
	if folder != filepath.Clean(places.Directory) {
		return "", false
	}
	return places.Dialogs().OnThisDevice, true
}
