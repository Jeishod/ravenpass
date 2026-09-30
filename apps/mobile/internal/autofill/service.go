// Package autofill answers Android's autofill service, Credential Manager provider and their screens over JNI.
package autofill

import (
	"context"

	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/vault"
)

// Service is the vault's autofill service; every method but Open fails with autofill.ErrLocked while locked.
type Service interface {
	Open() bool
	// Suggest lists the credentials matching r; code keeps only those with a one-time code setup.
	Suggest(r autofill.Requester, code bool) ([]autofill.Suggestion, error)
	// Fill releases the credential's values only while it still matches r.
	Fill(id string, r autofill.Requester) (autofill.Login, error)
	// OneTimeCode generates the credential's current code only while it still matches r.
	OneTimeCode(id string, r autofill.Requester) (autofill.Code, error)
	// Search ignores what the credentials match; code keeps only those with a one-time code setup.
	Search(query string, code bool) ([]autofill.Suggestion, error)
	Sites() ([]string, error)
	Link(id string, app autofill.App) error
	AddSite(id string, r autofill.Requester) error
	Hold(c autofill.Capture) (autofill.Offer, error)
	Save(token string, choice autofill.Choice) (created bool, err error)
	// Passkeys lists the passkeys of exactly rpID in allowed, or every discoverable one when allowed is empty.
	Passkeys(rpID string, allowed [][]byte) ([]autofill.PasskeyChoice, error)
	// HeldPasskey fails with autofill.ErrNotFound unless credential id holds the passkey credentialID of rpID.
	HeldPasskey(rpID, id string, credentialID []byte) (autofill.PasskeyChoice, error)
	// CheckExclusions fails with autofill.ErrPasskeyExcluded when the vault holds a passkey of rpID that exclude lists.
	CheckExclusions(rpID string, exclude [][]byte) error
	SignPasskey(signIn autofill.PasskeySignIn) (autofill.PasskeyAssertion, error)
	CreatePasskey(creation autofill.PasskeyCreation) (autofill.CreatedPasskey, error)
}

// Vault opens the vault from the unlock screen, and checks the open vault's PIN.
type Vault interface {
	UnlockMethods() (vaultservice.Methods, error)
	Unlock(reason string) (vault.Head, error)
	UnlockWithPIN(pin string) (vault.Head, error)
	VerifyPIN(pin string) error
}

// Owner asks the device to verify its owner until the owner answers or ctx ends.
type Owner interface {
	AuthenticateOwner(ctx context.Context, reason string) error
}

// Icons reads the website icons the device keeps for the open vault, from the cache alone.
type Icons interface {
	// Cached returns a base64 PNG, empty when icons are off or none is cached.
	Cached(site string) (string, error)
}

// Page serves the screens' page the icons, language, interface size and appearance the app's window reads.
type Page interface {
	SiteIcon(site string) (api.SiteIcon, error)
	GetLanguage() (api.LanguageSettings, error)
	GetInterfaceSize() (api.InterfaceSize, error)
	GetAppearance() (api.Appearance, error)
}
