//go:build android

package bridge

import "github.com/dortanes/ravenpass/packages/app/api"

var _ api.ScreenCapture = Screens{}

// Screens are the app's windows, kept out of screenshots; only the main window can let them in.
type Screens struct{}

// AllowScreenshots applies the owner's choice to the main window now and to each one created later.
func (Screens) AllowScreenshots(allowed bool) { allowScreenshots(allowed) }
