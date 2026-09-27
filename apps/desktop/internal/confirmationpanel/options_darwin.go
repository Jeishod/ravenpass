//go:build darwin

package confirmationpanel

import "github.com/wailsapp/wails/v3/pkg/application"

// panelOptions round the window itself: WKWebView draws no transparent page without its private drawsBackground key.
func panelOptions() application.WebviewWindowOptions {
	options := baseOptions()
	options.Mac = application.MacWindow{
		Backdrop:     application.MacBackdropTransparent,
		CornerType:   application.MacWindowCornerTypeRounded,
		CornerRadius: cornerRadius,
		WindowClass:  application.MacWindowClassPanel,
		PanelPreferences: application.MacPanelPreferences{
			NonActivating:          true,
			FloatingPanel:          true,
			BecomesKeyOnlyIfNeeded: false,
		},
		WindowLevel: application.MacWindowLevelFloating,
		CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
			application.MacWindowCollectionBehaviorFullScreenAuxiliary |
			application.MacWindowCollectionBehaviorIgnoresCycle,
	}
	return options
}
