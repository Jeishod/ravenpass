package api

import "testing"

func TestDockIconSetting(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	current, err := service.GetDockIcon()
	if err != nil {
		t.Fatal(err)
	}
	if current.HideWithWindow {
		t.Fatalf("default Dock icon = %+v", current)
	}
	if err := service.SetDockIcon(true); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetDockIcon(); !current.HideWithWindow {
		t.Fatalf("Dock icon after choosing to hide it = %+v", current)
	}
}
