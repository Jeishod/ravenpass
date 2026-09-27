package autofillbridge

import (
	"bytes"
	"context"
	"errors"
	"net"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/verification"
)

// es256 is the COSE algorithm identifier of ECDSA over P-256 with SHA-256.
const es256 = -7

func (v *fakeVault) Passkeys(rpID string, allowed [][]byte) ([]autofill.PasskeyChoice, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	switch {
	case rpID == "":
		return nil, autofill.ErrInvalidRelyingParty
	case !v.open:
		return nil, autofill.ErrLocked
	}
	var listed []autofill.PasskeyChoice
	for _, choice := range v.passkeys[rpID] {
		if len(allowed) == 0 || containsID(allowed, choice.CredentialID) {
			listed = append(listed, choice)
		}
	}
	return listed, nil
}

func (v *fakeVault) HeldPasskey(rpID, id string, credentialID []byte) (autofill.PasskeyChoice, error) {
	listed, err := v.Passkeys(rpID, [][]byte{credentialID})
	if err != nil {
		return autofill.PasskeyChoice{}, err
	}
	index := slices.IndexFunc(listed, func(choice autofill.PasskeyChoice) bool { return choice.ID == id })
	if index < 0 {
		return autofill.PasskeyChoice{}, autofill.ErrNotFound
	}
	return listed[index], nil
}

func (v *fakeVault) CheckExclusions(rpID string, exclude [][]byte) error {
	held, err := v.Passkeys(rpID, exclude)
	if err != nil {
		return err
	}
	if len(exclude) > 0 && len(held) > 0 {
		return autofill.ErrPasskeyExcluded
	}
	return nil
}

func (v *fakeVault) SignPasskey(signIn autofill.PasskeySignIn) (autofill.PasskeyAssertion, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.open {
		return autofill.PasskeyAssertion{}, autofill.ErrLocked
	}
	held := v.passkeys[signIn.RPID]
	index := slices.IndexFunc(held, func(choice autofill.PasskeyChoice) bool {
		return choice.ID == signIn.ID && bytes.Equal(choice.CredentialID, signIn.CredentialID)
	})
	if index < 0 {
		return autofill.PasskeyAssertion{}, autofill.ErrNotFound
	}
	v.signed = append(v.signed, signIn)
	return autofill.PasskeyAssertion{
		CredentialID: held[index].CredentialID, AuthenticatorData: []byte("authenticator data"),
		Signature: []byte("signature"), UserHandle: held[index].UserHandle,
	}, nil
}

func (v *fakeVault) CreatePasskey(creation autofill.PasskeyCreation) (autofill.CreatedPasskey, error) {
	if len(creation.Algorithms) > 0 && !slices.Contains(creation.Algorithms, es256) {
		return autofill.CreatedPasskey{}, autofill.ErrUnsupportedAlgorithm
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.open {
		return autofill.CreatedPasskey{}, autofill.ErrLocked
	}
	if slices.ContainsFunc(v.passkeys[creation.RPID], func(held autofill.PasskeyChoice) bool {
		return containsID(creation.Exclude, held.CredentialID)
	}) {
		return autofill.CreatedPasskey{}, autofill.ErrPasskeyExcluded
	}
	v.created = append(v.created, creation)
	return autofill.CreatedPasskey{ID: "new", CredentialID: []byte("new key"), AttestationObject: []byte("attestation")}, nil
}

func (v *fakeVault) signIns() []autofill.PasskeySignIn {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.signed)
}

func (v *fakeVault) creations() []autofill.PasskeyCreation {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.created)
}

func containsID(ids [][]byte, id []byte) bool {
	return slices.ContainsFunc(ids, func(listed []byte) bool { return bytes.Equal(listed, id) })
}

// fakeVerifier answers with err, or with ended set waits for the request to end and sends the cause there.
type fakeVerifier struct {
	mu      sync.Mutex
	err     error
	started chan struct{}
	ended   chan error
	reasons []confirmation.Reason
}

func (v *fakeVerifier) Verify(ctx context.Context, reason confirmation.Reason, asked func(verification.Method)) error {
	v.mu.Lock()
	v.reasons = append(v.reasons, reason)
	v.mu.Unlock()
	if v.started != nil {
		v.started <- struct{}{}
	}
	if v.ended == nil {
		return v.err
	}
	<-ctx.Done()
	v.ended <- ctx.Err()
	return ctx.Err()
}

func (v *fakeVerifier) asked() []confirmation.Reason {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.reasons)
}

var (
	clientHash = bytes.Repeat([]byte{7}, 32)
	alexKey    = autofill.PasskeyChoice{ID: "a", CredentialID: []byte("alex key"), UserHandle: []byte("alex handle"), Account: "alex", DisplayName: "Alex", Label: "Example"}
	samKey     = autofill.PasskeyChoice{ID: "b", CredentialID: []byte("sam key"), UserHandle: []byte("sam handle"), Account: "sam", Label: "Example"}
	otherKey   = autofill.PasskeyChoice{ID: "c", CredentialID: []byte("other key"), UserHandle: []byte("alex handle"), Account: "alex", Label: "Other"}
)

// passkeyVault is an open vault with two passkeys for example.com and one for example.org.
func passkeyVault() *fakeVault {
	return &fakeVault{open: true, passkeys: map[string][]autofill.PasskeyChoice{
		"example.com": {alexKey, samKey},
		"example.org": {otherKey},
	}}
}

func signIn(verify autofill.UserVerification) request {
	return request{
		Op: opPasskeySign, RPID: "example.com", ClientDataHash: clientHash,
		ID: alexKey.ID, CredentialID: alexKey.CredentialID, Verification: verify,
	}
}

func creation(verify autofill.UserVerification) request {
	return request{
		Op: opPasskeyCreate, RPID: "example.com", ClientDataHash: clientHash,
		User:       passkeyUser{Handle: []byte("new handle"), Name: "alex@example.com"},
		Algorithms: []int{es256, -257}, Excluded: [][]byte{otherKey.CredentialID}, Verification: verify,
	}
}

func TestPasskeysListsTheRelyingPartysPasskeysWithoutTheirUserHandlesAndNamesItsSite(t *testing.T) {
	path := serve(t, passkeyVault(), newQueue(t), admitted)
	reply := ask(t, path, request{Op: opPasskeys, RPID: "example.com"})
	want := []passkeyChoice{
		{ID: "a", CredentialID: []byte("alex key"), Account: "alex", Label: "Example"},
		{ID: "b", CredentialID: []byte("sam key"), Account: "sam", Label: "Example"},
	}
	if reply.Error != "" || reply.Site != "example.com" || !reflect.DeepEqual(reply.Passkeys, want) {
		t.Fatalf("passkeys = %+v, want %+v for example.com", reply, want)
	}
	reply = ask(t, path, request{Op: opPasskeys, RPID: "example.com", Allowed: [][]byte{samKey.CredentialID, otherKey.CredentialID}})
	if !reflect.DeepEqual(reply.Passkeys, want[1:]) {
		t.Fatalf("allowed passkeys = %+v, want %+v", reply, want[1:])
	}
}

func TestPasskeysRefusals(t *testing.T) {
	vault := passkeyVault()
	path := serve(t, vault, newQueue(t), admitted)
	if reply := ask(t, path, request{Op: opPasskeys}); !reflect.DeepEqual(reply, answer{Error: refusedInvalid}) {
		t.Fatalf("no relying party = %+v", reply)
	}
	vault.setOpen(false)
	if reply := ask(t, path, request{Op: opPasskeys, RPID: "example.com"}); !reflect.DeepEqual(reply, answer{Error: refusedLocked}) {
		t.Fatalf("locked = %+v", reply)
	}
}

func TestPasskeySignSignsOverTheSystemsHashOnceTheOwnerVerifies(t *testing.T) {
	vault := passkeyVault()
	verifier := &fakeVerifier{}
	path := serveVerifying(t, vault, newQueue(t), verifier, admitted)
	reply := ask(t, path, signIn(autofill.VerificationRequired))
	want := answer{
		CredentialID: []byte("alex key"), AuthenticatorData: []byte("authenticator data"),
		Signature: []byte("signature"), UserHandle: []byte("alex handle"),
	}
	if !reflect.DeepEqual(reply, want) {
		t.Fatalf("sign-in = %+v, want %+v", reply, want)
	}
	if asked := verifier.asked(); !slices.Equal(asked, []confirmation.Reason{confirmation.SigningIn("example.com", "alex")}) {
		t.Fatalf("verified %+v", asked)
	}
	signed := vault.signIns()
	if len(signed) != 1 || !signed[0].Verified || signed[0].ClientDataHash == nil || !bytes.Equal(signed[0].ClientDataHash[:], clientHash) {
		t.Fatalf("signed %+v", signed)
	}
}

func TestPasskeySignThatDiscouragesVerificationAsksNoOne(t *testing.T) {
	vault := passkeyVault()
	verifier := &fakeVerifier{err: verification.ErrDeclined}
	path := serveVerifying(t, vault, newQueue(t), verifier, admitted)
	if reply := ask(t, path, signIn(autofill.VerificationDiscouraged)); reply.Error != "" || len(reply.Signature) == 0 {
		t.Fatalf("sign-in = %+v", reply)
	}
	if asked := verifier.asked(); len(asked) != 0 {
		t.Fatalf("verified %+v", asked)
	}
	if signed := vault.signIns(); len(signed) != 1 || signed[0].Verified {
		t.Fatalf("signed %+v", signed)
	}
}

func TestAPasskeyThatOnlyPrefersVerificationSignsUnverifiedWhereTheVaultCannotVerify(t *testing.T) {
	vault := passkeyVault()
	path := serve(t, vault, newQueue(t), admitted)
	if reply := ask(t, path, signIn(autofill.VerificationPreferred)); reply.Error != "" {
		t.Fatalf("preferred sign-in = %+v", reply)
	}
	if signed := vault.signIns(); len(signed) != 1 || signed[0].Verified {
		t.Fatalf("signed %+v", signed)
	}
	if reply := ask(t, path, signIn(autofill.VerificationRequired)); !reflect.DeepEqual(reply, answer{Error: refusedUnverifiable}) {
		t.Fatalf("required sign-in = %+v", reply)
	}
	if signed := vault.signIns(); len(signed) != 1 {
		t.Fatalf("a required verification that failed signed %+v", signed[1:])
	}
}

func TestPasskeySignRefusals(t *testing.T) {
	short := signIn(autofill.VerificationRequired)
	short.ClientDataHash = clientHash[:31]
	unknown := signIn("always")
	otherRP := signIn(autofill.VerificationRequired)
	otherRP.ID, otherRP.CredentialID = otherKey.ID, otherKey.CredentialID
	otherCredential := signIn(autofill.VerificationRequired)
	otherCredential.ID = samKey.ID
	cases := []struct {
		name     string
		locked   bool
		verifier error
		asked    request
		want     string
		verified bool
	}{
		{name: "locked", locked: true, asked: signIn(autofill.VerificationRequired), want: refusedLocked},
		{name: "another relying party's passkey", asked: otherRP, want: refusedNotFound},
		{name: "a passkey another credential holds", asked: otherCredential, want: refusedNotFound},
		{name: "declined", verifier: verification.ErrDeclined, asked: signIn(autofill.VerificationRequired), want: refusedDeclined, verified: true},
		{name: "declined though only preferred", verifier: verification.ErrDeclined, asked: signIn(autofill.VerificationPreferred), want: refusedDeclined, verified: true},
		{name: "a hash of another length", asked: short, want: refusedInvalid},
		{name: "an unknown verification", asked: unknown, want: refusedInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vault := passkeyVault()
			vault.open = !c.locked
			verifier := &fakeVerifier{err: c.verifier}
			path := serveVerifying(t, vault, newQueue(t), verifier, admitted)
			if reply := ask(t, path, c.asked); !reflect.DeepEqual(reply, answer{Error: c.want}) {
				t.Fatalf("got %+v, want %s", reply, c.want)
			}
			if asked := len(verifier.asked()) > 0; asked != c.verified {
				t.Fatalf("owner asked: %v, want %v", asked, c.verified)
			}
			if signed := vault.signIns(); len(signed) != 0 {
				t.Fatalf("signed %+v", signed)
			}
		})
	}
}

func TestPasskeyCreateCreatesOnceTheOwnerVerifies(t *testing.T) {
	vault := passkeyVault()
	verifier := &fakeVerifier{}
	path := serveVerifying(t, vault, newQueue(t), verifier, admitted)
	reply := ask(t, path, creation(autofill.VerificationPreferred))
	if want := (answer{CredentialID: []byte("new key"), AttestationObject: []byte("attestation")}); !reflect.DeepEqual(reply, want) {
		t.Fatalf("creation = %+v, want %+v", reply, want)
	}
	if asked := verifier.asked(); !slices.Equal(asked, []confirmation.Reason{confirmation.SavingPasskey("example.com")}) {
		t.Fatalf("verified %+v", asked)
	}
	created := vault.creations()
	if len(created) != 1 {
		t.Fatalf("created %+v", created)
	}
	hash := [32]byte(clientHash)
	want := autofill.PasskeyCreation{
		RPID: "example.com", User: autofill.PasskeyUser{Handle: []byte("new handle"), Name: "alex@example.com"},
		ClientDataHash: &hash, Algorithms: []int{es256, -257}, Exclude: [][]byte{otherKey.CredentialID}, Verified: true,
	}
	if !reflect.DeepEqual(created[0], want) {
		t.Fatalf("created %+v, want %+v", created[0], want)
	}
}

func TestPasskeyCreateRefusals(t *testing.T) {
	excluded := creation(autofill.VerificationRequired)
	excluded.Excluded = [][]byte{[]byte("unknown key"), samKey.CredentialID}
	unsupported := creation(autofill.VerificationDiscouraged)
	unsupported.Algorithms = []int{-257}
	noRP := creation(autofill.VerificationRequired)
	noRP.RPID = ""
	noHash := creation(autofill.VerificationRequired)
	noHash.ClientDataHash = nil
	cases := []struct {
		name     string
		locked   bool
		verifier error
		asked    request
		want     string
		verified bool
	}{
		{name: "locked", locked: true, asked: creation(autofill.VerificationRequired), want: refusedLocked},
		{name: "an excluded passkey", asked: excluded, want: refusedExcluded},
		{name: "no algorithm the vault signs with", asked: unsupported, want: refusedUnsupported},
		{name: "no relying party", asked: noRP, want: refusedInvalid},
		{name: "no client data hash", asked: noHash, want: refusedInvalid},
		{name: "declined", verifier: verification.ErrDeclined, asked: creation(autofill.VerificationRequired), want: refusedDeclined, verified: true},
		{name: "unverifiable", verifier: verification.ErrUnverifiable, asked: creation(autofill.VerificationRequired), want: refusedUnverifiable, verified: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vault := passkeyVault()
			vault.open = !c.locked
			verifier := &fakeVerifier{err: c.verifier}
			path := serveVerifying(t, vault, newQueue(t), verifier, admitted)
			if reply := ask(t, path, c.asked); !reflect.DeepEqual(reply, answer{Error: c.want}) {
				t.Fatalf("got %+v, want %s", reply, c.want)
			}
			if asked := len(verifier.asked()) > 0; asked != c.verified {
				t.Fatalf("owner asked: %v, want %v", asked, c.verified)
			}
			if created := vault.creations(); len(created) != 0 {
				t.Fatalf("created %+v", created)
			}
		})
	}
}

func TestAnExtensionThatHangsUpEndsTheOwnersVerification(t *testing.T) {
	vault := passkeyVault()
	verifier := &fakeVerifier{started: make(chan struct{}, 1), ended: make(chan error, 1)}
	path := serveVerifying(t, vault, newQueue(t), verifier, admitted)
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMessage(conn, maxRequestBytes, signIn(autofill.VerificationRequired)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-verifier.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the owner was never asked")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-verifier.ended:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("verification ended with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the verification outlived the extension's connection")
	}
	if signed := vault.signIns(); len(signed) != 0 {
		t.Fatalf("signed %+v", signed)
	}
}
