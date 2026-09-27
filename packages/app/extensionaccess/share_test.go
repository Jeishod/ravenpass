package extensionaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/linkproto"
	"github.com/dortanes/ravenpass/packages/app/linkserver"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/app/verification"
	"github.com/dortanes/ravenpass/packages/vault"
)

var (
	alexID     = vault.ID{0xa1}
	passportID = vault.ID{0xb2}
	scanPDF    = []byte("%PDF-1.4\n%%EOF\n")
)

// fakeIdentities holds Alex with a photo and a passport scan, and Sam with nothing.
type fakeIdentities struct {
	err     error
	readErr error
	reads   []string
}

func (f *fakeIdentities) IdentityFiles() ([]vaultservice.IdentityFiles, error) {
	return []vaultservice.IdentityFiles{
		{ID: alexID, Label: "Alex", Thumbnail: []byte{0xff, 0xd8, 0x01}, Files: []vaultservice.IdentityFile{
			{ID: vaultservice.PhotoFile, Kind: vaultservice.FilePhoto, Name: "photo.jpg", MediaType: vault.MediaJPEG, Thumbnail: []byte{0xff, 0xd8, 0x01}},
			{ID: passportID.String(), Kind: vaultservice.FileScan, Name: "passport.pdf", MediaType: vault.MediaPDF, Document: &vaultservice.FileDocument{Type: vault.DocumentPassport}},
		}},
		{ID: vault.ID{0xc3}, Label: "Sam"},
	}, f.err
}

func (f *fakeIdentities) ReadIdentityFile(identity vault.ID, file string) (vaultservice.FileContent, error) {
	f.reads = append(f.reads, identity.String()+" "+file)
	if f.readErr != nil {
		return vaultservice.FileContent{}, f.readErr
	}
	return vaultservice.FileContent{Name: "passport.pdf", MediaType: vault.MediaPDF, Content: bytes.Clone(scanPDF)}, nil
}

// fakeVerifier asks the person with method and answers err.
type fakeVerifier struct {
	method  verification.Method
	err     error
	reasons []confirmation.Reason
	context context.Context
}

func (f *fakeVerifier) Verify(ctx context.Context, reason confirmation.Reason, asked func(verification.Method)) error {
	f.reasons = append(f.reasons, reason)
	f.context = ctx
	asked(f.method)
	return f.err
}

func sharingAccess(t *testing.T, identities *fakeIdentities, verifier *fakeVerifier) *Access {
	t.Helper()
	access, err := New(&fakeCredentials{}, identities, &fakeIcons{}, verifier, &fakeChoices{}, &fakeUnlocks{})
	if err != nil {
		t.Fatal(err)
	}
	return access
}

func TestIdentitiesDescribesEachIdentitysFiles(t *testing.T) {
	access := sharingAccess(t, &fakeIdentities{}, &fakeVerifier{})
	identities, err := access.Identities()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(identities)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":"a1000000000000000000000000000000","label":"Alex","thumbnail":"/9gB","files":[` +
		`{"id":"photo","kind":"photo","name":"photo.jpg","mediaType":"image/jpeg","thumbnail":"/9gB"},` +
		`{"id":"b2000000000000000000000000000000","kind":"scan","name":"passport.pdf","mediaType":"application/pdf","document":{"type":"passport","label":""},"thumbnail":""}]},` +
		`{"id":"c3000000000000000000000000000000","label":"Sam","thumbnail":"","files":[]}]`
	if string(encoded) != want {
		t.Fatalf("identities encode as %s", encoded)
	}
	locked := sharingAccess(t, &fakeIdentities{err: vaultservice.ErrNotReady}, &fakeVerifier{})
	if _, err := locked.Identities(); !errors.Is(err, linkserver.ErrLocked) {
		t.Fatalf("identities of a locked vault: got %v, want ErrLocked", err)
	}
}

func TestShareReleasesAFileOnceThePersonVerifiesIt(t *testing.T) {
	for method, progress := range map[verification.Method]linkproto.Progress{
		verification.MethodDevice: linkproto.ProgressConfirmOnDevice,
		verification.MethodPIN:    linkproto.ProgressConfirmInRavenpass,
	} {
		identities, verifier := &fakeIdentities{}, &fakeVerifier{method: method}
		access := sharingAccess(t, identities, verifier)
		ctx, cancel := context.WithCancel(context.Background())
		var asked []linkproto.Progress
		file, content, err := access.Share(ctx, alexID.String(), passportID.String(), "https://www.example.com:8443", func(p linkproto.Progress) { asked = append(asked, p) })
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if file != (linkproto.SharedFile{Name: "passport.pdf", MediaType: vault.MediaPDF, Size: len(scanPDF)}) || !bytes.Equal(content, scanPDF) {
			t.Fatalf("shared %+v", file)
		}
		if !slices.Equal(asked, []linkproto.Progress{progress}) {
			t.Fatalf("method %d reported %q", method, asked)
		}
		if !slices.Equal(verifier.reasons, []confirmation.Reason{confirmation.Sharing("passport.pdf", "Alex", "example.com")}) {
			t.Fatalf("the prompt named %+v", verifier.reasons)
		}
		if verifier.context != ctx {
			t.Fatal("the verification does not end with the session")
		}
		if !slices.Equal(identities.reads, []string{alexID.String() + " " + passportID.String()}) {
			t.Fatalf("read %q", identities.reads)
		}
	}
}

func TestShareNamesTheSiteEvenWhereItsHostHasNoSite(t *testing.T) {
	verifier := &fakeVerifier{}
	access := sharingAccess(t, &fakeIdentities{}, verifier)
	if _, _, err := access.Share(context.Background(), alexID.String(), vaultservice.PhotoFile, "https://exa_mple.com", func(linkproto.Progress) {}); err != nil {
		t.Fatal(err)
	}
	if verifier.reasons[0].Site != "https://exa_mple.com" || verifier.reasons[0].File != "photo.jpg" {
		t.Fatalf("the prompt named %+v", verifier.reasons[0])
	}
}

func TestShareRefusesAFileTheVaultDoesNotHoldBeforeAskingAnyone(t *testing.T) {
	identities, verifier := &fakeIdentities{}, &fakeVerifier{}
	access := sharingAccess(t, identities, verifier)
	for name, request := range map[string][2]string{
		"an identity that is not an ID": {"alex", vaultservice.PhotoFile},
		"an unknown identity":           {vault.ID{0xee}.String(), vaultservice.PhotoFile},
		"an identity without a photo":   {vault.ID{0xc3}.String(), vaultservice.PhotoFile},
		"another identity's file":       {vault.ID{0xc3}.String(), passportID.String()},
		"an unknown file":               {alexID.String(), vault.ID{0xdd}.String()},
	} {
		if _, _, err := access.Share(context.Background(), request[0], request[1], "https://example.com", func(linkproto.Progress) {}); !errors.Is(err, linkserver.ErrNotFound) {
			t.Fatalf("%s: got %v, want ErrNotFound", name, err)
		}
	}
	if len(verifier.reasons) != 0 || len(identities.reads) != 0 {
		t.Fatalf("missing files reached the verifier %+v or the vault %q", verifier.reasons, identities.reads)
	}
}

func TestShareRefusalsBecomeTheServersRefusals(t *testing.T) {
	failure := errors.New("unlock record unreadable")
	for cause, want := range map[error]error{
		verification.ErrDeclined:     linkserver.ErrDeclined,
		verification.ErrUnverifiable: linkserver.ErrUnverifiable,
		vaultservice.ErrNotReady:     linkserver.ErrLocked,
		context.Canceled:             context.Canceled,
		failure:                      failure,
	} {
		identities := &fakeIdentities{}
		access := sharingAccess(t, identities, &fakeVerifier{err: cause})
		if _, _, err := access.Share(context.Background(), alexID.String(), passportID.String(), "https://example.com", func(linkproto.Progress) {}); !errors.Is(err, want) {
			t.Fatalf("a verification failing with %v: got %v, want %v", cause, err, want)
		}
		if len(identities.reads) != 0 {
			t.Fatalf("an unverified share read %q", identities.reads)
		}
	}
	for cause, want := range map[error]error{
		vaultservice.ErrNotReady: linkserver.ErrLocked,
		vault.ErrNotFound:        linkserver.ErrNotFound,
	} {
		access := sharingAccess(t, &fakeIdentities{readErr: cause}, &fakeVerifier{})
		if _, _, err := access.Share(context.Background(), alexID.String(), passportID.String(), "https://example.com", func(linkproto.Progress) {}); !errors.Is(err, want) {
			t.Fatalf("a read failing with %v after verification: got %v, want %v", cause, err, want)
		}
	}
	locked := sharingAccess(t, &fakeIdentities{err: vaultservice.ErrNotReady}, &fakeVerifier{})
	if _, _, err := locked.Share(context.Background(), alexID.String(), passportID.String(), "https://example.com", func(linkproto.Progress) {}); !errors.Is(err, linkserver.ErrLocked) {
		t.Fatalf("a share from a locked vault: got %v, want ErrLocked", err)
	}
}
