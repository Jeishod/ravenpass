package api

import (
	"path/filepath"
	"strings"

	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const vaultExtension = ".rpv"

// VaultFiles is how the owner picks vault files on a host; Create and Existing report false on cancel.
type VaultFiles interface {
	// Kinds lists the storage kinds whose file the owner picks; others go where the host keeps them.
	Kinds() []storage.Kind
	// Create asks where a new vault file of kind goes, proposing name.
	Create(kind storage.Kind, name string) (storage.Target, bool, error)
	// Existing asks for a vault file that already exists.
	Existing() (storage.Target, bool, error)
}

// dialogFiles picks local vault files with the desktop's file dialogs.
type dialogFiles struct {
	currentApp func() *application.App
	dialogs    func() preferences.Dialogs
}

func (dialogFiles) Kinds() []storage.Kind { return []storage.Kind{storage.LocalFile} }

func (files dialogFiles) Create(kind storage.Kind, name string) (storage.Target, bool, error) {
	app := files.currentApp()
	if app == nil {
		return storage.Target{}, false, fail(failureWindowUnavailable)
	}
	dialogs := files.dialogs()
	path, err := app.Dialog.SaveFile().
		SetMessage(dialogs.ChooseVaultLocation).
		SetFilename(name).
		AddFilter(dialogs.VaultFilter, "*"+vaultExtension).
		PromptForSingleSelection()
	if err != nil {
		return storage.Target{}, false, fail(failureLocationNotSelected)
	}
	if path == "" {
		return storage.Target{}, false, nil
	}
	return storage.Target{Kind: kind, Path: withVaultExtension(path)}, true, nil
}

func (files dialogFiles) Existing() (storage.Target, bool, error) {
	app := files.currentApp()
	if app == nil {
		return storage.Target{}, false, fail(failureWindowUnavailable)
	}
	dialogs := files.dialogs()
	path, err := app.Dialog.OpenFile().
		SetTitle(dialogs.SelectVaultFile).
		AddFilter(dialogs.VaultFilter, "*"+vaultExtension).
		PromptForSingleSelection()
	if err != nil {
		return storage.Target{}, false, fail(failureFileNotSelected)
	}
	if path == "" {
		return storage.Target{}, false, nil
	}
	return storage.Target{Kind: storage.LocalFile, Path: path}, true, nil
}

// withVaultExtension appends the vault extension when the save dialog returns a name without it.
func withVaultExtension(path string) string {
	if strings.EqualFold(filepath.Ext(path), vaultExtension) {
		return path
	}
	return path + vaultExtension
}
