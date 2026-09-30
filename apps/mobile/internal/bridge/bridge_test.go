package bridge

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/photos"
	"github.com/dortanes/ravenpass/packages/app/storage"
)

// incomplete is the JNI glue's status for a call that threw or never reached Bridge.
const incomplete status = -1

func TestAPromptWithoutATitleIsRefused(t *testing.T) {
	for _, reason := range []string{"", " ", "\n\t"} {
		if err := titled(reason); !errors.Is(err, ErrUntitledPrompt) {
			t.Errorf("%q: got %v, want ErrUntitledPrompt", reason, err)
		}
	}
	if err := titled("Unlock Ravenpass"); err != nil {
		t.Fatalf("a titled prompt: %v", err)
	}
}

func TestKeystoreStatusesBecomeTheErrorsTheBindingsExpect(t *testing.T) {
	for _, test := range []struct {
		status status
		want   error
	}{
		{statusCanceled, ownerauth.ErrCanceled},
		{statusFailed, ownerauth.ErrFailed},
		{statusUnavailable, ownerauth.ErrUnavailable},
		{statusRejected, ErrRejected},
		{statusError, ErrKeystore},
		{incomplete, ErrKeystore},
		{status(99), ErrKeystore},
	} {
		if err := keystoreError(test.status); !errors.Is(err, test.want) {
			t.Errorf("status %d: got %v, want %v", test.status, err, test.want)
		}
	}
	if err := keystoreError(statusOK); err != nil {
		t.Fatalf("a Keystore call that succeeded failed with %v", err)
	}
}

func TestAnOwnerVerificationEndsWithTheOwnersAnswerOrAsUnavailable(t *testing.T) {
	for _, test := range []struct {
		status status
		want   error
	}{
		{statusCanceled, ownerauth.ErrCanceled},
		{statusFailed, ownerauth.ErrFailed},
		{statusUnavailable, ownerauth.ErrUnavailable},
		{statusRejected, ownerauth.ErrUnavailable},
		{statusError, ownerauth.ErrUnavailable},
		{incomplete, ownerauth.ErrUnavailable},
	} {
		if err := ownerError(test.status); !errors.Is(err, test.want) {
			t.Errorf("status %d: got %v, want %v", test.status, err, test.want)
		}
	}
	if err := ownerError(statusOK); err != nil {
		t.Fatalf("a verified owner failed with %v", err)
	}
}

func TestDocumentStatusesBecomeTheErrorsTheStoreExpects(t *testing.T) {
	for _, test := range []struct {
		status status
		want   error
	}{
		{statusUnavailable, ErrPickerUnavailable},
		{statusRejected, ErrNotWritable},
		{statusTooLarge, storage.ErrTooLarge},
		{statusGone, storage.ErrNotFound},
		{statusFailed, ErrDocument},
		{statusError, ErrDocument},
		{incomplete, ErrDocument},
	} {
		if err := documentError(test.status); !errors.Is(err, test.want) {
			t.Errorf("status %d: got %v, want %v", test.status, err, test.want)
		}
	}
	if err := documentError(statusOK); err != nil {
		t.Fatalf("a document call that succeeded failed with %v", err)
	}
}

func TestTheAutofillScreenFailsOnlyWhenItDidNotShow(t *testing.T) {
	for _, test := range []struct {
		status status
		err    error
	}{
		{statusOK, nil},
		{statusCanceled, nil},
		{statusUnavailable, ErrSettingsUnavailable},
		{statusError, ErrSettingsUnavailable},
		{incomplete, ErrSettingsUnavailable},
	} {
		if err := leftError(test.status); !errors.Is(err, test.err) {
			t.Errorf("status %d: got %v, want %v", test.status, err, test.err)
		}
	}
}

func TestPrintingFailsOnlyWhenTheDialogDidNotShow(t *testing.T) {
	for _, test := range []struct {
		status status
		err    error
	}{
		{statusOK, nil},
		{statusCanceled, nil},
		{statusUnavailable, ErrPrintUnavailable},
		{statusError, ErrPrintUnavailable},
		{incomplete, ErrPrintUnavailable},
	} {
		if err := printError(test.status); !errors.Is(err, test.err) {
			t.Errorf("status %d: got %v, want %v", test.status, err, test.err)
		}
	}
}

func TestNoticesAreMissingOnlyWhereTheAPKHasNone(t *testing.T) {
	for _, test := range []struct {
		status status
		err    error
	}{
		{statusOK, nil},
		{statusGone, fs.ErrNotExist},
		{statusError, ErrNoticesUnreadable},
		{incomplete, ErrNoticesUnreadable},
	} {
		if err := noticesError(test.status); !errors.Is(err, test.err) {
			t.Errorf("status %d: got %v, want %v", test.status, err, test.err)
		}
	}
}

func TestThePasskeyProviderStateIsKnownOnlyWhereThePhoneSaysSo(t *testing.T) {
	for _, test := range []struct {
		autofill bool
		passkeys int32
		want     api.SystemAutofillStatus
	}{
		{true, 1, api.SystemAutofillStatus{Autofill: true, Passkeys: true, PasskeyProviders: true}},
		{false, 0, api.SystemAutofillStatus{PasskeyProviders: true}},
		{true, -1, api.SystemAutofillStatus{Autofill: true}},
		{false, 2, api.SystemAutofillStatus{PasskeyProviders: true}},
	} {
		if got := systemAutofillStatus(test.autofill, test.passkeys); got != test.want {
			t.Errorf("autofill %v, passkeys %d: got %+v, want %+v", test.autofill, test.passkeys, got, test.want)
		}
	}
}

func TestAPickNamesTheDocumentAndItsProvider(t *testing.T) {
	address, label, err := pickedDocument([]byte("content://drive/document/1\x00Family.rpv\x00Drive"))
	if err != nil || address != "content://drive/document/1" || label != (storage.Label{Name: "Family.rpv", Place: "Drive"}) {
		t.Fatalf("a picked document: %q, %+v, %v", address, label, err)
	}
	for _, payload := range []string{"", "content://drive/document/1", "\x00Family.rpv\x00Drive"} {
		if _, _, err := pickedDocument([]byte(payload)); !errors.Is(err, ErrDocument) {
			t.Fatalf("payload %q: got %v, want ErrDocument", payload, err)
		}
	}
}

func TestPhotoStatusesBecomeTheErrorsThePhotoRulesExpect(t *testing.T) {
	for _, test := range []struct {
		status status
		want   error
	}{
		{statusUnavailable, ErrPickerUnavailable},
		{statusTooLarge, photos.ErrTooLarge},
		{statusRejected, ErrDocument},
		{statusGone, ErrDocument},
		{statusError, ErrDocument},
		{incomplete, ErrDocument},
	} {
		if err := photoError(test.status); !errors.Is(err, test.want) {
			t.Errorf("status %d: got %v, want %v", test.status, err, test.want)
		}
	}
	if err := photoError(statusOK); err != nil {
		t.Fatalf("a photo call that succeeded failed with %v", err)
	}
}

func TestAPhotoPickNamesThePhoto(t *testing.T) {
	address, name, err := pickedPhoto([]byte("content://media/picker/0/photo/1000000042\x00PXL_20260912.jpg"))
	if err != nil || address != "content://media/picker/0/photo/1000000042" || name != "PXL_20260912.jpg" {
		t.Fatalf("a picked photo: %q, %q, %v", address, name, err)
	}
	for _, payload := range []string{"", "content://media/picker/0/photo/1", "content://media/picker/0/photo/1\x00", "\x00PXL_20260912.jpg"} {
		if _, _, err := pickedPhoto([]byte(payload)); !errors.Is(err, ErrDocument) {
			t.Fatalf("payload %q: got %v, want ErrDocument", payload, err)
		}
	}
}
