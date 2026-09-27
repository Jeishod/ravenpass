package importers

// Labels name, in the person's language from the interface catalog, the values an import writes into an item's notes.
type Labels struct {
	Website, OneTimeCode, Title, Company, Username, Expiry, SecurityCode, CardNumber,
	CardholderName, Brand, PrivateKey, PublicKey, Fingerprint, BankName, AccountHolder,
	AccountType, AccountNumber, RoutingNumber, BranchNumber, PIN, SWIFT, IBAN, BankPhone,
	LicenseClass, Sex, BirthPlace, Nationality, PassportType, NationalID, IssuedOn,
	ExpiresOn, Birthday, Issuer, Name, Gender, Nickname string
}
