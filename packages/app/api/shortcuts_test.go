package api

import "testing"

func TestShortcutSettings(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	current, err := service.GetShortcuts()
	if err != nil || current == nil || len(current) != 0 {
		t.Fatalf("shortcuts before any change = %v, %v", current, err)
	}
	if err := service.SetShortcut("palette", "Mod+P"); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetShortcuts(); current["palette"] != "Mod+P" {
		t.Fatalf("shortcuts after a change = %v", current)
	}
	err = service.SetShortcut("Not An Action", "Mod+K")
	if err == nil || err.Error() != failurePrefix+string(failureShortcutInvalid) {
		t.Fatalf("an invalid action: error = %v", err)
	}
	if err := service.SetShortcut("palette", ""); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetShortcuts(); len(current) != 0 {
		t.Fatalf("shortcuts after a reset = %v", current)
	}
}
