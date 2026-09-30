package api

import (
	"encoding/hex"
	"slices"

	"github.com/dortanes/ravenpass/packages/vault"
)

// LinkedApp is an Android app a credential signs in to; only autofill links one.
type LinkedApp struct {
	Package string `json:"package"`
	// Signer is the lower-case hex SHA-256 digest of one signing certificate.
	Signer string `json:"signer"`
	// Name is what the app installed on this device shows as its name; empty where the device has no such app.
	Name string `json:"name"`
}

// AppNames names the apps installed on the device.
type AppNames interface {
	// Name is what the app of pkg, an Android package name, shows as its name; empty where the device has none.
	Name(pkg string) string
}

type noAppNames struct{}

func (noAppNames) Name(string) string { return "" }

// linkedApps are a credential's app links as the interface shows them, never null.
func linkedApps(apps []vault.App, names AppNames) []LinkedApp {
	views := make([]LinkedApp, len(apps))
	for i, app := range apps {
		views[i] = LinkedApp{Package: app.Package, Signer: hex.EncodeToString(app.Signer[:]), Name: names.Name(app.Package)}
	}
	return views
}

// keptApps are the links of held that given names, in given order; a link not held fails with invalid-item.
func keptApps(held []vault.App, given []LinkedApp) ([]vault.App, error) {
	kept := make([]vault.App, 0, len(given))
	for _, app := range given {
		index := slices.IndexFunc(held, func(link vault.App) bool {
			return link.Package == app.Package && hex.EncodeToString(link.Signer[:]) == app.Signer
		})
		if index < 0 {
			return nil, fail(failureInvalidItem)
		}
		kept = append(kept, held[index])
	}
	return kept, nil
}

// heldApps are the indexed app links of the credential id, none when id is not a held credential.
func (s *Service) heldApps(id vault.ID) ([]vault.App, error) {
	entries, err := s.vault.List()
	if err != nil {
		return nil, present(err)
	}
	index := slices.IndexFunc(entries, func(entry vault.Entry) bool { return entry.ID == id && entry.Kind == vault.KindCredential })
	if index < 0 {
		return nil, nil
	}
	return entries[index].Apps, nil
}
