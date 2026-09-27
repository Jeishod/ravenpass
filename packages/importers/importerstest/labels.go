// Package importerstest holds fixtures for tests of the importers.
package importerstest

import "github.com/dortanes/ravenpass/packages/importers"

// English returns labels in the wording of the English interface catalog.
func English() importers.Labels {
	return importers.Labels{
		Website:        "Website",
		OneTimeCode:    "One-time code setup",
		Title:          "Title",
		Company:        "Company",
		Username:       "Username",
		Expiry:         "Expiration date",
		SecurityCode:   "Security code",
		CardNumber:     "Card number",
		CardholderName: "Cardholder",
		Brand:          "Brand",
		PrivateKey:     "Private key",
		PublicKey:      "Public key",
		Fingerprint:    "Fingerprint",
		BankName:       "Bank",
		AccountHolder:  "Account holder",
		AccountType:    "Account type",
		AccountNumber:  "Account number",
		RoutingNumber:  "Routing number",
		BranchNumber:   "Branch number",
		PIN:            "PIN",
		SWIFT:          "SWIFT",
		IBAN:           "IBAN",
		BankPhone:      "Bank phone",
		LicenseClass:   "License class",
		Sex:            "Sex",
		BirthPlace:     "Place of birth",
		Nationality:    "Nationality",
		PassportType:   "Passport type",
		NationalID:     "National ID number",
		IssuedOn:       "Issued on",
		ExpiresOn:      "Expires on",
		Birthday:       "Date of birth",
		Issuer:         "Issued by",
		Name:           "Name",
		Gender:         "Gender",
		Nickname:       "Nickname",
	}
}
