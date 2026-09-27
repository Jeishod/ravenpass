package api

import (
	"slices"
	"testing"
)

func TestSignInStyleSetting(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	current, err := service.GetSignInStyle()
	if err != nil {
		t.Fatal(err)
	}
	if current.Style != "card" || !slices.Equal(current.Offered, []string{"card", "field"}) {
		t.Fatalf("default sign-in style = %+v", current)
	}
	if err := service.SetSignInStyle("field"); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetSignInStyle(); current.Style != "field" {
		t.Fatalf("sign-in style after choosing the menu under the field = %+v", current)
	}
	for _, style := range []string{"", "popup", "Card"} {
		err := service.SetSignInStyle(style)
		if err == nil || err.Error() != failurePrefix+string(failureSignInStyleUnsupported) {
			t.Fatalf("style %q: error = %v", style, err)
		}
	}
	if current, _ = service.GetSignInStyle(); current.Style != "field" {
		t.Fatalf("a refused style changed the setting to %+v", current)
	}
}
