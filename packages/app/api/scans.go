package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/photos"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/vault"
)

const (
	scanExtensions = photoExtensions + ";*.pdf"
	// newScanPrefix marks a staged scan in a document's scans, followed by its token.
	newScanPrefix = "new:"
	// maxStagedScans bounds what one editor session holds in memory: 16 documents of 4 scans.
	maxStagedScans = 64
)

var errStagingFull = errors.New("the editor holds as many new scans as it can")

// ScanSummary describes one scan without its content.
type ScanSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// MediaType is image/jpeg or application/pdf.
	MediaType string `json:"mediaType"`
	// Thumbnail is a base64 JPEG for a picture and empty for a PDF.
	Thumbnail string `json:"thumbnail"`
}

// ScanDraft is a chosen scan staged for the editor's save; Chosen is false for a canceled dialog.
type ScanDraft struct {
	Chosen bool `json:"chosen"`
	// Token names the draft in a document's scans as "new:" followed by Token.
	Token     string `json:"token"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Thumbnail string `json:"thumbnail"`
}

// ScanSaved reports whether a copy was written.
type ScanSaved struct {
	Saved bool `json:"saved"`
}

func scanSummary(scan vault.ScanSummary) ScanSummary {
	return ScanSummary{ID: scan.ID.String(), Name: scan.Name, MediaType: scan.MediaType, Thumbnail: base64.StdEncoding.EncodeToString(scan.Thumbnail)}
}

// scanStaging holds editor-chosen scans by one-time token until a save, cancel or lock; a refused save keeps them.
type scanStaging struct {
	mu    sync.Mutex
	scans map[string]vault.PreparedScan
}

func (s *scanStaging) stage(scan vault.PreparedScan) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.scans) >= maxStagedScans {
		return "", errStagingFull
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	if s.scans == nil {
		s.scans = make(map[string]vault.PreparedScan)
	}
	key := hex.EncodeToString(token)
	s.scans[key] = scan
	return key, nil
}

func (s *scanStaging) find(token string) (vault.PreparedScan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan, staged := s.scans[token]
	return scan, staged
}

// clear drops the staged scans without overwriting them: a concurrent save may still be sealing them.
func (s *scanStaging) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scans = nil
}

// resolveScans splits references into kept ids and staged scans; an unknown or repeated token is refused.
func (s *Service) resolveScans(references []string, used map[string]struct{}) ([]vault.ID, []vault.PreparedScan, error) {
	var stored []vault.ID
	var attached []vault.PreparedScan
	for _, reference := range references {
		token, isNew := strings.CutPrefix(reference, newScanPrefix)
		if !isNew {
			id, err := vault.ParseID(reference)
			if err != nil {
				return nil, nil, fail(failureInvalidItem)
			}
			stored = append(stored, id)
			continue
		}
		scan, staged := s.scans.find(token)
		if _, repeated := used[token]; !staged || repeated {
			return nil, nil, fail(failureFileNotSelected)
		}
		used[token] = struct{}{}
		attached = append(attached, scan)
	}
	return stored, attached, nil
}

// presentScan names failures of a chosen scan file and presents the rest as present does.
func presentScan(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errStagingFull):
		return fail(failureScanLimit)
	case errors.Is(err, photos.ErrUnsupported), errors.Is(err, vault.ErrInvalidInput):
		return fail(failureScanUnsupported)
	case errors.Is(err, photos.ErrTooLarge), errors.Is(err, vault.ErrResourceLimit), errors.Is(err, storage.ErrTooLarge):
		return fail(failureScanTooLarge)
	default:
		return present(err)
	}
}

// ChooseScan picks a picture or PDF and stages it for the editor's save; its content never reaches the interface.
func (s *Service) ChooseScan() (ScanDraft, error) {
	dialogs := s.preferences.Dialogs()
	picked, err := s.dialog.pick(dialogs.ChooseScan, dialogs.ScanFilter, scanExtensions)
	if err != nil || picked.path == "" {
		return ScanDraft{}, err
	}
	data, err := readChosenFile(picked.path, photos.MaxFileBytes)
	picked.discard()
	if err != nil {
		return ScanDraft{}, presentScan(err)
	}
	defer clear(data)
	return s.stageScan(filepath.Base(picked.path), data)
}

// ChooseScanPhoto picks a picture with the host's photo picker and stages it as ChooseScan does.
func (s *Service) ChooseScanPhoto() (ScanDraft, error) {
	photo, picked, err := s.photoPicker.Pick(photos.MaxFileBytes)
	if err != nil {
		return ScanDraft{}, presentScan(err)
	}
	if !picked {
		return ScanDraft{}, nil
	}
	defer clear(photo.Data)
	return s.stageScan(photo.Name, photo.Data)
}

// stageScan stages a PDF as it is and anything else as a PNG from the picture pipeline.
func (s *Service) stageScan(name string, data []byte) (ScanDraft, error) {
	given := data
	if !vault.HasPDFSignature(data) {
		picture, err := photos.Open(data)
		if err != nil {
			return ScanDraft{}, presentScan(err)
		}
		if given, err = picture.Scan(); err != nil {
			return ScanDraft{}, presentScan(err)
		}
	}
	scan, err := vault.PrepareScan(name, given)
	if err != nil {
		return ScanDraft{}, presentScan(err)
	}
	token, err := s.scans.stage(scan)
	if err != nil {
		return ScanDraft{}, presentScan(err)
	}
	return ScanDraft{
		Chosen: true, Token: token, Name: scan.Name(), MediaType: scan.MediaType(),
		Thumbnail: base64.StdEncoding.EncodeToString(scan.Thumbnail()),
	}, nil
}

// DiscardScans forgets every scan chosen since the editor opened.
func (s *Service) DiscardScans() error {
	s.scans.clear()
	return nil
}

func (s *Service) readScan(id string) (vault.Scan, error) {
	parsed, err := vault.ParseID(id)
	if err != nil {
		return vault.Scan{}, fail(failureItemUnreadable)
	}
	scan, err := s.vault.ReadScan(parsed)
	if err != nil {
		return vault.Scan{}, present(err)
	}
	return scan, nil
}

// CopyScan copies a scan to the clipboard, hidden from history and cleared on the text delay.
func (s *Service) CopyScan(id string) error {
	return s.copyScan(id, clearAfter)
}

func (s *Service) copyScan(id string, schedule func(time.Duration, func())) error {
	scan, err := s.readScan(id)
	if err != nil {
		return err
	}
	defer clear(scan.Content)
	if err := s.copyScanToPasteboard(scan.Content, scan.MediaType, schedule); err != nil {
		return fail(failureCopyFailed)
	}
	return nil
}

// SaveScan writes an unencrypted copy of a scan where the person chooses.
func (s *Service) SaveScan(id string) (ScanSaved, error) {
	scan, err := s.readScan(id)
	if err != nil {
		return ScanSaved{}, err
	}
	defer clear(scan.Content)
	file := SavedFile{Prompt: s.preferences.Dialogs().SaveScan, Name: scan.Name, MediaType: scan.MediaType}
	saved, err := s.saveFile(file, func(target SaveTarget) error { return target.Write(scan.Content) })
	return ScanSaved{Saved: saved}, err
}
