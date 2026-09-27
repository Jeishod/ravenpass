// Package printing prints in-memory pages through the macOS print panel.
package printing

import (
	"errors"

	"github.com/dortanes/ravenpass/packages/app/api"
)

// ErrUnavailable reports a print panel that cannot be shown now: no main window, another page printing, or a
// page that did not load.
var ErrUnavailable = errors.New("the print panel cannot be shown now")

var _ api.Printer = Mac{}

// Mac prints a page as a sheet on the main window, from an offscreen web view that keeps nothing on disk.
type Mac struct{}
