//go:build android

package bridge

import (
	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/pasteboard"
)

var _ api.Pasteboard = Clipboard{}

// Clipboard is the device's clipboard; copies are marked sensitive and each clip's timestamp is its change count.
type Clipboard struct{}

// WriteText replaces the clip with text and reports the new clip's timestamp.
func (Clipboard) WriteText(text string) (int64, error) {
	return stamped(writeText(text))
}

// Write replaces the clip with content served from memory until the clip is cleared or replaced.
func (Clipboard) Write(content []byte, mediaType string) (int64, error) {
	if len(content) == 0 || mediaType == "" {
		return 0, pasteboard.ErrUnsupportedType
	}
	return stamped(writeScan(content, mediaType))
}

// ChangeCount is the current clip's timestamp, or the app's last copy's while Android hides the clipboard.
func (Clipboard) ChangeCount() int64 { return changeCount() }

// Clear stops serving a copied scan and empties the clipboard; it does nothing while the Java bridge is unreachable.
func (Clipboard) Clear() { clearClipboard() }

func stamped(stamp int64) (int64, error) {
	if stamp < 0 {
		return 0, pasteboard.ErrUnavailable
	}
	return stamp, nil
}
