package bitwarden

// Bitwarden item types.
const (
	typeLogin          = 1
	typeSecureNote     = 2
	typeCard           = 3
	typeIdentity       = 4
	typeSSHKey         = 5
	typeBankAccount    = 6
	typeDriversLicense = 7
	typePassport       = 8
)

// Bitwarden custom field types that hold something other than text.
const (
	fieldBoolean = 2
	fieldLinked  = 3
)

// repromptPassword is the reprompt of an item Bitwarden shows only after the master password.
const repromptPassword = 1

// item is one Bitwarden JSON item, also filled from CSV; any string may be null, read as empty.
type item struct {
	Type           int            `json:"type"`
	Name           string         `json:"name"`
	Notes          string         `json:"notes"`
	Favorite       bool           `json:"favorite"`
	Reprompt       int            `json:"reprompt"`
	FolderID       string         `json:"folderId"`
	CollectionIDs  []string       `json:"collectionIds"`
	Fields         []field        `json:"fields"`
	Login          login          `json:"login"`
	Card           card           `json:"card"`
	Identity       identity       `json:"identity"`
	SSHKey         sshKey         `json:"sshKey"`
	BankAccount    bankAccount    `json:"bankAccount"`
	DriversLicense driversLicense `json:"driversLicense"`
	Passport       passport       `json:"passport"`
}

type field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  int    `json:"type"`
}

type login struct {
	URIs     []uri             `json:"uris"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	TOTP     string            `json:"totp"`
	Passkeys []fido2Credential `json:"fido2Credentials"`
}

type uri struct {
	URI string `json:"uri"`
}

type card struct {
	CardholderName string `json:"cardholderName"`
	Brand          string `json:"brand"`
	Number         string `json:"number"`
	ExpMonth       string `json:"expMonth"`
	ExpYear        string `json:"expYear"`
	Code           string `json:"code"`
}

type identity struct {
	Title          string `json:"title"`
	FirstName      string `json:"firstName"`
	MiddleName     string `json:"middleName"`
	LastName       string `json:"lastName"`
	Address1       string `json:"address1"`
	Address2       string `json:"address2"`
	Address3       string `json:"address3"`
	City           string `json:"city"`
	State          string `json:"state"`
	PostalCode     string `json:"postalCode"`
	Country        string `json:"country"`
	Company        string `json:"company"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	SSN            string `json:"ssn"`
	Username       string `json:"username"`
	PassportNumber string `json:"passportNumber"`
	LicenseNumber  string `json:"licenseNumber"`
}

type sshKey struct {
	PrivateKey     string `json:"privateKey"`
	PublicKey      string `json:"publicKey"`
	KeyFingerprint string `json:"keyFingerprint"`
}

type bankAccount struct {
	BankName         string `json:"bankName"`
	NameOnAccount    string `json:"nameOnAccount"`
	AccountType      string `json:"accountType"`
	AccountNumber    string `json:"accountNumber"`
	RoutingNumber    string `json:"routingNumber"`
	BranchNumber     string `json:"branchNumber"`
	PIN              string `json:"pin"`
	SwiftCode        string `json:"swiftCode"`
	IBAN             string `json:"iban"`
	BankContactPhone string `json:"bankContactPhone"`
}

type driversLicense struct {
	FirstName        string `json:"firstName"`
	MiddleName       string `json:"middleName"`
	LastName         string `json:"lastName"`
	DateOfBirth      string `json:"dateOfBirth"`
	LicenseNumber    string `json:"licenseNumber"`
	IssuingCountry   string `json:"issuingCountry"`
	IssuingState     string `json:"issuingState"`
	IssueDate        string `json:"issueDate"`
	ExpirationDate   string `json:"expirationDate"`
	IssuingAuthority string `json:"issuingAuthority"`
	LicenseClass     string `json:"licenseClass"`
}

type passport struct {
	Surname                      string `json:"surname"`
	GivenName                    string `json:"givenName"`
	DateOfBirth                  string `json:"dateOfBirth"`
	Sex                          string `json:"sex"`
	BirthPlace                   string `json:"birthPlace"`
	Nationality                  string `json:"nationality"`
	IssuingCountry               string `json:"issuingCountry"`
	PassportNumber               string `json:"passportNumber"`
	PassportType                 string `json:"passportType"`
	NationalIdentificationNumber string `json:"nationalIdentificationNumber"`
	IssuingAuthority             string `json:"issuingAuthority"`
	IssueDate                    string `json:"issueDate"`
	ExpirationDate               string `json:"expirationDate"`
}
