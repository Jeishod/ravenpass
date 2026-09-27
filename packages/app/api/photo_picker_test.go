package api

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/photos"
	"github.com/dortanes/ravenpass/packages/vault"
)

// fakePhotoPicker answers with its photo, a cancel or err, recording each pick's limit.
type fakePhotoPicker struct {
	photo  PickedPhoto
	picked bool
	err    error
	limits []int64
}

func (f *fakePhotoPicker) Pick(limit int64) (PickedPhoto, bool, error) {
	f.limits = append(f.limits, limit)
	if f.err != nil || !f.picked {
		return PickedPhoto{}, false, f.err
	}
	return PickedPhoto{Name: f.photo.Name, Data: bytes.Clone(f.photo.Data)}, true, nil
}

func pickerOf(t *testing.T, name string) *fakePhotoPicker {
	t.Helper()
	return &fakePhotoPicker{photo: PickedPhoto{Name: name, Data: photoFixture(t, name)}, picked: true}
}

func TestAHostPhotoPickerIsOffered(t *testing.T) {
	if newServiceOnHost(t, Host{}).Capabilities().PhotoPicker {
		t.Fatal("a host that picks photos with the file dialog offers a photo picker")
	}
	offers := newServiceOnHost(t, Host{Photos: &fakePhotoPicker{}, Offers: Capabilities{SaveFiles: true}}).Capabilities()
	if offers != (Capabilities{SaveFiles: true, PhotoPicker: true}) {
		t.Fatalf("capabilities = %+v, want the host's offers and a photo picker", offers)
	}
}

func TestTheDesktopPicksPhotosWithItsFileDialog(t *testing.T) {
	service := newReadyService(t)
	_, err := service.ChooseIdentityPhoto()
	assertFailure(t, err, failureWindowUnavailable)
	_, err = service.ChooseScanPhoto()
	assertFailure(t, err, failureWindowUnavailable)
}

func TestAPickedPhotoBecomesTheIdentityPhoto(t *testing.T) {
	service := newReadyService(t)
	picker := pickerOf(t, "orientation-6.jpg")
	service.photoPicker = picker
	draft, err := service.ChooseIdentityPhoto()
	if err != nil || !draft.Chosen || draft.Width != 48 || draft.Height != 32 {
		t.Fatalf("draft = %+v, error = %v", draft, err)
	}
	if _, err := service.CropIdentityPhoto(16, 0, 32); err != nil {
		t.Fatalf("cropping a picked photo: %v", err)
	}
	if !slices.Equal(picker.limits, []int64{photos.MaxFileBytes}) {
		t.Fatalf("picks were bounded at %v, want the picture file bound", picker.limits)
	}
}

func TestAPickedPhotoIsStagedAsAScan(t *testing.T) {
	service := newReadyService(t)
	picker := pickerOf(t, "upright.webp")
	service.photoPicker = picker
	draft, err := service.ChooseScanPhoto()
	if err != nil || !draft.Chosen || draft.Name != "upright.webp" || draft.MediaType != vault.MediaJPEG || draft.Thumbnail == "" {
		t.Fatalf("draft = %+v, error = %v", draft, err)
	}
	if _, staged := service.scans.find(draft.Token); !staged {
		t.Fatal("a picked photo was not staged")
	}
	if !slices.Equal(picker.limits, []int64{photos.MaxFileBytes}) {
		t.Fatalf("picks were bounded at %v, want the picture file bound", picker.limits)
	}
}

func TestACanceledPickChoosesNothing(t *testing.T) {
	service := newReadyService(t)
	if _, err := service.stagePhoto(photoFixture(t, "upright.png")); err != nil {
		t.Fatal(err)
	}
	service.photoPicker = &fakePhotoPicker{}
	draft, err := service.ChooseIdentityPhoto()
	if err != nil || draft != (PhotoDraft{}) {
		t.Fatalf("a canceled photo pick = %+v, error = %v", draft, err)
	}
	_, err = service.CropIdentityPhoto(0, 0, 32)
	assertFailure(t, err, failureFileNotSelected)
	scan, err := service.ChooseScanPhoto()
	if err != nil || scan != (ScanDraft{}) {
		t.Fatalf("a canceled scan pick = %+v, error = %v", scan, err)
	}
	if len(service.scans.scans) != 0 {
		t.Fatal("a canceled scan pick staged a scan")
	}
}

func TestAPickedPhotoMeetsTheRulesOfAChosenFile(t *testing.T) {
	for _, test := range []struct {
		name   string
		picker *fakePhotoPicker
		photo  failure
		scan   failure
	}{
		{"not a picture", &fakePhotoPicker{photo: PickedPhoto{Name: "note.txt", Data: []byte("not a picture")}, picked: true}, failurePhotoUnsupported, failureScanUnsupported},
		{"refused over the bound", &fakePhotoPicker{err: photos.ErrTooLarge}, failurePhotoTooLarge, failureScanTooLarge},
		{"handed over whole past the bound", &fakePhotoPicker{photo: PickedPhoto{Name: "large.png", Data: make([]byte, photos.MaxFileBytes+1)}, picked: true}, failurePhotoTooLarge, failureScanTooLarge},
		{"picker failed", &fakePhotoPicker{err: errors.New("the picker did not answer")}, failureGeneral, failureGeneral},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := newReadyService(t)
			service.photoPicker = test.picker
			_, err := service.ChooseIdentityPhoto()
			assertFailure(t, err, test.photo)
			_, err = service.ChooseScanPhoto()
			assertFailure(t, err, test.scan)
			if len(service.scans.scans) != 0 {
				t.Fatal("a refused photo was staged as a scan")
			}
		})
	}
}
