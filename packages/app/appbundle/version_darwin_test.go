//go:build darwin

package appbundle

import (
	"regexp"
	"testing"
)

func TestTheSystemIsMacOSWithItsProductVersion(t *testing.T) {
	name, version := Running{}.System()
	if name != "macOS" || !regexp.MustCompile(`^\d+(\.\d+)*$`).MatchString(version) {
		t.Fatalf("system = %q %q", name, version)
	}
}
