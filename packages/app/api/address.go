package api

import (
	"strings"

	"github.com/dortanes/ravenpass/packages/vault"
)

// Address is a postal address of an identity or a card.
type Address struct {
	// ID is an identity address's stable hex id, empty for a new one and for a card's address.
	ID         string `json:"id"`
	Label      string `json:"label"`
	Street     string `json:"street"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

func fromVaultAddress(address vault.Address) Address {
	result := Address{
		Label: address.Label, Street: address.Street, City: address.City,
		Region: address.Region, PostalCode: address.PostalCode, Country: address.Country,
	}
	if address.ID != (vault.ID{}) {
		result.ID = address.ID.String()
	}
	return result
}

func (address Address) toVault() (vault.Address, error) {
	result := vault.Address{
		Label: address.Label, Street: address.Street, City: address.City,
		Region: address.Region, PostalCode: address.PostalCode, Country: address.Country,
	}
	if address.ID == "" {
		return result, nil
	}
	id, err := vault.ParseID(address.ID)
	if err != nil {
		return vault.Address{}, fail(failureInvalidItem)
	}
	result.ID = id
	return result, nil
}

// addressParts reads each copyable part of an address by name.
var addressParts = map[string]func(vault.Address) string{
	"street":     func(address vault.Address) string { return address.Street },
	"city":       func(address vault.Address) string { return address.City },
	"region":     func(address vault.Address) string { return address.Region },
	"postalCode": func(address vault.Address) string { return address.PostalCode },
	"country":    func(address vault.Address) string { return address.Country },
}

// addressPart returns the reader of one named part of an address, trimmed of surrounding space.
func addressPart(name string) (func(vault.Address) string, bool) {
	part, known := addressParts[name]
	if !known {
		return nil, false
	}
	return func(address vault.Address) string { return strings.TrimSpace(part(address)) }, true
}

// formatAddress writes street, postal code and city, region and country as lines, skipping empty parts.
func formatAddress(address vault.Address) string {
	return joinFilled("\n", address.Street, joinFilled(" ", address.PostalCode, address.City), address.Region, address.Country)
}

func joinFilled(separator string, parts ...string) string {
	filled := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			filled = append(filled, trimmed)
		}
	}
	return strings.Join(filled, separator)
}
