package genhistory

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kinds a generator makes.
const (
	KindPassword   = "password"
	KindPassphrase = "passphrase"
)

// Limits of the generator's options.
const (
	MinLength         = 5
	MaxLength         = 128
	MaxMinimum        = 9
	MinWords          = 3
	MaxWords          = 20
	maxSeparatorRunes = 1
)

const (
	uppercase          = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowercase          = "abcdefghijklmnopqrstuvwxyz"
	numbers            = "0123456789"
	symbols            = "!@#$%^&*"
	unambiguousUpper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	unambiguousLower   = "abcdefghijkmnopqrstuvwxyz"
	unambiguousNumbers = "23456789"
)

// ErrInvalidOptions reports options the generator cannot satisfy.
var ErrInvalidOptions = errors.New("generator options are invalid")

// Options are what the generator makes; Kind picks which of the other fields apply.
type Options struct {
	Kind string `json:"kind"`
	// Length, the character sets, the minimums and AvoidAmbiguous shape a password.
	Length         int  `json:"length"`
	Uppercase      bool `json:"uppercase"`
	Lowercase      bool `json:"lowercase"`
	Numbers        bool `json:"numbers"`
	Symbols        bool `json:"symbols"`
	MinNumbers     int  `json:"minNumbers"`
	MinSymbols     int  `json:"minSymbols"`
	AvoidAmbiguous bool `json:"avoidAmbiguous"`
	// Words, Separator, Capitalize and IncludeNumber shape a passphrase.
	Words         int    `json:"words"`
	Separator     string `json:"separator"`
	Capitalize    bool   `json:"capitalize"`
	IncludeNumber bool   `json:"includeNumber"`
}

// DefaultOptions are the options before the owner changes any.
func DefaultOptions() Options {
	return Options{
		Kind: KindPassword, Length: 14, Uppercase: true, Lowercase: true, Numbers: true, MinNumbers: 1,
		Words: 6, Separator: "-",
	}
}

// Validate reports whether the generator can make what options describe.
func (o Options) Validate() error {
	switch o.Kind {
	case KindPassword:
		if o.Length < MinLength || o.Length > MaxLength || o.MinNumbers < 0 || o.MinNumbers > MaxMinimum ||
			o.MinSymbols < 0 || o.MinSymbols > MaxMinimum {
			return ErrInvalidOptions
		}
		if !o.Uppercase && !o.Lowercase && !o.Numbers && !o.Symbols {
			return ErrInvalidOptions
		}
		if len(o.required()) > o.Length {
			return ErrInvalidOptions
		}
	case KindPassphrase:
		if o.Words < MinWords || o.Words > MaxWords || utf8.RuneCountInString(o.Separator) > maxSeparatorRunes ||
			!utf8.ValidString(o.Separator) || strings.ContainsFunc(o.Separator, unicode.IsControl) {
			return ErrInvalidOptions
		}
	default:
		return ErrInvalidOptions
	}
	return nil
}

func (o Options) sets() (upper, lower, digits string) {
	if o.AvoidAmbiguous {
		return unambiguousUpper, unambiguousLower, unambiguousNumbers
	}
	return uppercase, lowercase, numbers
}

// required lists the character set each guaranteed character comes from.
func (o Options) required() []string {
	upper, lower, digits := o.sets()
	var required []string
	if o.Uppercase {
		required = append(required, upper)
	}
	if o.Lowercase {
		required = append(required, lower)
	}
	if o.Numbers {
		for range max(1, o.MinNumbers) {
			required = append(required, digits)
		}
	}
	if o.Symbols {
		for range max(1, o.MinSymbols) {
			required = append(required, symbols)
		}
	}
	return required
}

// Generate makes a password or passphrase from random; wordlist supplies a passphrase's words.
func Generate(options Options, wordlist []string, random io.Reader) (string, error) {
	if err := options.Validate(); err != nil {
		return "", err
	}
	if options.Kind == KindPassphrase {
		return passphrase(options, wordlist, random)
	}
	return password(options, random)
}

func password(options Options, random io.Reader) (string, error) {
	upper, lower, digits := options.sets()
	var all strings.Builder
	for _, set := range []struct {
		on    bool
		chars string
	}{{options.Uppercase, upper}, {options.Lowercase, lower}, {options.Numbers, digits}, {options.Symbols, symbols}} {
		if set.on {
			all.WriteString(set.chars)
		}
	}
	sets := options.required()
	for len(sets) < options.Length {
		sets = append(sets, all.String())
	}
	result := make([]byte, len(sets))
	for i, set := range sets {
		index, err := uniform(random, len(set))
		if err != nil {
			return "", err
		}
		result[i] = set[index]
	}
	// Shuffle so the guaranteed characters sit anywhere.
	for i := len(result) - 1; i > 0; i-- {
		j, err := uniform(random, i+1)
		if err != nil {
			return "", err
		}
		result[i], result[j] = result[j], result[i]
	}
	return string(result), nil
}

func passphrase(options Options, wordlist []string, random io.Reader) (string, error) {
	if len(wordlist) < 2 {
		return "", ErrInvalidOptions
	}
	words := make([]string, options.Words)
	for i := range words {
		index, err := uniform(random, len(wordlist))
		if err != nil {
			return "", err
		}
		words[i] = wordlist[index]
		if options.Capitalize {
			first, size := utf8.DecodeRuneInString(words[i])
			words[i] = string(unicode.ToUpper(first)) + words[i][size:]
		}
	}
	if options.IncludeNumber {
		which, err := uniform(random, len(words))
		if err != nil {
			return "", err
		}
		digit, err := uniform(random, len(numbers))
		if err != nil {
			return "", err
		}
		words[which] += numbers[digit : digit+1]
	}
	return strings.Join(words, options.Separator), nil
}

// uniform draws an unbiased integer in [0, n).
func uniform(random io.Reader, n int) (int, error) {
	value, err := rand.Int(random, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}
