// Package dock switches Ravenpass's macOS activation policy at runtime; Wails sets it only at launch.
package dock

// Show puts Ravenpass in the Dock and brings it to the front.
func Show() { show() }

// Hide takes Ravenpass out of the Dock and the app switcher while its menu bar item stays.
func Hide() { hide() }
