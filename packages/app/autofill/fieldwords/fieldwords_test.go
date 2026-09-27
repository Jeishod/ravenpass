package fieldwords

import (
	"slices"
	"strings"
	"testing"
)

func TestLoadReadsEveryListAsDistinctLowercaseTerms(t *testing.T) {
	terms := Load()
	for name, list := range map[string][]string{
		"search": terms.Search, "newPassword": terms.NewPassword, "login": terms.Login,
		"oneTime": terms.OneTime, "code": terms.Code,
	} {
		if len(list) == 0 {
			t.Errorf("%s holds no term", name)
		}
		for index, term := range list {
			if term != strings.ToLower(strings.TrimSpace(term)) || term == "" {
				t.Errorf("%s term %q is not trimmed lowercase", name, term)
			}
			if slices.Contains(list[:index], term) {
				t.Errorf("%s repeats %q", name, term)
			}
		}
	}
}

func TestLoadReturnsACopyCallersMayChange(t *testing.T) {
	first := Load()
	first.Login[0] = "changed"
	if Load().Login[0] == "changed" {
		t.Fatal("a change to one load reached the next")
	}
}
