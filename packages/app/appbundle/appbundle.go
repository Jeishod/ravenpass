// Package appbundle reads what the running macOS app bundle and its system say about the app.
package appbundle

import (
	"os"
	"path/filepath"
)

// noticesFile is where apps/desktop's Makefile copies the third-party notices, in the bundle's Resources.
const noticesFile = "THIRD_PARTY_NOTICES.txt"

// Running is the app bundle the process runs from.
type Running struct{}

// System names macOS and its product version, such as "15.4"; the version is empty where the system does not report it.
func (Running) System() (string, string) { return "macOS", productVersion() }

// Notices reads the bundle's third-party notices; fs.ErrNotExist reports a build outside a bundle or without them.
func (Running) Notices() ([]byte, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(noticesPath(executable))
}

// noticesPath places the notices beside Contents/MacOS, where a bundle keeps its executable.
func noticesPath(executable string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(executable)), "Resources", noticesFile)
}
