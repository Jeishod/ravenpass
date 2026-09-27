package pasteboard

import (
	"errors"
	"testing"
)

// A successful write would replace the pasteboard of whoever runs the tests.
func TestWriteRefusesAMediaTypeWithoutAPasteboardType(t *testing.T) {
	for _, mediaType := range []string{"image/png", "text/plain", ""} {
		if _, err := (System{}).Write([]byte("content"), mediaType); !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("%q: got %v", mediaType, err)
		}
	}
}
