package autofill

import (
	"slices"
	"strings"
)

// maxListed bounds a listing as the vault bounds the suggestions for one requester.
const maxListed = 50

// Scope is what a search screen lists: the credentials matching the sign-in, or the vault's.
type Scope string

const (
	ScopeMatches Scope = "matches"
	ScopeVault   Scope = "vault"
)

// Listed is a credential a search screen lists; Matches marks one that already fills for a requester.
type Listed struct {
	Suggestion
	Matches bool
}

// Listing is what a search screen lists.
type Listing struct {
	Scope   Scope
	Results []Listed
}

// Finder is what a listing reads; Service is one.
type Finder interface {
	Suggest(r Requester, code bool) ([]Suggestion, error)
	Search(query string, code bool) ([]Suggestion, error)
}

// List answers a search screen for requesters, most specific first: an empty query lists their matches, if any.
func List(f Finder, requesters []Requester, query string, code bool) (Listing, error) {
	matching, err := suggestAll(f, requesters, false)
	if err != nil {
		return Listing{}, err
	}
	if strings.TrimSpace(query) == "" {
		listed := matching
		if code {
			if listed, err = suggestAll(f, requesters, true); err != nil {
				return Listing{}, err
			}
		}
		if len(listed) > 0 {
			return Listing{Scope: ScopeMatches, Results: marked(listed, matching)}, nil
		}
	}
	found, err := f.Search(query, code)
	if err != nil {
		return Listing{}, err
	}
	return Listing{Scope: ScopeVault, Results: marked(found, matching)}, nil
}

func suggestAll(f Finder, requesters []Requester, code bool) ([]Suggestion, error) {
	var listed []Suggestion
	for _, requester := range requesters {
		found, err := f.Suggest(requester, code)
		if err != nil {
			return nil, err
		}
		for _, offered := range found {
			if len(listed) < maxListed && !slices.ContainsFunc(listed, func(held Suggestion) bool { return held.ID == offered.ID }) {
				listed = append(listed, offered)
			}
		}
	}
	return listed, nil
}

func marked(found, matching []Suggestion) []Listed {
	results := make([]Listed, len(found))
	for i, suggestion := range found {
		results[i] = Listed{Suggestion: suggestion, Matches: slices.ContainsFunc(matching, func(held Suggestion) bool {
			return held.ID == suggestion.ID
		})}
	}
	return results
}
