package autofill

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/autofill"
	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/devicerecords"
	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/unlock"
	"github.com/dortanes/ravenpass/packages/app/unlock/unlocktest"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	relyingParty  = "example.com"
	pageOrigin    = "https://login.example.com"
	loginRelation = "delegate_permission/common.get_login_creds"
)

var (
	challenge   = []byte("challenge from the relying party")
	browserHash = sha256.Sum256([]byte("client data the browser built"))
	userHandle  = []byte("user-1")
	passkeyID   = []byte{1, 2, 3}
	heldPasskey = autofill.PasskeyChoice{ID: "a", CredentialID: passkeyID, Account: "alex", DisplayName: "Alex", Label: "Example"}
)

func encoded(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// signInJSON is the PublicKeyCredentialRequestOptionsJSON of a sign-in to rpID.
func signInJSON(t *testing.T, rpID, verification string, allow ...[]byte) string {
	t.Helper()
	var listed []map[string]any
	for _, id := range allow {
		listed = append(listed, map[string]any{"type": "public-key", "id": encoded(id)})
	}
	return marshal(t, map[string]any{
		"challenge": encoded(challenge), "rpId": rpID, "allowCredentials": listed, "userVerification": verification,
	})
}

// creationJSON is the PublicKeyCredentialCreationOptionsJSON of a passkey for alex on rpID.
func creationJSON(t *testing.T, rpID, verification string, exclude ...[]byte) string {
	t.Helper()
	var listed []map[string]any
	for _, id := range exclude {
		listed = append(listed, map[string]any{"type": "public-key", "id": encoded(id)})
	}
	return marshal(t, map[string]any{
		"rp":        map[string]any{"id": rpID, "name": "Example"},
		"user":      map[string]any{"id": encoded(userHandle), "name": "alex@example.com", "displayName": "Alex"},
		"challenge": encoded(challenge),
		"pubKeyCredParams": []map[string]any{
			{"type": "public-key", "alg": -7}, {"type": "public-key", "alg": -257}, {"type": "password", "alg": -8},
		},
		"excludeCredentials":     listed,
		"authenticatorSelection": map[string]any{"residentKey": "required", "userVerification": verification},
	})
}

func marshal(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// callerOf is app as Java names a caller, with the web origin Android passed when origin is set.
func callerOf(app autofill.App, origin string) map[string]any {
	caller := wireOf(app)
	if origin != "" {
		caller["origin"] = origin
	}
	return caller
}

func listing(caller map[string]any, request string) map[string]any {
	return map[string]any{"op": "passkeys", "caller": caller, "request": request}
}

func signing(caller map[string]any, request string, hash []byte, pin string) map[string]any {
	return map[string]any{
		"op": "sign-passkey", "caller": caller, "request": request, "clientDataHash": hash,
		"id": heldPasskey.ID, "credentialId": heldPasskey.CredentialID, "pin": pin,
	}
}

func creating(caller map[string]any, request string, hash []byte, pin string) map[string]any {
	return map[string]any{"op": "create-passkey", "caller": caller, "request": request, "clientDataHash": hash, "pin": pin}
}

// deviceUnlock is a vault that opens with the device's unlock, which the device offers.
var deviceUnlock = vaultservice.Methods{BiometryEnabled: true, BiometryAvailable: true}

// openPasskeys is an open vault holding heldPasskey for relyingParty.
func openPasskeys() *fakeService {
	return &fakeService{open: true, passkeyRPID: relyingParty, passkeyChoices: []autofill.PasskeyChoice{heldPasskey}}
}

func TestABrowserSignsInForTheOriginItVouchedFor(t *testing.T) {
	h := newHarness(openPasskeys(), nil)
	h.vault.methods = deviceUnlock
	chrome := callerOf(chrome(t), pageOrigin)
	var listed passkeysAnswer
	h.call(t, listing(chrome, signInJSON(t, relyingParty, "preferred")), &listed)
	want := []passkeyWire{{ID: "a", CredentialID: passkeyID, Account: "alex", DisplayName: "Alex"}}
	if listed.Status != statusOK || !reflect.DeepEqual(listed.Passkeys, want) {
		t.Fatalf("passkeys %+v", listed)
	}
	var signed passkeyAnswer
	h.call(t, signing(chrome, signInJSON(t, relyingParty, "preferred"), browserHash[:], ""), &signed)
	if signed.Status != statusOK || signed.Response == "" || len(h.service.signIns) != 1 {
		t.Fatalf("answer %+v", signed)
	}
	asked := h.service.signIns[0]
	if asked.RPID != relyingParty || asked.ClientDataHash == nil || *asked.ClientDataHash != browserHash ||
		asked.Origin != "" || asked.Challenge != nil || !asked.Verified || asked.ID != "a" || !bytes.Equal(asked.CredentialID, passkeyID) {
		t.Fatalf("signed %+v", asked)
	}
	if h.sites.count(relyingParty) != 0 {
		t.Fatal("a browser's relying party was looked up in Asset Links")
	}
	if want := []string{words(confirmation.SigningIn(vault.SiteName(relyingParty), "alex"))}; !slices.Equal(h.owner.reasons, want) {
		t.Fatalf("the owner was asked %q, want %q", h.owner.reasons, want)
	}
}

func TestAPasskeyWithoutAnAccountIsListedByItsCredentialsLabel(t *testing.T) {
	service := openPasskeys()
	service.passkeyChoices[0].Account = ""
	h := newHarness(service, nil)
	var listed passkeysAnswer
	h.call(t, listing(callerOf(chrome(t), pageOrigin), signInJSON(t, relyingParty, "preferred")), &listed)
	if listed.Status != statusOK || len(listed.Passkeys) != 1 || listed.Passkeys[0].Account != "Example" {
		t.Fatalf("passkeys %+v", listed)
	}
}

func TestABrowserThatGaveNoHashSignsTheClientDataRavenpassBuilds(t *testing.T) {
	service := openPasskeys()
	service.passkeyRPID = "login.example.com"
	h := newHarness(service, nil)
	h.vault.methods = deviceUnlock
	var signed passkeyAnswer
	h.call(t, signing(callerOf(chrome(t), pageOrigin), signInJSON(t, "", "preferred"), nil, ""), &signed)
	if signed.Status != statusOK || len(h.service.signIns) != 1 {
		t.Fatalf("answer %+v", signed)
	}
	asked := h.service.signIns[0]
	if asked.ClientDataHash != nil || asked.Origin != pageOrigin || !bytes.Equal(asked.Challenge, challenge) || asked.RPID != "login.example.com" {
		t.Fatalf("signed %+v", asked)
	}
}

func TestAnAppSignsInWhereTheRelyingPartysAssetLinksNameIt(t *testing.T) {
	app := signedApp("com.example.app", "example key")
	h := newHarness(openPasskeys(), map[string]siteResponse{relyingParty: served(statementFor(app, loginRelation))})
	h.vault.methods = deviceUnlock
	caller := callerOf(app, "")
	var listed passkeysAnswer
	h.call(t, listing(caller, signInJSON(t, relyingParty, "preferred", passkeyID)), &listed)
	if listed.Status != statusOK || len(listed.Passkeys) != 1 {
		t.Fatalf("passkeys %+v", listed)
	}
	var signed passkeyAnswer
	h.call(t, signing(caller, signInJSON(t, relyingParty, "preferred"), browserHash[:], ""), &signed)
	if signed.Status != statusOK || len(h.service.signIns) != 1 {
		t.Fatalf("answer %+v", signed)
	}
	asked := h.service.signIns[0]
	wantOrigin := "android:apk-key-hash:" + encoded(app.Signers[0][:])
	if asked.ClientDataHash != nil || asked.Origin != wantOrigin || !bytes.Equal(asked.Challenge, challenge) || asked.RPID != relyingParty {
		t.Fatalf("an app's sign-in went to the vault as %+v", asked)
	}
	var created passkeyAnswer
	h.call(t, creating(caller, creationJSON(t, relyingParty, "preferred"), browserHash[:], ""), &created)
	if created.Status != statusOK || len(h.service.creations) != 1 || h.service.creations[0].Origin != wantOrigin || h.service.creations[0].ClientDataHash != nil {
		t.Fatalf("answer %+v, created %+v", created, h.service.creations)
	}
}

func TestCallersTheRelyingPartyDoesNotTrustAreRefused(t *testing.T) {
	app := signedApp("com.example.app", "example key")
	other := signedApp("com.lookalike", "lookalike key")
	sites := map[string]siteResponse{relyingParty: served(statementFor(app, loginRelation))}
	for name, test := range map[string]struct {
		caller map[string]any
		rpID   string
	}{
		"an app the Asset Links do not name":        {caller: callerOf(other, ""), rpID: relyingParty},
		"an app claiming a web origin":              {caller: callerOf(app, pageOrigin), rpID: relyingParty},
		"a copy of Chrome signed by another":        {caller: callerOf(autofill.App{Package: "com.android.chrome", Signers: other.Signers}, pageOrigin), rpID: relyingParty},
		"a browser asking for another site":         {caller: callerOf(chrome(t), pageOrigin), rpID: "other.com"},
		"a browser asking for a public suffix":      {caller: callerOf(chrome(t), pageOrigin), rpID: "com"},
		"a browser asking for a subdomain":          {caller: callerOf(chrome(t), "https://example.com"), rpID: "login.example.com"},
		"an app for a relying party with a port":    {caller: callerOf(app, ""), rpID: "example.com:443"},
		"a browser without its certificate digests": {caller: map[string]any{"package": "com.android.chrome", "origin": pageOrigin}, rpID: relyingParty},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(openPasskeys(), sites)
			h.vault.methods = deviceUnlock
			var listed passkeysAnswer
			h.call(t, listing(test.caller, signInJSON(t, test.rpID, "preferred")), &listed)
			var signed, created passkeyAnswer
			h.call(t, signing(test.caller, signInJSON(t, test.rpID, "preferred"), browserHash[:], ""), &signed)
			h.call(t, creating(test.caller, creationJSON(t, test.rpID, "preferred"), browserHash[:], ""), &created)
			if listed.Status != statusFailed || listed.Passkeys != nil || signed.Status != statusFailed || created.Status != statusFailed {
				t.Fatalf("listed %+v, signed %+v, created %+v", listed, signed, created)
			}
			if len(h.service.signIns) != 0 || len(h.service.creations) != 0 || len(h.owner.reasons) != 0 {
				t.Fatalf("the vault signed %+v and created %+v, the owner was asked %q", h.service.signIns, h.service.creations, h.owner.reasons)
			}
		})
	}
}

func TestAssetLinksAreLookedUpOnlyForARelyingPartyTheVaultHoldsPasskeysOf(t *testing.T) {
	app := signedApp("com.example.app", "example key")
	service := openPasskeys()
	service.passkeyRPID = "other.com"
	h := newHarness(service, map[string]siteResponse{relyingParty: served(statementFor(app, loginRelation))})
	var listed passkeysAnswer
	h.call(t, listing(callerOf(app, ""), signInJSON(t, relyingParty, "preferred")), &listed)
	if listed.Status != statusOK || listed.Passkeys != nil || h.sites.count(relyingParty) != 0 {
		t.Fatalf("passkeys %+v after %d lookups", listed, h.sites.count(relyingParty))
	}
}

func TestALockedVaultOnlyAsksToUnlockForPasskeys(t *testing.T) {
	app := signedApp("com.example.app", "example key")
	service := openPasskeys()
	service.open = false
	h := newHarness(service, map[string]siteResponse{relyingParty: served(statementFor(app, loginRelation))})
	var listed passkeysAnswer
	h.call(t, listing(callerOf(app, ""), signInJSON(t, relyingParty, "preferred")), &listed)
	if listed.Status != statusLocked || listed.Passkeys != nil || len(service.listed) != 0 || h.sites.count(relyingParty) != 0 {
		t.Fatalf("a locked vault answered %+v after listing %q", listed, service.listed)
	}
}

func TestAPasskeyRequestOpensALockedVaultAsItsVerification(t *testing.T) {
	t.Run("with the device's unlock", func(t *testing.T) {
		service := openPasskeys()
		service.open = false
		h := newHarness(service, nil)
		h.vault.methods = deviceUnlock
		var signed passkeyAnswer
		h.call(t, signing(callerOf(chrome(t), pageOrigin), signInJSON(t, relyingParty, "discouraged"), browserHash[:], ""), &signed)
		if signed.Status != statusOK || h.opened != 1 || !slices.Equal(h.vault.reasons, []string{"Unlock Ravenpass to fill in"}) {
			t.Fatalf("answer %+v, opened %d, unlock reasons %q", signed, h.opened, h.vault.reasons)
		}
		if len(h.owner.reasons) != 0 || len(service.signIns) != 1 || !service.signIns[0].Verified {
			t.Fatalf("the owner was asked %q again, signed %+v", h.owner.reasons, service.signIns)
		}
	})
	t.Run("with the PIN", func(t *testing.T) {
		service := openPasskeys()
		service.open = false
		h := newHarness(service, nil)
		h.vault.methods = vaultservice.Methods{PINSet: true, PINAttemptsLeft: 4}
		request := creating(callerOf(chrome(t), pageOrigin), creationJSON(t, relyingParty, "preferred"), browserHash[:], "")
		var asked passkeyAnswer
		h.call(t, request, &asked)
		if asked.Status != statusPIN || len(service.creations) != 0 {
			t.Fatalf("answer %+v", asked)
		}
		h.vault.pinErr = unlock.ErrWrongPIN
		request["pin"] = "000000"
		var wrong passkeyAnswer
		h.call(t, request, &wrong)
		if wrong.Status != statusWrongPIN || wrong.AttemptsLeft != 4 || h.opened != 0 || len(service.creations) != 0 {
			t.Fatalf("a wrong PIN answered %+v", wrong)
		}
		h.vault.pinErr = nil
		request["pin"] = "135790"
		var created passkeyAnswer
		h.call(t, request, &created)
		if created.Status != statusOK || h.opened != 1 || len(service.creations) != 1 || !service.creations[0].Verified {
			t.Fatalf("answer %+v, created %+v", created, service.creations)
		}
		if !slices.Equal(h.vault.pins, []string{"000000", "135790"}) || len(h.vault.verified) != 0 {
			t.Fatalf("unlocked with %q, checked %q", h.vault.pins, h.vault.verified)
		}
	})
}

func TestAnOpenVaultVerifiesTheOwnerAsTheRelyingPartyAsks(t *testing.T) {
	for name, test := range map[string]struct {
		methods      vaultservice.Methods
		verification string
		ownerErr     error
		pin          string
		verifyErr    error
		want         status
		verified     bool
		prompted     bool
		checked      bool
	}{
		"preferred, with the device's unlock":   {methods: deviceUnlock, verification: "preferred", want: statusOK, verified: true, prompted: true},
		"required, with the device's unlock":    {methods: deviceUnlock, verification: "required", want: statusOK, verified: true, prompted: true},
		"an unknown preference reads preferred": {methods: deviceUnlock, verification: "sometimes", want: statusOK, verified: true, prompted: true},
		"discouraged":                           {methods: deviceUnlock, verification: "discouraged", want: statusOK},
		"the owner turns the prompt down":       {methods: deviceUnlock, verification: "preferred", ownerErr: ownerauth.ErrCanceled, want: statusCanceled, prompted: true},
		"the device does not recognize them":    {methods: deviceUnlock, verification: "preferred", ownerErr: ownerauth.ErrFailed, want: statusFailed, prompted: true},
		"a PIN is needed":                       {methods: vaultservice.Methods{PINSet: true}, verification: "required", want: statusPIN},
		"the PIN":                               {methods: vaultservice.Methods{PINSet: true}, verification: "required", pin: "135790", want: statusOK, verified: true, checked: true},
		"a wrong PIN":                           {methods: vaultservice.Methods{PINSet: true, PINAttemptsLeft: 3}, verification: "required", pin: "000000", verifyErr: unlock.ErrWrongPIN, want: statusWrongPIN, checked: true},
		"the PIN's last attempt":                {methods: vaultservice.Methods{PINSet: true}, verification: "required", pin: "000000", verifyErr: unlock.ErrPINRemoved, want: statusPINRemoved, checked: true},
		"a PIN before its delay":                {methods: vaultservice.Methods{PINSet: true, PINAttemptsLeft: 3}, verification: "required", pin: "000000", verifyErr: unlock.ErrTooSoon, want: statusTooSoon, checked: true},
		"preferred where no way verifies":       {verification: "preferred", want: statusOK},
		"required where no way verifies":        {verification: "required", want: statusFailed},
	} {
		t.Run(name, func(t *testing.T) {
			service := openPasskeys()
			h := newHarness(service, nil)
			h.vault.methods, h.vault.verifyErr, h.owner.err = test.methods, test.verifyErr, test.ownerErr
			var signed passkeyAnswer
			h.call(t, signing(callerOf(chrome(t), pageOrigin), signInJSON(t, relyingParty, test.verification), browserHash[:], test.pin), &signed)
			if signed.Status != test.want || (signed.Status == statusWrongPIN) != (signed.AttemptsLeft == 3) {
				t.Fatalf("answer %+v, want %s", signed, test.want)
			}
			if (len(h.owner.reasons) == 1) != test.prompted || (len(h.vault.verified) == 1) != test.checked || len(h.vault.reasons)+len(h.vault.pins) != 0 {
				t.Fatalf("prompted %q, checked the PIN %q, unlocked %q %q", h.owner.reasons, h.vault.verified, h.vault.reasons, h.vault.pins)
			}
			signedIn := len(service.signIns) == 1
			if signedIn != (test.want == statusOK) || signedIn && service.signIns[0].Verified != test.verified {
				t.Fatalf("signed %+v", service.signIns)
			}
		})
	}
}

func TestAPasskeyTheRelyingPartyDoesNotHoldIsNotSigned(t *testing.T) {
	for name, request := range map[string]map[string]any{
		"another credential":    {"id": "b"},
		"another passkey":       {"credentialId": []byte{9}},
		"another relying party": {"request": signInJSON(t, "login.example.com", "preferred")},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(openPasskeys(), nil)
			h.vault.methods = deviceUnlock
			sign := signing(callerOf(chrome(t), pageOrigin), signInJSON(t, relyingParty, "preferred"), browserHash[:], "")
			for field, value := range request {
				sign[field] = value
			}
			var signed passkeyAnswer
			h.call(t, sign, &signed)
			if signed.Status != statusFailed || len(h.service.signIns) != 0 || len(h.owner.reasons) != 0 {
				t.Fatalf("answer %+v, signed %+v, prompted %q", signed, h.service.signIns, h.owner.reasons)
			}
		})
	}
}

func TestACreationMeetingAnExcludedPasskeyEndsBeforeTheOwnerIsAsked(t *testing.T) {
	h := newHarness(openPasskeys(), nil)
	h.vault.methods = deviceUnlock
	var created passkeyAnswer
	h.call(t, creating(callerOf(chrome(t), pageOrigin), creationJSON(t, relyingParty, "preferred", []byte{8}, passkeyID), browserHash[:], ""), &created)
	if created.Status != statusExcluded || len(h.service.creations) != 0 || len(h.owner.reasons) != 0 {
		t.Fatalf("answer %+v, created %+v, prompted %q", created, h.service.creations, h.owner.reasons)
	}
	h.service.err = autofill.ErrPasskeyExcluded
	h.service.passkeyChoices = nil
	h.call(t, creating(callerOf(chrome(t), pageOrigin), creationJSON(t, relyingParty, "preferred", []byte{8}), browserHash[:], ""), &created)
	if created.Status != statusExcluded {
		t.Fatalf("the vault's refusal answered %+v", created)
	}
}

func TestACreationReachesTheVaultAsTheRelyingPartyAsked(t *testing.T) {
	h := newHarness(openPasskeys(), nil)
	h.vault.methods = deviceUnlock
	var created passkeyAnswer
	h.call(t, creating(callerOf(chrome(t), pageOrigin), creationJSON(t, relyingParty, "required", []byte{8}), browserHash[:], ""), &created)
	if created.Status != statusOK || len(h.service.creations) != 1 {
		t.Fatalf("answer %+v", created)
	}
	want := autofill.PasskeyCreation{
		RPID: relyingParty, RPName: "Example",
		User:           autofill.PasskeyUser{Handle: userHandle, Name: "alex@example.com", DisplayName: "Alex"},
		ClientDataHash: &browserHash, Algorithms: []int{-7, -257}, Exclude: [][]byte{{8}}, Verified: true,
	}
	if !reflect.DeepEqual(h.service.creations[0], want) {
		t.Fatalf("created %+v, want %+v", h.service.creations[0], want)
	}
	if reason := words(confirmation.SavingPasskey(vault.SiteName(relyingParty))); !slices.Equal(h.owner.reasons, []string{reason}) {
		t.Fatalf("the owner was asked %q", h.owner.reasons)
	}
}

func TestMalformedPasskeyRequestsFail(t *testing.T) {
	caller := callerOf(chrome(t), pageOrigin)
	for name, request := range map[string]map[string]any{
		"options that are not JSON":  signing(caller, "{", nil, ""),
		"a challenge not base64url":  signing(caller, `{"rpId":"example.com","challenge":"a+b/"}`, nil, ""),
		"a hash of another size":     signing(caller, signInJSON(t, relyingParty, "preferred"), []byte{1, 2}, ""),
		"a creation's hash too long": creating(caller, creationJSON(t, relyingParty, "preferred"), make([]byte, 33), ""),
		"a listing without a caller": {"op": "passkeys", "request": signInJSON(t, relyingParty, "preferred")},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(openPasskeys(), nil)
			h.vault.methods = deviceUnlock
			var answer passkeyAnswer
			h.call(t, request, &answer)
			if answer.Status != statusFailed || len(h.service.signIns)+len(h.service.creations) != 0 {
				t.Fatalf("answer %+v", answer)
			}
		})
	}
}

// credentialReply is a PublicKeyCredential as the relying party receives it.
type credentialReply struct {
	ID                      string         `json:"id"`
	RawID                   string         `json:"rawId"`
	Type                    string         `json:"type"`
	AuthenticatorAttachment string         `json:"authenticatorAttachment"`
	ClientExtensionResults  map[string]any `json:"clientExtensionResults"`
	Response                struct {
		ClientDataJSON     string   `json:"clientDataJSON"`
		AuthenticatorData  string   `json:"authenticatorData"`
		Signature          string   `json:"signature"`
		UserHandle         string   `json:"userHandle"`
		AttestationObject  string   `json:"attestationObject"`
		PublicKey          string   `json:"publicKey"`
		PublicKeyAlgorithm int      `json:"publicKeyAlgorithm"`
		Transports         []string `json:"transports"`
	} `json:"response"`
}

// decoded reads a base64url member of a reply.
func decoded(t *testing.T, member string) []byte {
	t.Helper()
	value, err := base64.RawURLEncoding.DecodeString(member)
	if err != nil {
		t.Fatalf("%q is not unpadded base64url: %v", member, err)
	}
	return value
}

func reply(t *testing.T, answer passkeyAnswer) credentialReply {
	t.Helper()
	if answer.Status != statusOK {
		t.Fatalf("answer %+v", answer)
	}
	var credential credentialReply
	if err := json.Unmarshal([]byte(answer.Response), &credential); err != nil {
		t.Fatal(err)
	}
	if credential.ID == "" || credential.ID != credential.RawID || credential.Type != "public-key" ||
		credential.AuthenticatorAttachment != "platform" || credential.ClientExtensionResults == nil {
		t.Fatalf("credential %+v", credential)
	}
	return credential
}

// presentOwner is a device whose owner can always authenticate.
type presentOwner struct{}

func (presentOwner) DeviceOwnerAvailable() bool { return true }

// keptInBackups keeps every file in the system's backups.
type keptInBackups struct{}

func (keptInBackups) Exclude(string) error { return nil }

// noGroup puts new credentials in no group.
type noGroup struct{}

func (noGroup) DefaultGroup() string { return "" }

// vaultHandler answers over a new vault the device unlocks and verifies the owner of, with sites answering Asset Links.
func vaultHandler(t *testing.T, sites map[string]siteResponse) *Handler {
	t.Helper()
	directory := t.TempDir()
	files, err := storage.NewManager(filepath.Join(directory, "storage.json"),
		storage.Target{Kind: storage.LocalFile, Path: filepath.Join(directory, localfile.DefaultVaultName)},
		vault.MaxContainerBytes, localfile.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	records, err := devicerecords.New(filepath.Join(directory, "device.json"), keptInBackups{})
	if err != nil {
		t.Fatal(err)
	}
	core, err := vaultservice.New(files, records, vaultservice.Device{
		Owner: presentOwner{}, PIN: unlocktest.NewBinding(), Platform: unlocktest.NewPresenceBinding(),
		PINKey: unlocktest.PINKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	phrase, err := core.BeginCreation()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.ConfirmCreation(phrase, vaultservice.MethodChoice{Biometry: true}); err != nil {
		t.Fatal(err)
	}
	service, err := autofill.New(core, noGroup{})
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(Options{
		Service: service, Vault: core, Owner: &fakeOwner{}, Icons: &fakeIcons{}, Page: &fakePage{},
		Links: newLinks(newFakeSites(sites)), Reason: func() string { return "unlock" }, Words: words,
		Opened: func() {}, Follow: func() {}, Requested: func() {}, Hold: func() func() { return func() {} },
	})
}

// clientDataOf reads the client data Ravenpass built for an app.
func clientDataOf(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

// verifySignature checks signature over authData followed by clientDataHash with a SubjectPublicKeyInfo publicKey.
func verifySignature(t *testing.T, publicKey, authData, signature []byte, clientDataHash [sha256.Size]byte) {
	t.Helper()
	parsed, err := x509.ParsePKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("the public key is a %T", parsed)
	}
	signed := sha256.Sum256(append(bytes.Clone(authData), clientDataHash[:]...))
	if !ecdsa.VerifyASN1(key, signed[:], signature) {
		t.Fatal("the signature does not verify with the passkey's public key")
	}
}

// checkAuthenticatorData checks the RP ID hash and the UP, UV, BE and BS flags, plus AT when attested (WebAuthn §6.1).
func checkAuthenticatorData(t *testing.T, authData []byte, attested bool) {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(relyingParty))
	flags := byte(0x01 | 0x04 | 0x08 | 0x10)
	if attested {
		flags |= 0x40
	}
	if len(authData) < 37 || !bytes.Equal(authData[:32], rpIDHash[:]) || authData[32] != flags {
		t.Fatalf("authenticator data %x", authData)
	}
}

func TestPasskeysCreatedAndSignedThroughTheVaultAreWebAuthnJSON(t *testing.T) {
	app := signedApp("com.example.app", "example key")
	appOrigin := "android:apk-key-hash:" + encoded(app.Signers[0][:])
	for name, test := range map[string]struct {
		caller map[string]any
		hash   []byte
	}{
		"for an app, over the client data Ravenpass builds": {caller: callerOf(app, "")},
		"for a browser, over the hash it gave":              {caller: callerOf(chrome(t), pageOrigin), hash: browserHash[:]},
	} {
		t.Run(name, func(t *testing.T) {
			h := vaultHandler(t, map[string]siteResponse{relyingParty: served(statementFor(app, loginRelation))})
			var created passkeyAnswer
			if err := json.Unmarshal(h.Call([]byte(marshal(t, creating(test.caller, creationJSON(t, relyingParty, "required"), test.hash, "")))), &created); err != nil {
				t.Fatal(err)
			}
			registration := reply(t, created)
			authData := decoded(t, registration.Response.AuthenticatorData)
			publicKey := decoded(t, registration.Response.PublicKey)
			checkAuthenticatorData(t, authData, true)
			if !bytes.Contains(decoded(t, registration.Response.AttestationObject), authData) ||
				registration.Response.PublicKeyAlgorithm != -7 || !slices.Equal(registration.Response.Transports, []string{"hybrid", "internal"}) {
				t.Fatalf("registration %+v", registration.Response)
			}
			if _, err := x509.ParsePKIXPublicKey(publicKey); err != nil {
				t.Fatal(err)
			}
			registered := decoded(t, registration.Response.ClientDataJSON)
			if test.hash != nil && string(registered) != "{}" {
				t.Fatalf("a browser's registration carries client data %s", registered)
			}
			if fields := clientDataOf(t, registered); test.hash == nil &&
				(fields["type"] != "webauthn.create" || fields["origin"] != appOrigin || fields["challenge"] != encoded(challenge)) {
				t.Fatalf("an app's registration carries client data %s", registered)
			}

			var listed passkeysAnswer
			if err := json.Unmarshal(h.Call([]byte(marshal(t, listing(test.caller, signInJSON(t, relyingParty, "required"))))), &listed); err != nil {
				t.Fatal(err)
			}
			if listed.Status != statusOK || len(listed.Passkeys) != 1 || encoded(listed.Passkeys[0].CredentialID) != registration.ID ||
				listed.Passkeys[0].Account != "alex@example.com" || listed.Passkeys[0].DisplayName != "Alex" {
				t.Fatalf("passkeys %+v", listed)
			}
			sign := signing(test.caller, signInJSON(t, relyingParty, "required"), test.hash, "")
			sign["id"], sign["credentialId"] = listed.Passkeys[0].ID, listed.Passkeys[0].CredentialID
			var signed passkeyAnswer
			if err := json.Unmarshal(h.Call([]byte(marshal(t, sign))), &signed); err != nil {
				t.Fatal(err)
			}
			assertion := reply(t, signed)
			if assertion.ID != registration.ID || !bytes.Equal(decoded(t, assertion.Response.UserHandle), userHandle) {
				t.Fatalf("assertion %+v", assertion)
			}
			signedData := decoded(t, assertion.Response.AuthenticatorData)
			checkAuthenticatorData(t, signedData, false)
			clientData := decoded(t, assertion.Response.ClientDataJSON)
			hash := sha256.Sum256(clientData)
			if test.hash != nil {
				if string(clientData) != "{}" {
					t.Fatalf("a browser's assertion carries client data %s", clientData)
				}
				hash = browserHash
			} else if fields := clientDataOf(t, clientData); fields["type"] != "webauthn.get" || fields["origin"] != appOrigin || fields["challenge"] != encoded(challenge) {
				t.Fatalf("an app's assertion carries client data %s", clientData)
			}
			verifySignature(t, publicKey, signedData, decoded(t, assertion.Response.Signature), hash)
		})
	}
}

func TestThePrivilegedAllowlistNamesTheTrustedBrowsers(t *testing.T) {
	h := newHarness(&fakeService{}, nil)
	var answer allowlistAnswer
	h.call(t, map[string]any{"op": "privileged-apps"}, &answer)
	var allowlist struct {
		Apps []privilegedApp `json:"apps"`
	}
	if err := json.Unmarshal([]byte(answer.Allowlist), &allowlist); answer.Status != statusOK || err != nil {
		t.Fatalf("answer %+v: %v", answer, err)
	}
	listed := map[string][]string{}
	for _, app := range allowlist.Apps {
		if app.Type != "android" {
			t.Fatalf("an entry of type %q", app.Type)
		}
		for _, signature := range app.Info.Signatures {
			digest, ok := fingerprint(signature.Fingerprint)
			if signature.Build != "release" || !ok || !trustedBrowser(autofill.App{Package: app.Info.PackageName, Signers: [][32]byte{digest}}) {
				t.Fatalf("%s: signature %+v", app.Info.PackageName, signature)
			}
			listed[app.Info.PackageName] = append(listed[app.Info.PackageName], signature.Fingerprint)
		}
	}
	if !reflect.DeepEqual(listed, browserCertificates) {
		t.Fatalf("the allowlist names %v, want %v", listed, browserCertificates)
	}
}

func TestABrowsersOriginIsReadAsSchemeAndHost(t *testing.T) {
	app := chrome(t)
	browser := callerWire{
		appWire: appWire{Package: app.Package, Signers: []string{base64.StdEncoding.EncodeToString(app.Signers[0][:])}},
		Origin:  "https://example.com/",
	}
	if caller, ok := checkCaller(browser, "example.com"); !ok || caller.origin != "https://example.com" || caller.rpID != "example.com" {
		t.Fatalf("Chrome's origin with a trailing slash: %+v, %t", caller, ok)
	}
}
