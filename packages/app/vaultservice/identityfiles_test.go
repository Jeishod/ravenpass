package vaultservice

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func testPhoto(t *testing.T) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, vault.PhotoSize, vault.PhotoSize))
	for x := range vault.PhotoSize {
		for y := range vault.PhotoSize {
			picture.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// identityWithFiles has a photo, a passport with a PDF scan, and another document with a second scan.
func identityWithFiles(t *testing.T) vault.IdentityInput {
	t.Helper()
	input := identityWithScan(t)
	input.Photo = testPhoto(t)
	other, err := vault.PrepareScan("membership.pdf", testPDF)
	if err != nil {
		t.Fatal(err)
	}
	input.Documents = append(input.Documents, vault.Document{Type: vault.DocumentOther, Label: "Club", Number: "M-7", Attach: []vault.PreparedScan{other}})
	return input
}

func TestIdentityFilesListsEachIdentitysPhotoAndScans(t *testing.T) {
	service, _ := readyVault(t)
	owner, err := service.CreateIdentity(identityWithFiles(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	bare, err := service.CreateIdentity(vault.IdentityInput{Label: "Sam"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	createTestCredential(t, service, vault.CredentialInput{Label: "Mail"})
	scans, err := service.ScansOf(owner)
	if err != nil || len(scans) != 2 {
		t.Fatalf("scans = %+v, error = %v", scans, err)
	}
	entries, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	var thumbnail []byte
	for _, entry := range entries {
		if entry.ID == owner {
			thumbnail = entry.Thumbnail
		}
	}

	got, err := service.IdentityFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []IdentityFiles{
		{ID: owner, Label: "Alex", Thumbnail: thumbnail, Files: []IdentityFile{
			{ID: PhotoFile, Kind: FilePhoto, Name: "photo.jpg", MediaType: vault.MediaJPEG, Thumbnail: thumbnail},
			{ID: scans[0].ID.String(), Kind: FileScan, Name: "passport.pdf", MediaType: vault.MediaPDF, Document: &FileDocument{Type: vault.DocumentPassport}},
			{ID: scans[1].ID.String(), Kind: FileScan, Name: "membership.pdf", MediaType: vault.MediaPDF, Document: &FileDocument{Type: vault.DocumentOther, Label: "Club"}},
		}},
		{ID: bare, Label: "Sam"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identity files = %#v, want %#v", got, want)
	}
}

func TestReadIdentityFileReadsThePhotoOrAScanOfThatIdentity(t *testing.T) {
	service, _ := readyVault(t)
	owner, err := service.CreateIdentity(identityWithFiles(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateIdentity(identityWithScan(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	stored := readTestIdentity(t, service, owner).Photo
	service.ClearSelection()

	photo, err := service.ReadIdentityFile(owner, PhotoFile)
	if err != nil {
		t.Fatal(err)
	}
	if photo.Name != "photo.jpg" || photo.MediaType != vault.MediaJPEG || !bytes.Equal(photo.Content, stored) {
		t.Fatalf("photo = %s as %s, %d bytes", photo.Name, photo.MediaType, len(photo.Content))
	}
	scans, err := service.ScansOf(owner)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := service.ReadIdentityFile(owner, scans[1].ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if scan.Name != "membership.pdf" || scan.MediaType != vault.MediaPDF || !bytes.Equal(scan.Content, testPDF) {
		t.Fatalf("scan = %s as %s", scan.Name, scan.MediaType)
	}

	elsewhere, err := service.ScansOf(other)
	if err != nil {
		t.Fatal(err)
	}
	for name, file := range map[string]struct {
		identity vault.ID
		file     string
	}{
		"another identity's scan":         {owner, elsewhere[0].ID.String()},
		"the photo of one without any":    {other, PhotoFile},
		"a file that is not an ID":        {owner, "front"},
		"an identity that does not exist": {vault.ID{0xff}, scans[0].ID.String()},
		"the scan ID as its identity":     {scans[0].ID, PhotoFile},
	} {
		if _, err := service.ReadIdentityFile(file.identity, file.file); !errors.Is(err, vault.ErrNotFound) {
			t.Fatalf("%s: got %v, want ErrNotFound", name, err)
		}
	}
}

func TestIdentityFileReadsLeaveTheSelectionAlone(t *testing.T) {
	service, _ := readyVault(t)
	owner, err := service.CreateIdentity(identityWithFiles(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	open := createTestCredential(t, service, vault.CredentialInput{Label: "Open", Password: "open-secret"})
	ticket, err := service.Select(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.IdentityFiles(); err != nil {
		t.Fatal(err)
	}
	scans, err := service.ScansOf(owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{PhotoFile, scans[0].ID.String()} {
		if _, err := service.ReadIdentityFile(owner, file); err != nil {
			t.Fatal(err)
		}
	}
	credential, err := service.ReadSelected(ticket)
	if err != nil || credential.ID != open || credential.Password != "open-secret" {
		t.Fatalf("the selected credential after reading files = %+v, error = %v", credential.ID, err)
	}
}

func TestIdentityFileReadsNeedAnOpenVault(t *testing.T) {
	service, _ := readyVault(t)
	owner, err := service.CreateIdentity(identityWithFiles(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	service.Lock()
	if _, err := service.IdentityFiles(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("listing while locked: got %v, want ErrNotReady", err)
	}
	if _, err := service.ReadIdentityFile(owner, PhotoFile); !errors.Is(err, ErrNotReady) {
		t.Fatalf("reading while locked: got %v, want ErrNotReady", err)
	}
}
