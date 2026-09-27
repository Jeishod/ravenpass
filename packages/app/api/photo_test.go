package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/photos"
	"github.com/dortanes/ravenpass/packages/vault"
)

func photoFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "photos", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeBase64(t *testing.T, value string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func croppedPhoto(t *testing.T, service *Service) string {
	t.Helper()
	if _, err := service.stagePhoto(photoFixture(t, "upright.png")); err != nil {
		t.Fatal(err)
	}
	photo, err := service.CropIdentityPhoto(0, 0, 32)
	if err != nil {
		t.Fatal(err)
	}
	return photo
}

func TestStagedPhotoIsPreviewedAndCroppedOnce(t *testing.T) {
	service := newReadyService(t)
	draft, err := service.stagePhoto(photoFixture(t, "orientation-6.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !draft.Chosen || draft.Width != 48 || draft.Height != 32 {
		t.Fatalf("draft = %+v", draft)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(decodeBase64(t, draft.Preview)))
	if err != nil || config.Width != 48 || config.Height != 32 {
		t.Fatalf("preview is %dx%d, error = %v", config.Width, config.Height, err)
	}
	_, err = service.CropIdentityPhoto(17, 0, 32)
	assertFailure(t, err, failureInvalidItem)
	square, err := service.CropIdentityPhoto(16, 0, 32)
	if err != nil {
		t.Fatalf("a crop after a refused square: %v", err)
	}
	config, err = png.DecodeConfig(bytes.NewReader(decodeBase64(t, square)))
	if err != nil || config.Width != vault.PhotoSize || config.Height != vault.PhotoSize {
		t.Fatalf("crop is %dx%d, error = %v", config.Width, config.Height, err)
	}
	_, err = service.CropIdentityPhoto(16, 0, 32)
	assertFailure(t, err, failureFileNotSelected)
}

func TestDiscardAndLockEmptyTheSlot(t *testing.T) {
	service := newReadyService(t)
	for name, empty := range map[string]func() error{"discard": service.DiscardIdentityPhoto, "lock": service.Lock} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.stagePhoto(photoFixture(t, "upright.webp")); err != nil {
				t.Fatal(err)
			}
			if err := empty(); err != nil {
				t.Fatal(err)
			}
			_, err := service.CropIdentityPhoto(0, 0, 32)
			assertFailure(t, err, failureFileNotSelected)
		})
	}
}

func TestUnusablePicturesReportTheirOwnCodes(t *testing.T) {
	service := newReadyService(t)
	_, err := service.stagePhoto([]byte("not a picture"))
	assertFailure(t, err, failurePhotoUnsupported)
	_, err = service.stagePhoto(make([]byte, photos.MaxFileBytes+1))
	assertFailure(t, err, failurePhotoTooLarge)

	path := filepath.Join(t.TempDir(), "large.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(photos.MaxFileBytes + 1024); err != nil {
		t.Fatal(err)
	}
	file.Close()
	data, err := readChosenFile(path, photos.MaxFileBytes)
	if !errors.Is(err, photos.ErrTooLarge) || data != nil {
		t.Fatalf("read %d bytes of a file over the bound, error = %v", len(data), err)
	}
	fixture := filepath.Join("..", "photos", "testdata", "upright.png")
	data, err = readChosenFile(fixture, photos.MaxFileBytes)
	if err != nil || !bytes.Equal(data, photoFixture(t, "upright.png")) {
		t.Fatalf("read %d bytes of a file within the bound, error = %v", len(data), err)
	}
}

func TestIdentityPhotoCrossesTheBridgeAsBase64(t *testing.T) {
	service := newReadyService(t)
	bare, err := service.CreateIdentity(IdentityInput{Label: "Bare"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	input := IdentityInput{Label: "Alex", Photo: croppedPhoto(t, service)}
	id, err := service.CreateIdentity(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	summaries, err := service.ListIdentities()
	if err != nil || len(summaries) != 2 {
		t.Fatalf("summaries = %+v, error = %v", summaries, err)
	}
	if summaries[0].ID != bare || summaries[0].Thumbnail != "" {
		t.Fatalf("an identity without a photo listed a thumbnail: %+v", summaries[0])
	}
	thumbnail, err := jpeg.DecodeConfig(bytes.NewReader(decodeBase64(t, summaries[1].Thumbnail)))
	if err != nil || thumbnail.Width != 96 || thumbnail.Height != 96 {
		t.Fatalf("thumbnail is %dx%d, error = %v", thumbnail.Width, thumbnail.Height, err)
	}
	identity, err := service.ReadIdentity(id)
	if err != nil {
		t.Fatal(err)
	}
	photo, err := jpeg.DecodeConfig(bytes.NewReader(decodeBase64(t, identity.Photo)))
	if err != nil || photo.Width != vault.PhotoSize || photo.Height != vault.PhotoSize {
		t.Fatalf("photo is %dx%d, error = %v", photo.Width, photo.Height, err)
	}

	kept := identity.IdentityInput
	kept.Label = "Alex renamed"
	if err := service.UpdateIdentity(id, kept, nil); err != nil {
		t.Fatal(err)
	}
	if again, err := service.ReadIdentity(id); err != nil || again.Photo != identity.Photo {
		t.Fatalf("an update that passed the photo back changed it, error = %v", err)
	}

	broken := kept
	broken.Photo = "not base64!"
	assertFailure(t, service.UpdateIdentity(id, broken, nil), failureInvalidItem)

	kept.Photo = ""
	if err := service.UpdateIdentity(id, kept, nil); err != nil {
		t.Fatal(err)
	}
	removed, err := service.ReadIdentity(id)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(removed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"photo":""`) {
		t.Fatalf("a removed photo crossed as %s", encoded)
	}
}
