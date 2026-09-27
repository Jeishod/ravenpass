//go:build !darwin

package confirmationpanel

import "github.com/wailsapp/wails/v3/pkg/application"

func panelOptions() application.WebviewWindowOptions {
	options := baseOptions()
	options.AlwaysOnTop = true
	return options
}
