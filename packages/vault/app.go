package vault

import (
	"regexp"
	"slices"
)

// An app link in a record or the index is [package, signer].
type wireApp struct {
	_       struct{} `cbor:",toarray"`
	Package string
	Signer  [32]byte
}

var packageName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$`)

// App links a credential to an Android package whose signing certificate has SHA-256 digest Signer.
type App struct {
	Package string
	Signer  [32]byte
}

func validApp(app App) bool {
	return len(app.Package) <= MaxPackageLength && packageName.MatchString(app.Package) && app.Signer != [32]byte{}
}

// acceptApps returns a credential's app links as the vault stores them, nil for none.
func acceptApps(apps []App) ([]App, error) {
	if len(apps) > MaxCredentialApps {
		return nil, ErrInvalidInput
	}
	for i, app := range apps {
		if !validApp(app) || slices.Contains(apps[:i], app) {
			return nil, ErrInvalidInput
		}
	}
	return nilIfEmpty(slices.Clone(apps)), nil
}

// LinksApp reports whether one of apps names package pkg with one of signers.
func LinksApp(apps []App, pkg string, signers [][32]byte) bool {
	return slices.ContainsFunc(apps, func(app App) bool {
		return app.Package == pkg && slices.Contains(signers, app.Signer)
	})
}

func wireApps(apps []App) []wireApp {
	wires := make([]wireApp, len(apps))
	for i, app := range apps {
		wires[i] = wireApp{Package: app.Package, Signer: app.Signer}
	}
	return wires
}

// parseApps reads a list of app links, each valid and none twice; nil for none.
func parseApps(wires []wireApp) ([]App, error) {
	if len(wires) > MaxCredentialApps {
		return nil, ErrMalformed
	}
	apps := make([]App, len(wires))
	for i, wire := range wires {
		apps[i] = App{Package: wire.Package, Signer: wire.Signer}
		if !validApp(apps[i]) || slices.Contains(apps[:i], apps[i]) {
			return nil, ErrMalformed
		}
	}
	return nilIfEmpty(apps), nil
}
