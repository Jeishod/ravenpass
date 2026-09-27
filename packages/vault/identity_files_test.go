package vault

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

// filesSession holds an identity with a photo and scans on two of three documents, and one with neither.
func filesSession(t *testing.T) (*Session, ID, ID) {
	t.Helper()
	session, _ := populatedSession(t)
	t.Cleanup(session.Lock)
	input := IdentityInput{
		Label: "Alex",
		Photo: pngBytes(t, patternPicture(PhotoSize, PhotoSize)),
		Documents: []Document{
			{Type: DocumentPassport, Number: "X1234567", Attach: []PreparedScan{prepared(t, "front.png", pngBytes(t, patternPicture(40, 30))), prepared(t, "back.pdf", tinyPDF)}},
			{Type: DocumentTaxNumber, Number: "TX-1"},
			{Type: DocumentOther, Label: "Membership", Number: "M-7", Attach: []PreparedScan{prepared(t, "card.pdf", tinyPDF)}},
		},
	}
	withFiles := commitIdentity(t, session, input, nil)
	bare := commitIdentity(t, session, IdentityInput{Label: "Sam"}, nil)
	return session, withFiles, bare
}

func TestIdentityFilesListsThePhotoAndEveryDocumentsScans(t *testing.T) {
	session, withFiles, bare := filesSession(t)
	commitCard(t, session, fullCard(), nil)
	documents := selectIdentity(t, session, withFiles).Documents
	scans, err := session.ScansOf(withFiles)
	if err != nil || len(scans) != 3 {
		t.Fatalf("scans = %+v, error = %v", scans, err)
	}
	thumbnail := listedEntry(t, session, withFiles).Thumbnail
	if len(thumbnail) == 0 {
		t.Fatal("the identity with a photo has no thumbnail")
	}
	got, err := session.IdentityFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []IdentityFiles{
		{Identity: withFiles, Label: "Alex", Thumbnail: thumbnail, Documents: []DocumentScans{
			{Type: DocumentPassport, Scans: []ScanSummary{scans[0], scans[1]}},
			{Type: DocumentOther, Label: "Membership", Scans: []ScanSummary{scans[2]}},
		}},
		{Identity: bare, Label: "Sam"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identity files = %#v, want %#v", got, want)
	}
	if got[0].Documents[0].Scans[0].ID != documents[0].Scans[0] || got[0].Documents[1].Scans[0].ID != documents[2].Scans[0] {
		t.Fatal("the scans are not listed in their documents' order")
	}
}

func TestReadIdentityPhotoReturnsTheStoredJPEG(t *testing.T) {
	session, withFiles, bare := filesSession(t)
	stored := selectIdentity(t, session, withFiles).Photo
	photo, err := session.ReadIdentityPhoto(withFiles)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(photo, stored) || !bytes.HasPrefix(photo, []byte{0xff, 0xd8}) {
		t.Fatal("the photo read differs from the stored JPEG")
	}
	scans, err := session.ScansOf(withFiles)
	if err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]ID{"an identity without a photo": bare, "a scan": scans[0].ID, "an unknown item": {0xff}} {
		if _, err := session.ReadIdentityPhoto(id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("the photo of %s: got %v, want ErrNotFound", name, err)
		}
	}
}

func TestIdentityFileReadsLeaveTheSelectionAlone(t *testing.T) {
	session, withFiles, _ := filesSession(t)
	open, err := session.BeginSelection(withFiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.IdentityFiles(); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ReadIdentityPhoto(withFiles); err != nil {
		t.Fatal(err)
	}
	if identity, err := session.ReadSelectedIdentity(open); err != nil || identity.ID != withFiles {
		t.Fatalf("the selection after reading files = %+v, error = %v", identity.ID, err)
	}
}

func TestIdentityFileReadsNeedAnOpenSession(t *testing.T) {
	session, withFiles, _ := filesSession(t)
	session.Lock()
	if _, err := session.IdentityFiles(); !errors.Is(err, ErrLocked) {
		t.Fatalf("IdentityFiles: %v", err)
	}
	if _, err := session.ReadIdentityPhoto(withFiles); !errors.Is(err, ErrLocked) {
		t.Fatalf("ReadIdentityPhoto: %v", err)
	}
}

func TestDocumentTypesAreExchangedByName(t *testing.T) {
	for documentType, name := range map[DocumentType]string{
		DocumentPassport:       "passport",
		DocumentDriversLicense: "drivers-license",
		DocumentIDCard:         "id-card",
		DocumentTaxNumber:      "tax-number",
		DocumentOther:          "other",
	} {
		if documentType.Name() != name {
			t.Errorf("%d is named %q, want %q", documentType, documentType.Name(), name)
		}
		if named, known := DocumentTypeNamed(name); !known || named != documentType {
			t.Errorf("%q names %d, %v", name, named, known)
		}
	}
	if DocumentType(99).Name() != "" {
		t.Error("an unknown type has a name")
	}
	if _, known := DocumentTypeNamed("visa"); known {
		t.Error("an unknown name names a type")
	}
}

func TestVerifyDeviceKeyChecksAnEnvelopeAgainstTheOpenVault(t *testing.T) {
	session, _ := populatedSession(t)
	defer session.Lock()
	key := bytes.Repeat([]byte{0x42}, 32)
	envelope, err := session.WrapDeviceKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.VerifyDeviceKey(key, envelope); err != nil {
		t.Fatalf("the key the envelope was made for: %v", err)
	}
	if err := session.VerifyDeviceKey(bytes.Repeat([]byte{0x43}, 32), envelope); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("another key: got %v, want ErrAuthentication", err)
	}
	if err := session.VerifyDeviceKey(key[:31], envelope); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a short key: got %v, want ErrInvalidInput", err)
	}

	other, err := Create()
	if err != nil {
		t.Fatal(err)
	}
	defer other.Session.Lock()
	foreign, err := other.Session.WrapDeviceKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.VerifyDeviceKey(key, foreign); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("another vault's envelope: got %v, want ErrAuthentication", err)
	}

	session.Lock()
	if err := session.VerifyDeviceKey(key, envelope); !errors.Is(err, ErrLocked) {
		t.Fatalf("a locked session: got %v, want ErrLocked", err)
	}
}
