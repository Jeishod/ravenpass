package autofill

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/authenticator"
	"github.com/dortanes/ravenpass/packages/vault"
)

// apkKeyHashOrigin prefixes the unpadded base64url SHA-256 digest of an app's signing certificate in its client data origin.
const apkKeyHashOrigin = "android:apk-key-hash:"

// callerWire is the app asking Credential Manager; Origin is set only for one of privilegedApps.
type callerWire struct {
	appWire
	Origin string `json:"origin,omitempty"`
}

// passkeyCaller is a browser with the web origin it vouched for, or an app the relying party's Asset Links must still name.
type passkeyCaller struct {
	rpID   string
	origin string
	app    autofill.App
}

// checkCaller checks what needs no network: a web origin counts only for a trusted browser.
func checkCaller(w callerWire, rpID string) (passkeyCaller, bool) {
	app, ok := w.app()
	if !ok {
		return passkeyCaller{}, false
	}
	if w.Origin != "" {
		origin, ok := vault.ParseOrigin(w.Origin)
		if !ok {
			return passkeyCaller{}, false
		}
		checked, err := authenticator.CheckRelyingParty(origin, rpID)
		if err != nil || !trustedBrowser(app) {
			return passkeyCaller{}, false
		}
		return passkeyCaller{rpID: checked, origin: origin}, true
	}
	checked, err := authenticator.RelyingPartyID(rpID)
	if err != nil {
		return passkeyCaller{}, false
	}
	origin := apkKeyHashOrigin + base64.RawURLEncoding.EncodeToString(app.Signers[0][:])
	return passkeyCaller{rpID: checked, origin: origin, app: app}, true
}

// vouched reports a browser, or an app the relying party's Asset Links name for sign-in.
func (h *Handler) vouched(c passkeyCaller) bool {
	if c.app.Package == "" {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookupWait)
	defer cancel()
	return slices.Contains(h.links.Verified(ctx, []string{c.rpID}, c.app), c.rpID)
}

// clientData is a browser's client data hash, or the origin and challenge Ravenpass builds it from.
type clientData struct {
	hash      *[sha256.Size]byte
	origin    string
	challenge []byte
}

// clientData ignores an app's hash: only a browser builds client data for another origin.
func (c passkeyCaller) clientData(hash, challenge []byte) (clientData, bool) {
	if c.app.Package != "" || hash == nil {
		return clientData{origin: c.origin, challenge: challenge}, true
	}
	checked, err := autofill.ClientDataHash(hash)
	if err != nil {
		return clientData{}, false
	}
	return clientData{hash: checked}, true
}

// passkeysRequest carries a PublicKeyCredentialRequestOptionsJSON in Request.
type passkeysRequest struct {
	Caller  callerWire `json:"caller"`
	Request string     `json:"request"`
}

// passkeyWire is a passkey's entry; Account is never empty.
type passkeyWire struct {
	ID           string `json:"id"`
	CredentialID []byte `json:"credentialId"`
	Account      string `json:"account"`
	DisplayName  string `json:"displayName"`
}

type passkeysAnswer struct {
	outcome
	Passkeys []passkeyWire `json:"passkeys,omitempty"`
}

// passkeys looks up an app's Asset Links only for a relying party the vault holds passkeys of.
func (h *Handler) passkeys(q passkeysRequest) passkeysAnswer {
	if !h.service.Open() {
		h.links.Forget()
		return passkeysAnswer{outcome: outcome{Status: statusLocked}}
	}
	var options requestOptions
	if json.Unmarshal([]byte(q.Request), &options) != nil {
		return passkeysAnswer{outcome: outcome{Status: statusFailed}}
	}
	caller, ok := checkCaller(q.Caller, options.RPID)
	if !ok {
		return passkeysAnswer{outcome: outcome{Status: statusFailed}}
	}
	choices, err := h.service.Passkeys(caller.rpID, credentialIDs(options.AllowCredentials))
	if err != nil {
		return passkeysAnswer{outcome: failure(err)}
	}
	if len(choices) == 0 {
		return passkeysAnswer{outcome: outcome{Status: statusOK}}
	}
	if !h.vouched(caller) {
		return passkeysAnswer{outcome: outcome{Status: statusFailed}}
	}
	wires := make([]passkeyWire, len(choices))
	for i, choice := range choices {
		wires[i] = passkeyWire{
			ID: choice.ID, CredentialID: choice.CredentialID, Account: cmp.Or(choice.Account, choice.Label), DisplayName: choice.DisplayName,
		}
	}
	return passkeysAnswer{outcome: outcome{Status: statusOK}, Passkeys: wires}
}

// signPasskeyRequest carries a PublicKeyCredentialRequestOptionsJSON in Request; an empty PIN asks for the device's unlock.
type signPasskeyRequest struct {
	Caller         callerWire `json:"caller"`
	Request        string     `json:"request"`
	ClientDataHash []byte     `json:"clientDataHash"`
	ID             string     `json:"id"`
	CredentialID   []byte     `json:"credentialId"`
	PIN            string     `json:"pin"`
}

// passkeyAnswer carries the WebAuthn JSON or the owner's next prompt; Refusal is logged and must never hold a request value.
type passkeyAnswer struct {
	outcome
	Response     string `json:"response,omitempty"`
	AttemptsLeft int    `json:"attemptsLeft,omitempty"`
	Refusal      string `json:"refusal,omitempty"`
}

// Checks a failed passkey request names as its refusal.
const (
	refusalRequest      = "request"
	refusalCaller       = "caller"
	refusalClientData   = "client-data"
	refusalRelyingParty = "relying-party"
	refusalPasskey      = "passkey"
	refusalVerification = "verification"
	refusalResponse     = "response"
	refusalVault        = "vault"
)

func ended(s status) *passkeyAnswer {
	return &passkeyAnswer{outcome: outcome{Status: s}}
}

func rejected(check string) *passkeyAnswer {
	return &passkeyAnswer{outcome: outcome{Status: statusFailed}, Refusal: check}
}

func failedWith(err error) *passkeyAnswer {
	answer := passkeyAnswer{outcome: failure(err)}
	if answer.Status == statusFailed {
		answer.Refusal = refusalVault
	}
	return &answer
}

// signPasskey checks the passkey against the caller before asking the owner when the vault is open.
func (h *Handler) signPasskey(q signPasskeyRequest) passkeyAnswer {
	var options requestOptions
	if json.Unmarshal([]byte(q.Request), &options) != nil {
		return *rejected(refusalRequest)
	}
	caller, ok := checkCaller(q.Caller, options.RPID)
	if !ok {
		return *rejected(refusalCaller)
	}
	client, ok := caller.clientData(q.ClientDataHash, options.Challenge)
	if !ok {
		return *rejected(refusalClientData)
	}
	verified, stop := h.confirm(q.PIN, options.UserVerification, func() (string, *passkeyAnswer) {
		held, err := h.service.HeldPasskey(caller.rpID, q.ID, q.CredentialID)
		if errors.Is(err, autofill.ErrNotFound) {
			return "", rejected(refusalPasskey)
		}
		if err != nil {
			return "", failedWith(err)
		}
		if !h.vouched(caller) {
			return "", rejected(refusalRelyingParty)
		}
		return h.words(confirmation.SigningIn(vault.SiteName(caller.rpID), held.Account)), nil
	})
	if stop != nil {
		return *stop
	}
	signed, err := h.service.SignPasskey(autofill.PasskeySignIn{
		RPID: caller.rpID, ClientDataHash: client.hash, Origin: client.origin, Challenge: client.challenge,
		Verified: verified, ID: q.ID, CredentialID: q.CredentialID,
	})
	if err != nil {
		return *failedWith(err)
	}
	return answered(authenticationJSON(signed))
}

// createPasskeyRequest carries a PublicKeyCredentialCreationOptionsJSON in Request.
type createPasskeyRequest struct {
	Caller         callerWire `json:"caller"`
	Request        string     `json:"request"`
	ClientDataHash []byte     `json:"clientDataHash"`
	PIN            string     `json:"pin"`
}

// createPasskey checks the caller and the excluded passkeys before asking the owner of an open vault.
func (h *Handler) createPasskey(q createPasskeyRequest) passkeyAnswer {
	var options creationOptions
	if json.Unmarshal([]byte(q.Request), &options) != nil {
		return *rejected(refusalRequest)
	}
	caller, ok := checkCaller(q.Caller, options.RP.ID)
	if !ok {
		return *rejected(refusalCaller)
	}
	client, ok := caller.clientData(q.ClientDataHash, options.Challenge)
	if !ok {
		return *rejected(refusalClientData)
	}
	exclude := credentialIDs(options.ExcludeCredentials)
	verified, stop := h.confirm(q.PIN, options.AuthenticatorSelection.UserVerification, func() (string, *passkeyAnswer) {
		if !h.vouched(caller) {
			return "", rejected(refusalRelyingParty)
		}
		if err := h.service.CheckExclusions(caller.rpID, exclude); err != nil {
			return "", failedWith(err)
		}
		return h.words(confirmation.SavingPasskey(vault.SiteName(caller.rpID))), nil
	})
	if stop != nil {
		return *stop
	}
	created, err := h.service.CreatePasskey(autofill.PasskeyCreation{
		RPID:   caller.rpID,
		RPName: options.RP.Name,
		User: autofill.PasskeyUser{
			Handle: options.User.ID, Name: options.User.Name, DisplayName: options.User.DisplayName,
		},
		ClientDataHash: client.hash, Origin: client.origin, Challenge: client.challenge,
		Algorithms: options.algorithms(),
		Exclude:    exclude,
		Verified:   verified,
	})
	if err != nil {
		return *failedWith(err)
	}
	return answered(registrationJSON(created, options.Extensions.CredProps))
}

func answered(response []byte, err error) passkeyAnswer {
	if err != nil {
		return *rejected(refusalResponse)
	}
	return passkeyAnswer{outcome: outcome{Status: statusOK}, Response: string(response)}
}

// confirm runs check, which needs the open vault, before verifying an open vault's owner and after unlocking a locked one.
func (h *Handler) confirm(pin string, preference autofill.UserVerification, check func() (reason string, stop *passkeyAnswer)) (bool, *passkeyAnswer) {
	if h.service.Open() {
		reason, stop := check()
		if stop != nil {
			return false, stop
		}
		return h.verifyOwner(pin, preference, reason)
	}
	verified, stop := h.verifyOwner(pin, preference, h.reason())
	if stop != nil {
		return false, stop
	}
	if _, stop := check(); stop != nil {
		return false, stop
	}
	return verified, nil
}

// verifyOwner uses pin, else the device's unlock, else asks for the PIN; a non-nil answer ends the request.
func (h *Handler) verifyOwner(pin string, preference autofill.UserVerification, reason string) (bool, *passkeyAnswer) {
	open := h.service.Open()
	if open && !preference.AsksOwner() {
		return false, nil
	}
	methods, err := h.vault.UnlockMethods()
	if err != nil {
		return false, failedWith(err)
	}
	device := methods.BiometryEnabled && methods.BiometryAvailable
	switch {
	case pin != "" && open:
		err = h.vault.VerifyPIN(pin)
	case pin != "":
		_, err = h.vault.UnlockWithPIN(pin)
	case device && open:
		err = h.owner.AuthenticateOwner(context.Background(), reason)
	case device:
		_, err = h.vault.Unlock(reason)
	case methods.PINSet:
		return false, ended(statusPIN)
	case open && preference.AllowsUnverified():
		return false, nil
	default:
		return false, rejected(refusalVerification)
	}
	if err != nil {
		refused := h.refused(err)
		answer := passkeyAnswer{outcome: refused.outcome, AttemptsLeft: refused.AttemptsLeft}
		if answer.Status == statusFailed {
			answer.Refusal = refusalVerification
		}
		return false, &answer
	}
	if !open {
		h.opened()
	}
	return true, nil
}
