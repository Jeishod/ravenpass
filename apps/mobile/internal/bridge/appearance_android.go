//go:build android

package bridge

// SetAppearance gives Android's own screens and the app's windows appearance: "system", "light" or "dark".
func SetAppearance(appearance string) { setAppearance(appearance) }
