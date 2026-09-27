// Package pasteboard writes concealed copies to the system pasteboard and reports each write's change count.
package pasteboard

import "errors"

// Errors a pasteboard write fails with.
var (
	ErrUnsupportedType = errors.New("pasteboard cannot carry this media type")
	ErrUnavailable     = errors.New("pasteboard is unavailable")
)

// types maps a scan's media type to the type identifier the pasteboard carries it under.
var types = map[string]string{
	"image/jpeg":      "public.jpeg",
	"application/pdf": "com.adobe.pdf",
}

// System is the device's general pasteboard.
type System struct{}

// Write replaces the pasteboard with concealed content and reports the change count after the write.
func (System) Write(content []byte, mediaType string) (int64, error) {
	uti, supported := types[mediaType]
	if !supported {
		return 0, ErrUnsupportedType
	}
	return write(content, uti)
}

// WriteText replaces the pasteboard with concealed, transient text and reports the change count after the write.
func (System) WriteText(text string) (int64, error) { return writeText(text) }

// ChangeCount rises every time anything replaces what the pasteboard holds.
func (System) ChangeCount() int64 { return changeCount() }

// Clear empties the pasteboard whichever app wrote it; it does nothing off macOS or without cgo.
func (System) Clear() { clearContents() }
