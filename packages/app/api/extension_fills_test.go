package api

import "testing"

func TestExtensionFillConfirmationSetting(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	if confirm, err := service.ConfirmExtensionFills(); err != nil || confirm {
		t.Fatalf("default = %t, error = %v", confirm, err)
	}
	if err := service.SetConfirmExtensionFills(true); err != nil {
		t.Fatal(err)
	}
	if confirm, err := service.ConfirmExtensionFills(); err != nil || !confirm {
		t.Fatalf("after turning it on = %t, error = %v", confirm, err)
	}
}
