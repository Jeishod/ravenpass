// Package fieldwords holds the words that name sign-in, code and search fields, shared with the browser extension.
package fieldwords

import (
	"bytes"
	_ "embed"
	"encoding/json"
)

//go:embed words.json
var source []byte

// Terms are lowercase word prefixes, the Russian ones stems; a term of several words matches them in a row, the last as a prefix.
type Terms struct {
	Search      []string `json:"search"`
	NewPassword []string `json:"newPassword"`
	Login       []string `json:"login"`
	OneTime     []string `json:"oneTime"`
	// Code also names promo, postal and card codes.
	Code []string `json:"code"`
}

// Load returns a fresh copy of the embedded terms.
func Load() Terms {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	var terms Terms
	if err := decoder.Decode(&terms); err != nil {
		panic("fieldwords: the embedded words are malformed: " + err.Error())
	}
	return terms
}
