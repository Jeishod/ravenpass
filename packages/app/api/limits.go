package api

import "github.com/dortanes/ravenpass/packages/vault"

// CredentialLimits are the vault core's credential field lengths in characters and the most websites.
type CredentialLimits struct {
	Label    int `json:"label"`
	Website  int `json:"website"`
	Websites int `json:"websites"`
	Login    int `json:"login"`
	Email    int `json:"email"`
	Password int `json:"password"`
	Notes    int `json:"notes"`
	TOTP     int `json:"totp"`
}

// IdentityLimits are the vault core's identity text lengths in characters and list sizes.
type IdentityLimits struct {
	Label          int `json:"label"`
	FullName       int `json:"fullName"`
	Email          int `json:"email"`
	Phone          int `json:"phone"`
	AddressLabel   int `json:"addressLabel"`
	Street         int `json:"street"`
	City           int `json:"city"`
	Region         int `json:"region"`
	PostalCode     int `json:"postalCode"`
	Country        int `json:"country"`
	DocumentLabel  int `json:"documentLabel"`
	DocumentNumber int `json:"documentNumber"`
	Issuer         int `json:"issuer"`
	Notes          int `json:"notes"`
	Emails         int `json:"emails"`
	Phones         int `json:"phones"`
	Addresses      int `json:"addresses"`
	Documents      int `json:"documents"`
}

// GetIdentityLimits reports the vault core's identity bounds.
func (s *Service) GetIdentityLimits() (IdentityLimits, error) {
	return IdentityLimits{
		Label:          vault.MaxLabelLength,
		FullName:       vault.MaxFullNameLength,
		Email:          vault.MaxEmailLength,
		Phone:          vault.MaxPhoneLength,
		AddressLabel:   vault.MaxPartLabelLength,
		Street:         vault.MaxStreetLength,
		City:           vault.MaxCityLength,
		Region:         vault.MaxRegionLength,
		PostalCode:     vault.MaxPostalCodeLength,
		Country:        vault.MaxCountryLength,
		DocumentLabel:  vault.MaxPartLabelLength,
		DocumentNumber: vault.MaxDocumentNumberLength,
		Issuer:         vault.MaxIssuerLength,
		Notes:          vault.MaxNotesLength,
		Emails:         vault.MaxIdentityEmails,
		Phones:         vault.MaxIdentityPhones,
		Addresses:      vault.MaxIdentityAddresses,
		Documents:      vault.MaxIdentityDocuments,
	}, nil
}

// CardLimits are the vault core's card text lengths in characters and digit counts for number, code and PIN.
type CardLimits struct {
	Label           int `json:"label"`
	Holder          int `json:"holder"`
	NumberMin       int `json:"numberMin"`
	NumberMax       int `json:"numberMax"`
	SecurityCodeMin int `json:"securityCodeMin"`
	SecurityCodeMax int `json:"securityCodeMax"`
	PINMin          int `json:"pinMin"`
	PINMax          int `json:"pinMax"`
	BankName        int `json:"bankName"`
	BankSite        int `json:"bankSite"`
	Street          int `json:"street"`
	City            int `json:"city"`
	Region          int `json:"region"`
	PostalCode      int `json:"postalCode"`
	Country         int `json:"country"`
	Notes           int `json:"notes"`
}

// GetCardLimits reports the vault core's card bounds.
func (s *Service) GetCardLimits() (CardLimits, error) {
	return CardLimits{
		Label:           vault.MaxLabelLength,
		Holder:          vault.MaxCardHolderLength,
		NumberMin:       vault.MinCardNumberLength,
		NumberMax:       vault.MaxCardNumberLength,
		SecurityCodeMin: vault.MinSecurityCodeLength,
		SecurityCodeMax: vault.MaxSecurityCodeLength,
		PINMin:          vault.MinCardPINLength,
		PINMax:          vault.MaxCardPINLength,
		BankName:        vault.MaxBankNameLength,
		BankSite:        vault.MaxOriginLength,
		Street:          vault.MaxStreetLength,
		City:            vault.MaxCityLength,
		Region:          vault.MaxRegionLength,
		PostalCode:      vault.MaxPostalCodeLength,
		Country:         vault.MaxCountryLength,
		Notes:           vault.MaxNotesLength,
	}, nil
}

// NoteLimits are the vault core's note bounds, counted in characters.
type NoteLimits struct {
	Label int `json:"label"`
	Body  int `json:"body"`
}

// GetNoteLimits reports the vault core's note bounds.
func (s *Service) GetNoteLimits() (NoteLimits, error) {
	return NoteLimits{Label: vault.MaxLabelLength, Body: vault.MaxNoteBodyLength}, nil
}

// SeedLimits are the vault core's seed text lengths in characters and list sizes.
type SeedLimits struct {
	Label        int `json:"label"`
	Words        int `json:"words"`
	Word         int `json:"word"`
	Passphrase   int `json:"passphrase"`
	Path         int `json:"path"`
	Key          int `json:"key"`
	Codes        int `json:"codes"`
	Code         int `json:"code"`
	Wallet       int `json:"wallet"`
	Addresses    int `json:"addresses"`
	AddressLabel int `json:"addressLabel"`
	Address      int `json:"address"`
	Notes        int `json:"notes"`
}

// GetSeedLimits reports the vault core's seed bounds.
func (s *Service) GetSeedLimits() (SeedLimits, error) {
	return SeedLimits{
		Label:        vault.MaxLabelLength,
		Words:        vault.MaxSeedWords,
		Word:         vault.MaxSeedWordLength,
		Passphrase:   vault.MaxSeedPassphraseLength,
		Path:         vault.MaxDerivationPathLength,
		Key:          vault.MaxPrivateKeyLength,
		Codes:        vault.MaxBackupCodes,
		Code:         vault.MaxBackupCodeLength,
		Wallet:       vault.MaxWalletNameLength,
		Addresses:    vault.MaxSeedAddresses,
		AddressLabel: vault.MaxSeedAddressLabelLength,
		Address:      vault.MaxSeedAddressLength,
		Notes:        vault.MaxNotesLength,
	}, nil
}

// GetCredentialLimits reports the vault core's credential bounds.
func (s *Service) GetCredentialLimits() (CredentialLimits, error) {
	return CredentialLimits{
		Label:    vault.MaxLabelLength,
		Website:  vault.MaxOriginLength,
		Websites: vault.MaxCredentialWebsites,
		Login:    vault.MaxLoginLength,
		Email:    vault.MaxEmailLength,
		Password: vault.MaxPasswordLength,
		Notes:    vault.MaxNotesLength,
		TOTP:     vault.MaxTOTPLength,
	}, nil
}
