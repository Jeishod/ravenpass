package appbundle

import (
	"errors"
	"io/fs"
	"testing"
)

func TestTheNoticesAreInTheBundlesResources(t *testing.T) {
	got := noticesPath("/Applications/Ravenpass.app/Contents/MacOS/Ravenpass")
	if want := "/Applications/Ravenpass.app/Contents/Resources/THIRD_PARTY_NOTICES.txt"; got != want {
		t.Fatalf("notices path = %q, want %q", got, want)
	}
}

func TestAProcessOutsideABundleHasNoNotices(t *testing.T) {
	if _, err := (Running{}).Notices(); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("notices outside a bundle: %v, want fs.ErrNotExist", err)
	}
}
