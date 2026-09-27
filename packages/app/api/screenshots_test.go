package api

import (
	"slices"
	"testing"
)

// recordedScreens records each choice the service applies to the host's window.
type recordedScreens struct{ applied []bool }

func (r *recordedScreens) AllowScreenshots(allowed bool) { r.applied = append(r.applied, allowed) }

func TestAHostThatKeepsItsWindowsOutOfScreenshotsOffersTheChoice(t *testing.T) {
	screens := &recordedScreens{}
	service := newServiceOnHost(t, Host{Screenshots: screens})
	if !service.Capabilities().Screenshots {
		t.Fatal("a host that keeps its windows out of screenshots does not offer the choice")
	}
	if !slices.Equal(screens.applied, []bool{false}) {
		t.Fatalf("the window started with %v, want screenshots blocked", screens.applied)
	}
	if newServiceOnHost(t, Host{}).Capabilities().Screenshots {
		t.Fatal("a host without the port offers the choice")
	}
}

func TestAllowingScreenshotsIsRecordedAndAppliedAtOnce(t *testing.T) {
	screens := &recordedScreens{}
	service := newServiceOnHost(t, Host{Screenshots: screens})
	if err := service.AllowScreenshots(true); err != nil {
		t.Fatal(err)
	}
	if allowed, err := service.ScreenshotsAllowed(); err != nil || !allowed {
		t.Fatalf("screenshots allowed = %v, %v", allowed, err)
	}
	if err := service.AllowScreenshots(false); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(screens.applied, []bool{false, true, false}) {
		t.Fatalf("applied choices = %v", screens.applied)
	}
}

func TestAStoredScreenshotChoiceReachesTheWindowAtStart(t *testing.T) {
	settings := newTestPreferences(t)
	if err := settings.SetScreenshotsAllowed(true); err != nil {
		t.Fatal(err)
	}
	screens := &recordedScreens{}
	newServiceOnHostWith(t, Host{Screenshots: screens}, settings)
	if !slices.Equal(screens.applied, []bool{true}) {
		t.Fatalf("the window started with %v, want screenshots allowed", screens.applied)
	}
}
