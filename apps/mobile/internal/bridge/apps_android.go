//go:build android

package bridge

// Apps names the apps installed on the device.
type Apps struct{}

// Name is what the app of pkg, an Android package name, shows as its name; empty where the device has no such app
// or cannot see it.
func (Apps) Name(pkg string) string { return string(appName(pkg)) }
