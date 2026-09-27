//go:build !darwin || !cgo

package confirmationpanel

import "github.com/wailsapp/wails/v3/pkg/application"

func pointer() (application.Point, bool) {
	return application.Point{}, false
}
