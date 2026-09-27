package preferences

import "golang.org/x/text/language"

// DeviceLanguages reports the owner's preferred BCP 47 tags, such as "ru-RU", most preferred first.
type DeviceLanguages func() []string

// deviceLanguage is the first of preferred whose explicit base language Ravenpass offers, else English.
func deviceLanguage(preferred []string) Language {
	for _, tag := range preferred {
		base, _, _ := language.Make(tag).Raw()
		if offered := Language(base.String()); supported(offered) {
			return offered
		}
	}
	return English
}
