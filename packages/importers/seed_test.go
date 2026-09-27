package importers

import (
	"reflect"
	"strings"
	"testing"
)

func TestPhraseInFindsASeedPhraseAlone(t *testing.T) {
	valid := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	mistyped := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon"
	tests := []struct {
		text  string
		words []string
	}{
		{valid, strings.Fields(valid)},
		{"  " + strings.ReplaceAll(valid, " ", "\n") + "\n", strings.Fields(valid)},
		{strings.ToUpper(valid), strings.Fields(strings.ToUpper(valid))},
		{mistyped, strings.Fields(mistyped)},
		{"Recovery phrase: " + valid, nil},
		{strings.TrimSuffix(valid, " about"), nil},
		{valid + " zebra wrong", nil},
		{"huge merry depth mixed base chapter dragon blame one possible amazing notaword", nil},
		{"", nil},
	}
	for _, test := range tests {
		words, found := PhraseIn(test.text)
		if found != (test.words != nil) || !reflect.DeepEqual(words, test.words) {
			t.Errorf("PhraseIn(%q) = %q, %v", test.text, words, found)
		}
	}
}
