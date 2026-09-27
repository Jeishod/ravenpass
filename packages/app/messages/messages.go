// Package messages serves the interface catalog's wording for the strings the Go side shows itself.
package messages

import (
	_ "embed"
	"encoding/json"
	"slices"
	"sync"

	"golang.org/x/text/language"
)

// Generated from names.json by packages/ui/scripts/export-messages.ts.
//
//go:embed catalog.generated.json
var source []byte

const fallback = "en"

type table struct {
	texts     map[string]map[string]string
	languages []string
	matcher   language.Matcher
}

var loaded = sync.OnceValue(func() table {
	var texts map[string]map[string]string
	if err := json.Unmarshal(source, &texts); err != nil {
		panic("messages: the embedded catalog is malformed: " + err.Error())
	}
	languages := []string{fallback}
	for _, byLanguage := range texts {
		for tag := range byLanguage {
			if !slices.Contains(languages, tag) {
				languages = append(languages, tag)
			}
		}
	}
	slices.Sort(languages[1:])
	tags := make([]language.Tag, len(languages))
	for i, tag := range languages {
		tags[i] = language.Make(tag)
	}
	return table{texts: texts, languages: languages, matcher: language.NewMatcher(tags)}
})

// Catalog is the wording of one language, with English for any message it lacks.
type Catalog struct {
	language string
}

// For returns the catalog that best matches a BCP 47 tag, English when none does.
func For(tag string) Catalog {
	catalog := loaded()
	_, index, _ := catalog.matcher.Match(language.Make(tag))
	return Catalog{language: catalog.languages[index]}
}

// Language is the catalog's language tag.
func (c Catalog) Language() string {
	if c.language == "" {
		return fallback
	}
	return c.language
}

// Text returns the message key names, empty for a key the catalog lacks.
func (c Catalog) Text(key string) string {
	texts := loaded().texts[key]
	if text := texts[c.Language()]; text != "" {
		return text
	}
	return texts[fallback]
}
