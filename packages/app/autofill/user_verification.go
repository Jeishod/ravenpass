package autofill

// UserVerification is the user verification a relying party asks for, WebAuthn §5.8.6; an absent or unknown one reads as preferred.
type UserVerification string

// The user verification requirements WebAuthn names.
const (
	VerificationRequired    UserVerification = "required"
	VerificationPreferred   UserVerification = "preferred"
	VerificationDiscouraged UserVerification = "discouraged"
)

// Known reports whether v is one of the requirements WebAuthn names.
func (v UserVerification) Known() bool {
	return v == VerificationRequired || v == VerificationPreferred || v == VerificationDiscouraged
}

// AsksOwner reports whether the owner of an open vault is asked to verify.
func (v UserVerification) AsksOwner() bool { return v != VerificationDiscouraged }

// AllowsUnverified reports whether the request goes on unverified where the vault cannot verify the owner.
func (v UserVerification) AllowsUnverified() bool { return v != VerificationRequired }
