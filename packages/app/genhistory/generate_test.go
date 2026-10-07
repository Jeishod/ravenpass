package genhistory

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"unicode"

	"github.com/dortanes/ravenpass/packages/vault"
)

func count(value, set string) int {
	n := 0
	for _, r := range value {
		if strings.ContainsRune(set, r) {
			n++
		}
	}
	return n
}

func TestAPasswordHasTheLengthAndEveryChosenSet(t *testing.T) {
	options := Options{Kind: KindPassword, Length: 20, Uppercase: true, Lowercase: true, Numbers: true, Symbols: true,
		MinNumbers: 3, MinSymbols: 2}
	for range 200 {
		value, err := Generate(options, nil, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if len(value) != 20 || count(value, uppercase) == 0 || count(value, lowercase) == 0 ||
			count(value, numbers) < 3 || count(value, symbols) < 2 {
			t.Fatalf("value %q misses its options", value)
		}
		if count(value, uppercase+lowercase+numbers+symbols) != len(value) {
			t.Fatalf("value %q holds a character outside its sets", value)
		}
	}
}

func TestAPasswordWithoutASetNeverHoldsIt(t *testing.T) {
	options := Options{Kind: KindPassword, Length: 64, Lowercase: true, Numbers: true}
	value, err := Generate(options, nil, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if count(value, uppercase+symbols) != 0 {
		t.Fatalf("value %q holds an unchosen set", value)
	}
}

func TestAvoidingAmbiguousCharactersLeavesThemOut(t *testing.T) {
	options := Options{Kind: KindPassword, Length: MaxLength, Uppercase: true, Lowercase: true, Numbers: true,
		AvoidAmbiguous: true}
	for range 50 {
		value, err := Generate(options, nil, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(value, "IOl01") {
			t.Fatalf("value %q holds an ambiguous character", value)
		}
	}
}

func TestOptionsTheGeneratorCannotMeetAreRefused(t *testing.T) {
	for name, options := range map[string]Options{
		"too short":          {Kind: KindPassword, Length: MinLength - 1, Lowercase: true},
		"too long":           {Kind: KindPassword, Length: MaxLength + 1, Lowercase: true},
		"no set":             {Kind: KindPassword, Length: 10},
		"minimums too large": {Kind: KindPassword, Length: 5, Numbers: true, Symbols: true, MinNumbers: 3, MinSymbols: 3},
		"minimum past limit": {Kind: KindPassword, Length: 20, Numbers: true, MinNumbers: MaxMinimum + 1},
		"too few words":      {Kind: KindPassphrase, Words: MinWords - 1, Separator: "-"},
		"too many words":     {Kind: KindPassphrase, Words: MaxWords + 1, Separator: "-"},
		"long separator":     {Kind: KindPassphrase, Words: 4, Separator: "--"},
		"control separator":  {Kind: KindPassphrase, Words: 4, Separator: "\n"},
		"unknown kind":       {Kind: "pin", Length: 10, Numbers: true},
	} {
		if _, err := Generate(options, vault.SeedWordlist(), rand.Reader); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("%s: err = %v, want ErrInvalidOptions", name, err)
		}
	}
}

func TestAPassphraseJoinsListedWords(t *testing.T) {
	wordlist := vault.SeedWordlist()
	options := Options{Kind: KindPassphrase, Words: 5, Separator: "_"}
	value, err := Generate(options, wordlist, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Split(value, "_")
	if len(words) != 5 {
		t.Fatalf("value %q has %d words", value, len(words))
	}
	for _, word := range words {
		if !strings.Contains(" "+strings.Join(wordlist, " ")+" ", " "+word+" ") {
			t.Fatalf("word %q is not listed", word)
		}
	}
}

func TestAPassphraseCanBeCapitalizedAndHoldANumber(t *testing.T) {
	options := Options{Kind: KindPassphrase, Words: 4, Separator: " ", Capitalize: true, IncludeNumber: true}
	value, err := Generate(options, vault.SeedWordlist(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range strings.Split(value, " ") {
		if !unicode.IsUpper([]rune(word)[0]) {
			t.Fatalf("word %q is not capitalized", word)
		}
	}
	if count(value, numbers) != 1 {
		t.Fatalf("value %q does not hold one number", value)
	}
}

func TestAPassphraseWithoutASeparatorIsAccepted(t *testing.T) {
	options := Options{Kind: KindPassphrase, Words: 3}
	if _, err := Generate(options, vault.SeedWordlist(), rand.Reader); err != nil {
		t.Fatal(err)
	}
}
