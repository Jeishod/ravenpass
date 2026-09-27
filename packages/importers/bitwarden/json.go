package bitwarden

import (
	"encoding/json"
	"time"

	"github.com/dortanes/ravenpass/packages/importers"
)

// header is the top level of a JSON export; KDFMemory is in MiB.
type header struct {
	Encrypted         *bool  `json:"encrypted"`
	PasswordProtected bool   `json:"passwordProtected"`
	Salt              string `json:"salt"`
	KDFType           int    `json:"kdfType"`
	KDFIterations     int    `json:"kdfIterations"`
	KDFMemory         int    `json:"kdfMemory"`
	KDFParallelism    int    `json:"kdfParallelism"`
	Validation        string `json:"encKeyValidation_DO_NOT_EDIT"`
	Data              string `json:"data"`
}

// jsonExport is an unencrypted JSON export; an organization export also carries collections.
type jsonExport struct {
	Folders     []named `json:"folders"`
	Collections []named `json:"collections"`
	Items       []item  `json:"items"`
}

// named is a folder or a collection.
type named struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// openJSON owns text: the returned file holds it, or it is cleared.
func openJSON(text []byte) (*importers.ExportFile, error) {
	plain, locked, err := readJSON(text)
	switch {
	case err != nil:
		return nil, err
	case locked != nil:
		return importers.NewLockedExportFile(importers.FormatEncryptedJSON, locked, 0), nil
	}
	return importers.NewExportFile(importers.FormatJSON, plain, 0), nil
}

// readJSON owns text: it returns text as an unencrypted export, or clears it and returns the sealed export it describes.
func readJSON(text []byte) (jsonText, *sealed, error) {
	var head header
	if err := json.Unmarshal(text, &head); err != nil {
		clear(text)
		return nil, nil, importers.Unrecognized(err)
	}
	if head.Encrypted != nil && !*head.Encrypted {
		return jsonText(text), nil, nil
	}
	clear(text)
	switch {
	case head.Encrypted == nil:
		return nil, nil, importers.ErrUnrecognized
	case !head.PasswordProtected:
		return nil, nil, importers.ErrAccountBound
	}
	locked, err := sealedFrom(head)
	if err != nil {
		return nil, nil, err
	}
	return nil, locked, nil
}

type jsonText []byte

// Read maps the export's items, writing unmapped values under labels.
func (t jsonText) Read(labels importers.Labels) (importers.Export, error) {
	var export jsonExport
	if err := json.Unmarshal(t, &export); err != nil {
		return importers.Export{}, importers.Unrecognized(err)
	}
	folders, collections := namesByID(export.Folders), namesByID(export.Collections)
	mapped := mapper{labels: labels, now: time.Now()}
	for _, source := range export.Items {
		var filed []string
		if name := folders[source.FolderID]; !importers.Blank(name) {
			filed = append(filed, name)
		}
		for _, id := range source.CollectionIDs {
			if name := collections[id]; !importers.Blank(name) {
				filed = append(filed, name)
			}
		}
		mapped.add(source, filed)
	}
	return mapped.export, nil
}

// Wipe zeroes the export's bytes in place.
func (t jsonText) Wipe() {
	clear(t)
}

// namesByID skips the empty ID, which an item with no folder names.
func namesByID(list []named) map[string]string {
	names := make(map[string]string, len(list))
	for _, entry := range list {
		if entry.ID != "" {
			names[entry.ID] = entry.Name
		}
	}
	return names
}
