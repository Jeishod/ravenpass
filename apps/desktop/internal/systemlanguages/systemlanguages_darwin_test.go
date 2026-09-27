//go:build darwin && cgo

package systemlanguages

import "testing"

func TestPreferredNamesTheMacsLanguages(t *testing.T) {
	languages := Preferred()
	if len(languages) == 0 {
		t.Fatal("the Mac reported no preferred language")
	}
	for _, tag := range languages {
		if tag == "" {
			t.Fatalf("an empty tag among %q", languages)
		}
	}
}
