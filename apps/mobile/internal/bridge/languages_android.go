//go:build android

package bridge

import "strings"

// PreferredLanguages reports the device's BCP 47 language tags, most preferred first, none before attach.
func PreferredLanguages() []string {
	tags := string(preferredLanguages())
	if tags == "" {
		return nil
	}
	return strings.Split(tags, ",")
}

// SetLanguage makes Android's own text in the app follow tag, a BCP 47 language tag; an empty tag follows the device.
func SetLanguage(tag string) { setLanguage(tag) }
