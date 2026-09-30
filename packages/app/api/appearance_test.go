package api

import (
	"slices"
	"testing"
)

func TestAppearanceSetting(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	current, err := service.GetAppearance()
	if err != nil {
		t.Fatal(err)
	}
	if current.Appearance != "system" || !slices.Equal(current.Offered, []string{"system", "light", "dark"}) {
		t.Fatalf("default appearance = %+v", current)
	}
	if err := service.SetAppearance("dark"); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetAppearance(); current.Appearance != "dark" {
		t.Fatalf("appearance after choosing dark = %+v", current)
	}
	for _, appearance := range []string{"", "sepia", "Dark"} {
		err := service.SetAppearance(appearance)
		if err == nil || err.Error() != failurePrefix+string(failureAppearanceUnsupported) {
			t.Fatalf("appearance %q: error = %v", appearance, err)
		}
	}
}
