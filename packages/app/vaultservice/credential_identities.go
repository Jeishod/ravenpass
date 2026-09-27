package vaultservice

import (
	"strings"

	"github.com/dortanes/ravenpass/packages/vault"
)

// IdentityKind is what a credential identity fills.
type IdentityKind uint8

// Kinds of CredentialIdentity.
const (
	IdentityPassword IdentityKind = iota + 1
	IdentityPasskey
	IdentityCode
)

// CredentialIdentity is a secret-free account the platform's AutoFill may suggest; Site is the RP ID for a passkey.
type CredentialIdentity struct {
	Kind         IdentityKind
	Site         string
	User         string
	Credential   vault.ID
	CredentialID []byte
	UserHandle   []byte
}

// CredentialIdentities lists the open vault's accounts a platform may suggest.
func (s *Service) CredentialIdentities() ([]CredentialIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, ErrNotReady
	}
	entries, err := s.session.List()
	if err != nil {
		return nil, err
	}
	return credentialIdentitiesOf(entries, s.recordedUserHandle), nil
}

// recordedUserHandle reads a passkey's user handle from its record, nil on failure. The caller holds s.mu.
func (s *Service) recordedUserHandle(credential vault.ID, credentialID []byte) []byte {
	passkey, err := s.session.ReadPasskey(credential, credentialID)
	if err != nil {
		return nil
	}
	clear(passkey.PrivateKey)
	return passkey.UserHandle
}

// credentialIdentitiesOf includes non-discoverable passkeys and omits IPv6 sites, which a service identifier cannot carry.
func credentialIdentitiesOf(entries []vault.Entry, recorded func(credential vault.ID, credentialID []byte) []byte) []CredentialIdentity {
	var identities []CredentialIdentity
	for _, entry := range entries {
		if entry.Kind != vault.KindCredential {
			continue
		}
		account := entryAccount(entry)
		for _, site := range entry.Sites {
			if strings.Contains(site, ":") {
				continue
			}
			identities = append(identities, CredentialIdentity{Kind: IdentityPassword, Site: site, User: account, Credential: entry.ID})
			if entry.Code.Digits > 0 {
				identities = append(identities, CredentialIdentity{Kind: IdentityCode, Site: site, User: account, Credential: entry.ID})
			}
		}
		for _, face := range entry.Passkeys {
			handle := face.UserHandle
			if len(handle) == 0 {
				handle = recorded(entry.ID, face.CredentialID)
			}
			if len(handle) == 0 {
				continue
			}
			user := face.UserName
			if user == "" {
				user = account
			}
			identities = append(identities, CredentialIdentity{
				Kind: IdentityPasskey, Site: face.RPID, User: user, Credential: entry.ID,
				CredentialID: face.CredentialID, UserHandle: handle,
			})
		}
	}
	return identities
}
