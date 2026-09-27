package api

import (
	"context"

	"github.com/dortanes/ravenpass/packages/app/siteicons"
	"github.com/dortanes/ravenpass/packages/vault"
)

// brandSource reads the name and colour a site host declares for itself.
type brandSource interface {
	Brand(ctx context.Context, site string) (siteicons.Brand, error)
}

// BankDetails reports whether a card's bank name and colour are looked up on the bank's site.
type BankDetails struct {
	Enabled bool `json:"enabled"`
}

// BankLookup is what a bank's site declares; either field is empty when unusable.
type BankLookup struct {
	Name string `json:"name"`
	// Color is "#rrggbb".
	Color string `json:"color"`
}

// GetBankDetails reports whether bank details are looked up.
func (s *Service) GetBankDetails() (BankDetails, error) {
	return BankDetails{Enabled: s.preferences.BankDetails()}, nil
}

// SetBankDetails records whether bank details are looked up.
func (s *Service) SetBankDetails(enabled bool) error {
	return present(s.preferences.SetBankDetails(enabled))
}

// LookupBank asks a card's bank site for its name and colour; an unanswered or off lookup is empty, never an error.
func (s *Service) LookupBank(site string) (BankLookup, error) {
	host := vault.SiteOf(site)
	if !s.preferences.BankDetails() || host == "" {
		return BankLookup{}, nil
	}
	brand, err := s.brands.Brand(context.Background(), host)
	if err != nil {
		return BankLookup{}, nil
	}
	lookup := BankLookup{Name: brand.Name, Color: brand.Color}
	if lookup.Color != "" {
		return lookup, nil
	}
	icon, err := s.icons.Icon(host)
	if err != nil {
		return BankLookup{}, present(err)
	}
	lookup.Color = siteicons.Tint(icon)
	return lookup, nil
}
