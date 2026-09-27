package confirmationpanel

import (
	"fmt"
	"runtime"
	"slices"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type fakeWindow struct {
	calls []string
}

func (f *fakeWindow) SetSize(width, height int) {
	f.calls = append(f.calls, fmt.Sprintf("size %dx%d", width, height))
}

func (f *fakeWindow) CenterOn(screen *application.Screen) {
	if screen == nil {
		f.calls = append(f.calls, "center")
		return
	}
	f.calls = append(f.calls, "center on "+screen.ID)
}

func (f *fakeWindow) Show()  { f.calls = append(f.calls, "show") }
func (f *fakeWindow) Focus() { f.calls = append(f.calls, "focus") }
func (f *fakeWindow) Hide()  { f.calls = append(f.calls, "hide") }

type fakeDisplay struct {
	window *fakeWindow
	opened int
	screen *application.Screen
}

func (f *fakeDisplay) Open() Window {
	f.opened++
	return f.window
}

func (f *fakeDisplay) ScreenUnderPointer() *application.Screen { return f.screen }

func newPanel(t *testing.T) (*Panel, *fakeDisplay) {
	t.Helper()
	display := &fakeDisplay{window: &fakeWindow{}, screen: &application.Screen{ID: "left"}}
	panel, err := New(display)
	if err != nil {
		t.Fatal(err)
	}
	return panel, display
}

func TestNewRequiresADisplay(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("a panel without a display was accepted")
	}
}

func TestThePanelShowsOnTheFirstRequestAndHidesWhenNoneWaits(t *testing.T) {
	panel, display := newPanel(t)
	panel.Follow(false)
	if display.opened != 0 {
		t.Fatal("an empty queue built the window")
	}
	panel.Follow(true)
	panel.Follow(false)
	display.screen = &application.Screen{ID: "right"}
	panel.Follow(true)
	if display.opened != 1 {
		t.Fatalf("the window was built %d times", display.opened)
	}
	want := []string{
		"size 360x240", "center on left", "show", "focus",
		"hide",
		"size 360x240", "center on right", "show", "focus",
	}
	if !slices.Equal(display.window.calls, want) {
		t.Fatalf("window calls = %q, want %q", display.window.calls, want)
	}
}

func TestThePageSetsThePanelsHeight(t *testing.T) {
	panel, display := newPanel(t)
	panel.Fit(300)
	panel.Follow(true)
	panel.Fit(300)
	panel.Fit(10)
	panel.Fit(5000)
	want := []string{
		"size 360x300", "center on left", "show", "focus",
		fmt.Sprintf("size 360x%d", minHeight), "center on left",
		fmt.Sprintf("size 360x%d", maxHeight), "center on left",
	}
	if !slices.Equal(display.window.calls, want) {
		t.Fatalf("window calls = %q, want %q", display.window.calls, want)
	}
}

func TestThePanelShowsOnTheScreenUnderThePointer(t *testing.T) {
	left := &application.Screen{ID: "left", IsPrimary: true, Bounds: application.Rect{X: 0, Y: 0, Width: 1440, Height: 900}}
	above := &application.Screen{ID: "above", Bounds: application.Rect{X: 0, Y: -1080, Width: 1920, Height: 1080}}
	right := &application.Screen{ID: "right", Bounds: application.Rect{X: 1440, Y: 0, Width: 1920, Height: 1080}}
	screens := []*application.Screen{left, above, right}
	for name, test := range map[string]struct {
		point application.Point
		known bool
		want  *application.Screen
	}{
		"on the primary screen":  {application.Point{X: 700, Y: 400}, true, left},
		"above the primary":      {application.Point{X: 10, Y: -5}, true, above},
		"on the right":           {application.Point{X: 1440, Y: 1079}, true, right},
		"in a gap between":       {application.Point{X: 2000, Y: -10}, true, left},
		"where it is not known":  {application.Point{X: 1500, Y: 10}, false, left},
		"past the right edge":    {application.Point{X: 3360, Y: 0}, true, left},
		"on the primary's first": {application.Point{X: 0, Y: 0}, true, left},
	} {
		if got := screenAt(screens, test.point, test.known); got != test.want {
			t.Fatalf("%s: screen %v, want %v", name, got.ID, test.want.ID)
		}
	}
	if got := screenAt(nil, application.Point{}, true); got != nil {
		t.Fatalf("no screens answered %v", got)
	}
	if got := screenAt([]*application.Screen{right}, application.Point{}, false); got != right {
		t.Fatalf("with no primary screen the panel shows on %v", got)
	}
}

func TestThePanelsOptions(t *testing.T) {
	options := panelOptions()
	if options.URL != pageURL || options.Width != width || options.Height != initialHeight || !options.Frameless ||
		!options.Hidden || !options.DisableResize || options.BackgroundType != application.BackgroundTypeTransparent {
		t.Fatalf("panel options = %+v", options)
	}
	if runtime.GOOS != "darwin" {
		if !options.AlwaysOnTop {
			t.Fatal("the panel does not stay above other windows")
		}
		return
	}
	mac := options.Mac
	if mac.WindowClass != application.MacWindowClassPanel ||
		mac.PanelPreferences != (application.MacPanelPreferences{NonActivating: true, FloatingPanel: true}) ||
		mac.WindowLevel != application.MacWindowLevelFloating ||
		mac.CollectionBehavior != application.MacWindowCollectionBehaviorCanJoinAllSpaces|
			application.MacWindowCollectionBehaviorFullScreenAuxiliary|
			application.MacWindowCollectionBehaviorIgnoresCycle ||
		mac.DisableShadow || mac.CornerType != application.MacWindowCornerTypeRounded || mac.CornerRadius != cornerRadius {
		t.Fatalf("Mac panel options = %+v", mac)
	}
}
