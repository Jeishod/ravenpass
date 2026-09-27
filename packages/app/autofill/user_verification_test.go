package autofill

import "testing"

func TestUserVerificationDecidesWhetherTheOwnerIsAskedAndMayGoUnverified(t *testing.T) {
	cases := []struct {
		stated           UserVerification
		known            bool
		asksOwner        bool
		allowsUnverified bool
	}{
		{VerificationRequired, true, true, false},
		{VerificationPreferred, true, true, true},
		{VerificationDiscouraged, true, false, true},
		{"", false, true, true},
		{"always", false, true, true},
	}
	for _, c := range cases {
		if got := c.stated.Known(); got != c.known {
			t.Errorf("%q known = %v, want %v", c.stated, got, c.known)
		}
		if got := c.stated.AsksOwner(); got != c.asksOwner {
			t.Errorf("%q asks the owner = %v, want %v", c.stated, got, c.asksOwner)
		}
		if got := c.stated.AllowsUnverified(); got != c.allowsUnverified {
			t.Errorf("%q allows unverified = %v, want %v", c.stated, got, c.allowsUnverified)
		}
	}
}
