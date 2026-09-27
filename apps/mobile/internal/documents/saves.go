package documents

import (
	"bytes"
	"fmt"
	"log/slog"

	"github.com/dortanes/ravenpass/packages/app/api"
)

var _ api.FileSaver = Saves{}

// Saves writes each file to a new document the owner creates with the system file picker.
type Saves struct {
	Provider Provider
}

// Choose asks where file goes, proposing its name.
func (saves Saves) Choose(file api.SavedFile) (api.SaveTarget, bool, error) {
	address, created, err := saves.Provider.Create(file.Name, file.MediaType)
	if err != nil || !created {
		return nil, false, err
	}
	return savedDocument{provider: saves.Provider, address: address}, true, nil
}

type savedDocument struct {
	provider Provider
	address  string
}

// Write writes data and reads it back.
func (d savedDocument) Write(data []byte) error {
	if err := d.provider.Write(d.address, data); err != nil {
		return fmt.Errorf("write saved document: %w", err)
	}
	written, err := d.provider.Read(d.address, int64(len(data)))
	defer clear(written)
	if err != nil {
		return fmt.Errorf("%w: %w", api.ErrFileUnverified, err)
	}
	if !bytes.Equal(written, data) {
		return fmt.Errorf("%w: the document reads back other content", api.ErrFileUnverified)
	}
	return nil
}

// Discard deletes the document with whatever a failed write left in it.
func (d savedDocument) Discard() {
	if err := d.provider.Delete(d.address); err != nil {
		slog.Warn("deleting a document after a failed save failed", "err", err)
	}
}
