package api

import (
	"encoding/base64"

	"github.com/dortanes/ravenpass/packages/vault"
)

// PasskeyView is a credential's passkey as the workspace shows it, without its key.
type PasskeyView struct {
	// ID is the credential ID in unpadded base64url.
	ID string `json:"id"`
	// Site is the relying party ID.
	Site        string `json:"site"`
	Account     string `json:"account"`
	DisplayName string `json:"displayName"`
	// CreatedAt is in Unix milliseconds.
	CreatedAt int64 `json:"createdAt"`
}

// passkeyViews reports passkeys as an array, never null.
func passkeyViews(passkeys []vault.Passkey) []PasskeyView {
	views := make([]PasskeyView, len(passkeys))
	for i, passkey := range passkeys {
		views[i] = PasskeyView{
			ID:          base64.RawURLEncoding.EncodeToString(passkey.CredentialID),
			Site:        passkey.RPID,
			Account:     passkey.UserName,
			DisplayName: passkey.UserDisplayName,
			CreatedAt:   passkey.CreatedAt.UnixMilli(),
		}
	}
	return views
}

// passkeyCredentialIDs decodes PasskeyView IDs, reporting false for one that is not unpadded base64url.
func passkeyCredentialIDs(passkeyIDs []string) ([][]byte, bool) {
	if len(passkeyIDs) == 0 {
		return nil, true
	}
	credentialIDs := make([][]byte, len(passkeyIDs))
	for i, passkeyID := range passkeyIDs {
		credentialID, err := base64.RawURLEncoding.DecodeString(passkeyID)
		if err != nil {
			return nil, false
		}
		credentialIDs[i] = credentialID
	}
	return credentialIDs, true
}
