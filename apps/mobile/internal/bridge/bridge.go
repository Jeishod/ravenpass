// Package bridge reaches the Android services of com.dortanes.ravenpass.Bridge over JNI.
package bridge

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/photos"
	"github.com/dortanes/ravenpass/packages/app/storage"
)

var (
	// ErrRejected reports a missing Keystore key or one Android invalidated when the screen lock was removed.
	ErrRejected = errors.New("the Android Keystore does not accept this key")
	// ErrKeystore reports a Keystore operation that failed for any other reason.
	ErrKeystore = errors.New("the Android Keystore did not complete the operation")
	// ErrPickerUnavailable reports a file or photo picker that cannot be shown while the app is not in front.
	ErrPickerUnavailable = errors.New("the system picker cannot be shown now")
	// ErrNotWritable reports a document its provider does not let Ravenpass write.
	ErrNotWritable = fmt.Errorf("%w: the document provider does not allow writing this document", fs.ErrPermission)
	// ErrDocument reports a document operation that failed for any other reason.
	ErrDocument = errors.New("the document provider did not complete the operation")
	// ErrSettingsUnavailable reports a settings screen the device did not show.
	ErrSettingsUnavailable = errors.New("the settings screen cannot be shown now")
	// ErrNoticesUnreadable reports third-party notices the APK holds but Android did not read.
	ErrNoticesUnreadable = errors.New("the third-party notices could not be read")
	// ErrPrintUnavailable reports a print dialog the device did not show, or a page it did not load.
	ErrPrintUnavailable = errors.New("the print dialog cannot be shown now")
)

// printError treats a dialog the owner closed without printing as done.
func printError(s status) error {
	switch s {
	case statusOK, statusCanceled:
		return nil
	default:
		return ErrPrintUnavailable
	}
}

// noticesError reads statusGone as an APK built without the notices asset.
func noticesError(s status) error {
	switch s {
	case statusOK:
		return nil
	case statusGone:
		return fs.ErrNotExist
	default:
		return ErrNoticesUnreadable
	}
}

// systemAutofillStatus reads Bridge.passkeyProvider: 1 enabled, 0 not, -1 unknown, as before Android 14.
func systemAutofillStatus(autofill bool, passkeys int32) api.SystemAutofillStatus {
	return api.SystemAutofillStatus{Autofill: autofill, Passkeys: passkeys == 1, PasskeyProviders: passkeys >= 0}
}

// leftError treats a screen whose activity ended before the owner answered as left.
func leftError(s status) error {
	switch s {
	case statusOK, statusCanceled:
		return nil
	default:
		return ErrSettingsUnavailable
	}
}

// status values are shared with Bridge.java.
type status int32

const (
	statusOK          status = 0
	statusCanceled    status = 1
	statusFailed      status = 2
	statusUnavailable status = 3
	statusRejected    status = 4
	statusError       status = 5
	statusTooLarge    status = 6
	statusGone        status = 7
)

// keystoreError maps a status Bridge does not define, such as the -1 of an incomplete call, to ErrKeystore.
func keystoreError(s status) error {
	switch s {
	case statusOK:
		return nil
	case statusCanceled:
		return ownerauth.ErrCanceled
	case statusFailed:
		return ownerauth.ErrFailed
	case statusUnavailable:
		return ownerauth.ErrUnavailable
	case statusRejected:
		return ErrRejected
	default:
		return ErrKeystore
	}
}

func ownerError(s status) error {
	switch s {
	case statusOK:
		return nil
	case statusCanceled:
		return ownerauth.ErrCanceled
	case statusFailed:
		return ownerauth.ErrFailed
	default:
		return ownerauth.ErrUnavailable
	}
}

// documentError leaves statusCanceled to its caller: a canceled pick is not an error.
func documentError(s status) error {
	switch s {
	case statusOK:
		return nil
	case statusUnavailable:
		return ErrPickerUnavailable
	case statusRejected:
		return ErrNotWritable
	case statusTooLarge:
		return storage.ErrTooLarge
	case statusGone:
		return storage.ErrNotFound
	default:
		return ErrDocument
	}
}

// pickedDocument reads the payload "address NUL name NUL provider label", UTF-8.
func pickedDocument(payload []byte) (string, storage.Label, error) {
	fields := strings.SplitN(string(payload), "\x00", 3)
	if len(fields) != 3 || fields[0] == "" {
		return "", storage.Label{}, ErrDocument
	}
	return fields[0], storage.Label{Name: fields[1], Place: fields[2]}, nil
}

// photoError leaves statusCanceled to its caller: a canceled pick is not an error.
func photoError(s status) error {
	switch s {
	case statusOK:
		return nil
	case statusUnavailable:
		return ErrPickerUnavailable
	case statusTooLarge:
		return photos.ErrTooLarge
	default:
		return ErrDocument
	}
}

// pickedPhoto reads the payload "address NUL name", UTF-8.
func pickedPhoto(payload []byte) (string, string, error) {
	address, name, found := strings.Cut(string(payload), "\x00")
	if !found || address == "" || name == "" {
		return "", "", ErrDocument
	}
	return address, name, nil
}
