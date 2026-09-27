package autofillbridge

import (
	"context"
	"errors"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/verification"
	"github.com/dortanes/ravenpass/packages/vault"
)

func (s *Server) passkeys(asked request) answer {
	found, err := s.vault.Passkeys(asked.RPID, asked.Allowed)
	if err != nil {
		return refusal(err)
	}
	listed := make([]passkeyChoice, len(found))
	for i, choice := range found {
		listed[i] = passkeyChoice{ID: choice.ID, CredentialID: choice.CredentialID, Account: choice.Account, Label: choice.Label}
	}
	return answer{Site: vault.SiteOf(asked.RPID), Passkeys: listed}
}

// signPasskey checks the relying party, the open vault and the passkey before the owner is asked.
func (s *Server) signPasskey(ctx context.Context, asked request) answer {
	hash, err := autofill.ClientDataHash(asked.ClientDataHash)
	if err != nil || !asked.Verification.Known() {
		return answer{Error: refusedInvalid}
	}
	held, err := s.vault.HeldPasskey(asked.RPID, asked.ID, asked.CredentialID)
	if err != nil {
		return refusal(err)
	}
	verified, err := s.verify(ctx, asked.Verification, confirmation.SigningIn(vault.SiteName(asked.RPID), held.Account))
	if err != nil {
		return refusal(err)
	}
	signed, err := s.vault.SignPasskey(autofill.PasskeySignIn{
		RPID: asked.RPID, ClientDataHash: hash, Verified: verified, ID: asked.ID, CredentialID: asked.CredentialID,
	})
	if err != nil {
		return refusal(err)
	}
	return answer{
		CredentialID: signed.CredentialID, AuthenticatorData: signed.AuthenticatorData,
		Signature: signed.Signature, UserHandle: signed.UserHandle,
	}
}

// createPasskey labels a new credential with its relying party ID; the system passes no relying party name.
func (s *Server) createPasskey(ctx context.Context, asked request) answer {
	hash, err := autofill.ClientDataHash(asked.ClientDataHash)
	if err != nil || !asked.Verification.Known() {
		return answer{Error: refusedInvalid}
	}
	if err := s.vault.CheckExclusions(asked.RPID, asked.Excluded); err != nil {
		return refusal(err)
	}
	verified, err := s.verify(ctx, asked.Verification, confirmation.SavingPasskey(vault.SiteName(asked.RPID)))
	if err != nil {
		return refusal(err)
	}
	created, err := s.vault.CreatePasskey(autofill.PasskeyCreation{
		RPID:           asked.RPID,
		User:           autofill.PasskeyUser{Handle: asked.User.Handle, Name: asked.User.Name},
		ClientDataHash: hash,
		Algorithms:     asked.Algorithms,
		Exclude:        asked.Excluded,
		Verified:       verified,
	})
	if err != nil {
		return refusal(err)
	}
	return answer{CredentialID: created.CredentialID, AttestationObject: created.AttestationObject}
}

// verify reports whether the owner verified reason as preference asks.
func (s *Server) verify(ctx context.Context, preference autofill.UserVerification, reason confirmation.Reason) (bool, error) {
	if !preference.AsksOwner() {
		return false, nil
	}
	err := s.verifier.Verify(ctx, reason, func(verification.Method) {})
	switch {
	case err == nil:
		return true, nil
	case preference.AllowsUnverified() && errors.Is(err, verification.ErrUnverifiable):
		return false, nil
	default:
		return false, err
	}
}
