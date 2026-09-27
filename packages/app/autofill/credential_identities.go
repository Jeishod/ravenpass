package autofill

import "github.com/dortanes/ravenpass/packages/app/vaultservice"

// IdentityKind is what a credential identity fills.
type IdentityKind string

const (
	IdentityPassword IdentityKind = "password"
	IdentityPasskey  IdentityKind = "passkey"
	IdentityCode     IdentityKind = "code"
)

var identityKinds = map[vaultservice.IdentityKind]IdentityKind{
	vaultservice.IdentityPassword: IdentityPassword,
	vaultservice.IdentityPasskey:  IdentityPasskey,
	vaultservice.IdentityCode:     IdentityCode,
}

// CredentialIdentity is an account the platform may suggest unasked and stores unencrypted: never put a password, code setup or key in it.
type CredentialIdentity struct {
	Kind IdentityKind
	// Site is a credential's website, or the relying party ID for a passkey.
	Site string
	User string
	// Record is the credential holding it; Fill, OneTimeCode and SignPasskey recheck the match before release.
	Record       string
	CredentialID []byte
	UserHandle   []byte
}

// CredentialIdentities lists every account of the open vault a platform may suggest, reading the index only.
func (s *Service) CredentialIdentities() ([]CredentialIdentity, error) {
	identities, err := s.vault.CredentialIdentities()
	if err != nil {
		return nil, err
	}
	result := make([]CredentialIdentity, len(identities))
	for i, identity := range identities {
		result[i] = CredentialIdentity{
			Kind: identityKinds[identity.Kind], Site: identity.Site, User: identity.User, Record: identity.Credential.String(),
			CredentialID: identity.CredentialID, UserHandle: identity.UserHandle,
		}
	}
	return result, nil
}
