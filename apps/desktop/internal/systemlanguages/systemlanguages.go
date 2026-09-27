// Package systemlanguages reads the user's preferred system languages.
package systemlanguages

// Preferred returns the user's preferred languages as BCP 47 tags, most preferred first.
func Preferred() []string { return preferred() }
