// Package appearance gives Ravenpass's windows, menus and dialogs the appearance chosen in settings.
package appearance

import "github.com/dortanes/ravenpass/packages/app/preferences"

// Follow applies the recorded appearance now and after each change; call it before any window opens.
func Follow(store *preferences.Store) {
	store.OnAppearanceChange(func() { apply(store.Appearance()) })
	apply(store.Appearance())
}
