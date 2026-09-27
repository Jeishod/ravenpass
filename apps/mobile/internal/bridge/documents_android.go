//go:build android

package bridge

import (
	"github.com/dortanes/ravenpass/apps/mobile/internal/documents"
	"github.com/dortanes/ravenpass/packages/app/storage"
)

var _ documents.Provider = Documents{}

// Documents reaches the documents and folders the owner picks with the system file picker, across restarts.
type Documents struct{}

// Pick fails with ErrNotWritable for a document its provider does not let Ravenpass write.
func (Documents) Pick(create bool, name string) (string, storage.Label, bool, error) {
	return picked(pickDocument(create, name))
}

// Create shows the picker that creates a document; access to it lasts until the device restarts.
func (Documents) Create(name, mediaType string) (string, bool, error) {
	s, payload := createDocument(name, mediaType)
	if s == statusCanceled {
		return "", false, nil
	}
	if err := documentError(s); err != nil {
		return "", false, err
	}
	if len(payload) == 0 {
		return "", false, ErrDocument
	}
	return string(payload), true, nil
}

// PickFolder fails with ErrNotWritable for a folder its provider does not let Ravenpass create documents in.
func (Documents) PickFolder() (string, storage.Label, bool, error) {
	return picked(pickFolder())
}

// CreateIn fails with ErrNotWritable once Ravenpass no longer has access to the folder.
func (Documents) CreateIn(folder, name, mediaType string) (string, error) {
	s, payload := createInFolder(folder, name, mediaType)
	if err := documentError(s); err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", ErrDocument
	}
	return string(payload), nil
}

// Read fails with storage.ErrTooLarge for a document over limit bytes and storage.ErrNotFound for one its provider no
// longer has.
func (Documents) Read(address string, limit int64) ([]byte, error) {
	s, content := readDocument(address, limit)
	if err := documentError(s); err != nil {
		return nil, err
	}
	return content, nil
}

// Write truncates the document and writes data; a cloud provider uploads it once it is closed.
func (Documents) Write(address string, data []byte) error {
	return documentError(writeDocument(address, data))
}

// Delete removes the document and gives up the access Ravenpass kept to it.
func (Documents) Delete(address string) error {
	return documentError(deleteDocument(address))
}

func picked(s status, payload []byte) (string, storage.Label, bool, error) {
	if s == statusCanceled {
		return "", storage.Label{}, false, nil
	}
	if err := documentError(s); err != nil {
		return "", storage.Label{}, false, err
	}
	address, label, err := pickedDocument(payload)
	if err != nil {
		return "", storage.Label{}, false, err
	}
	return address, label, true, nil
}
