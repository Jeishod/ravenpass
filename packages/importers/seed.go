package importers

import (
	"strings"

	"github.com/dortanes/ravenpass/packages/vault"
)

// PhraseIn returns the words of text when it is nothing but 12 to 24 English BIP-39 words, checksum or not.
func PhraseIn(text string) ([]string, bool) {
	words := strings.Fields(text)
	checksum, _ := vault.CheckSeedPhrase(words)
	if checksum == vault.SeedChecksumUnknown {
		return nil, false
	}
	return words, true
}
