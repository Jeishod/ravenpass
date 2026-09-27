package api

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// fileDialog asks the owner for one file to read with the system's open-file dialog.
type fileDialog struct {
	currentApp func() *application.App
	hold       func() (release func())
	// temporary reports a dialog that hands over a copy of the picked file, as Android's does.
	temporary bool
}

// pickedFile is a file the owner picked; an empty path means the dialog was canceled.
type pickedFile struct {
	path      string
	temporary bool
}

// pick shows the dialog titled title for files matching patterns while the host is held.
func (d fileDialog) pick(title, filter, patterns string) (pickedFile, error) {
	app := d.currentApp()
	if app == nil {
		return pickedFile{}, fail(failureWindowUnavailable)
	}
	defer d.hold()()
	path, err := app.Dialog.OpenFile().SetTitle(title).AddFilter(filter, patterns).PromptForSingleSelection()
	if err != nil {
		return pickedFile{}, fail(failureFileNotSelected)
	}
	return pickedFile{path: path, temporary: d.temporary && path != ""}, nil
}

// discard deletes the file where it is the dialog's temporary copy, which holds plaintext.
func (f pickedFile) discard() {
	if !f.temporary {
		return
	}
	if err := os.Remove(f.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("remove the file dialog's copy of a picked file", "err", err)
	}
}
