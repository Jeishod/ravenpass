package autofill

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

var _ Finder = (*Service)(nil)

// fakeFinder answers Suggest by origin and Search by query, with separate answers for code requests.
type fakeFinder struct {
	offers     map[string][]Suggestion
	codes      map[string][]Suggestion
	found      map[string][]Suggestion
	foundCodes map[string][]Suggestion
	err        error
}

func (f fakeFinder) Suggest(r Requester, code bool) ([]Suggestion, error) {
	if code {
		return f.codes[r.Origin], f.err
	}
	return f.offers[r.Origin], f.err
}

func (f fakeFinder) Search(query string, code bool) ([]Suggestion, error) {
	if code {
		return f.foundCodes[query], f.err
	}
	return f.found[query], f.err
}

var (
	mail  = Suggestion{ID: "a", Label: "Mail", Account: "alex", Site: "mail.example.com", Exact: true}
	shop  = Suggestion{ID: "b", Label: "Example", Account: "sam", Site: "example.com"}
	bank  = Suggestion{ID: "c", Label: "Bank", Account: "alex", Site: "bank.example"}
	pages = []Requester{{Origin: "https://mail.example.com"}, {Origin: "https://example.com"}}
)

func TestAnEmptyQueryListsWhatMatchesEachRequesterMostSpecificFirstAndEachCredentialOnce(t *testing.T) {
	finder := fakeFinder{offers: map[string][]Suggestion{
		"https://mail.example.com": {mail, shop},
		"https://example.com":      {shop},
	}}
	listing, err := List(finder, pages, " ", false)
	if err != nil {
		t.Fatal(err)
	}
	want := Listing{Scope: ScopeMatches, Results: []Listed{{mail, true}, {shop, true}}}
	if !reflect.DeepEqual(listing, want) {
		t.Fatalf("listing = %+v, want %+v", listing, want)
	}
}

func TestACodeFieldListsTheMatchesWithACodeMarkedByTheirSignInMatch(t *testing.T) {
	finder := fakeFinder{
		offers: map[string][]Suggestion{"https://example.com": {shop}},
		codes:  map[string][]Suggestion{"https://mail.example.com": {mail}},
	}
	listing, err := List(finder, pages, "", true)
	if err != nil {
		t.Fatal(err)
	}
	want := Listing{Scope: ScopeMatches, Results: []Listed{{mail, false}}}
	if !reflect.DeepEqual(listing, want) {
		t.Fatalf("listing = %+v, want %+v", listing, want)
	}
}

func TestWithoutAMatchAnEmptyQueryListsTheVault(t *testing.T) {
	finder := fakeFinder{found: map[string][]Suggestion{"": {bank, shop}}}
	listing, err := List(finder, pages, "", false)
	if err != nil {
		t.Fatal(err)
	}
	want := Listing{Scope: ScopeVault, Results: []Listed{{bank, false}, {shop, false}}}
	if !reflect.DeepEqual(listing, want) {
		t.Fatalf("listing = %+v, want %+v", listing, want)
	}
}

func TestAQuerySearchesTheVaultAndMarksWhatAlreadyMatches(t *testing.T) {
	finder := fakeFinder{
		offers: map[string][]Suggestion{"https://example.com": {shop}},
		found:  map[string][]Suggestion{"a": {bank, shop}},
	}
	listing, err := List(finder, pages, "a", false)
	if err != nil {
		t.Fatal(err)
	}
	want := Listing{Scope: ScopeVault, Results: []Listed{{bank, false}, {shop, true}}}
	if !reflect.DeepEqual(listing, want) {
		t.Fatalf("listing = %+v, want %+v", listing, want)
	}
}

func TestACodeFieldSearchesOnlyCredentialsWithACode(t *testing.T) {
	finder := fakeFinder{
		offers:     map[string][]Suggestion{"https://mail.example.com": {mail}},
		found:      map[string][]Suggestion{"a": {bank, mail}},
		foundCodes: map[string][]Suggestion{"a": {mail}},
	}
	listing, err := List(finder, pages, "a", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Listing{Scope: ScopeVault, Results: []Listed{{mail, true}}}); !reflect.DeepEqual(listing, want) {
		t.Fatalf("listing = %+v, want %+v", listing, want)
	}
}

func TestWithoutARequesterNothingMatches(t *testing.T) {
	finder := fakeFinder{found: map[string][]Suggestion{"": {bank}}}
	listing, err := List(finder, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Listing{Scope: ScopeVault, Results: []Listed{{bank, false}}}); !reflect.DeepEqual(listing, want) {
		t.Fatalf("listing = %+v, want %+v", listing, want)
	}
}

func TestMatchesOfSeveralRequestersStayWithinTheBound(t *testing.T) {
	offers := map[string][]Suggestion{}
	for i := range maxListed {
		for _, requester := range pages {
			offers[requester.Origin] = append(offers[requester.Origin], Suggestion{ID: fmt.Sprintf("%s-%d", requester.Origin, i)})
		}
	}
	listing, err := List(fakeFinder{offers: offers}, pages, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Results) != maxListed || listing.Results[0].ID != "https://mail.example.com-0" {
		t.Fatalf("listed %d, first %+v", len(listing.Results), listing.Results[0])
	}
}

func TestAListingFailsAsTheVaultDoes(t *testing.T) {
	if _, err := List(fakeFinder{err: ErrLocked}, pages, "", false); !errors.Is(err, ErrLocked) {
		t.Fatalf("got %v, want ErrLocked", err)
	}
}
