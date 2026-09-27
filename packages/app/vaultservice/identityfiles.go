package vaultservice

import (
	"slices"

	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	// PhotoFile names an identity's photo among its files. A scan is named by its ID.
	PhotoFile = "photo"
	// photoName is the name an identity's photo is shared under.
	photoName = "photo.jpg"
)

// FileKind tells an identity's photo from a document's scan.
type FileKind uint8

// Kinds of IdentityFile.
const (
	FilePhoto FileKind = iota + 1
	FileScan
)

// IdentityFile is an identity's photo or scan; its Thumbnail is a small JPEG, empty for a PDF.
type IdentityFile struct {
	ID        string
	Kind      FileKind
	Name      string
	MediaType string
	Document  *FileDocument
	Thumbnail []byte
}

// FileDocument is the document a scan belongs to.
type FileDocument struct {
	Type  vault.DocumentType
	Label string
}

// IdentityFiles is an identity with its photo first, then each document's scans in order.
type IdentityFiles struct {
	ID        vault.ID
	Label     string
	Thumbnail []byte
	Files     []IdentityFile
}

// FileContent is an identity file as it is shared.
type FileContent struct {
	Name      string
	MediaType string
	Content   []byte
}

// IdentityFiles lists every identity with the files it holds.
func (s *Service) IdentityFiles() ([]IdentityFiles, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, ErrNotReady
	}
	held, err := s.session.IdentityFiles()
	if err != nil {
		return nil, err
	}
	identities := make([]IdentityFiles, len(held))
	for i, identity := range held {
		listed := IdentityFiles{ID: identity.Identity, Label: identity.Label, Thumbnail: identity.Thumbnail}
		if len(identity.Thumbnail) > 0 {
			listed.Files = append(listed.Files, IdentityFile{ID: PhotoFile, Kind: FilePhoto, Name: photoName, MediaType: vault.MediaJPEG, Thumbnail: identity.Thumbnail})
		}
		for _, document := range identity.Documents {
			for _, scan := range document.Scans {
				listed.Files = append(listed.Files, IdentityFile{
					ID: scan.ID.String(), Kind: FileScan, Name: scan.Name, MediaType: scan.MediaType,
					Document: &FileDocument{Type: document.Type, Label: document.Label}, Thumbnail: scan.Thumbnail,
				})
			}
		}
		identities[i] = listed
	}
	return identities, nil
}

// ReadIdentityFile reads one of an identity's files, or fails with vault.ErrNotFound.
func (s *Service) ReadIdentityFile(identity vault.ID, file string) (FileContent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return FileContent{}, ErrNotReady
	}
	if file == PhotoFile {
		photo, err := s.session.ReadIdentityPhoto(identity)
		if err != nil {
			return FileContent{}, err
		}
		return FileContent{Name: photoName, MediaType: vault.MediaJPEG, Content: photo}, nil
	}
	id, err := vault.ParseID(file)
	if err != nil {
		return FileContent{}, vault.ErrNotFound
	}
	scans, err := s.session.ScansOf(identity)
	if err != nil {
		return FileContent{}, err
	}
	if !slices.ContainsFunc(scans, func(scan vault.ScanSummary) bool { return scan.ID == id }) {
		return FileContent{}, vault.ErrNotFound
	}
	scan, err := s.session.ReadScan(id)
	if err != nil {
		return FileContent{}, err
	}
	return FileContent{Name: scan.Name, MediaType: scan.MediaType, Content: scan.Content}, nil
}
