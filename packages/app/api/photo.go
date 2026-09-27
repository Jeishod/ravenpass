package api

import (
	"encoding/base64"
	"io"
	"os"
	"sync"

	"github.com/dortanes/ravenpass/packages/app/photos"
)

const photoExtensions = "*.jpg;*.jpeg;*.png;*.webp"

// PhotoDraft is the chosen picture's preview; Chosen is false for a canceled dialog.
type PhotoDraft struct {
	Chosen bool `json:"chosen"`
	// Preview is a base64 JPEG of Width by Height pixels.
	Preview string `json:"preview"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

// photoSlot holds the one decoded picture between its preview and its crop.
type photoSlot struct {
	mu      sync.Mutex
	picture *photos.Picture
}

func (s *photoSlot) put(picture *photos.Picture) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.picture = picture
}

func (s *photoSlot) clear() {
	s.put(nil)
}

// ChooseIdentityPhoto picks and decodes a picture, replacing the kept one until it is cropped or discarded.
func (s *Service) ChooseIdentityPhoto() (PhotoDraft, error) {
	s.photo.clear()
	photo, picked, err := s.photoPicker.Pick(photos.MaxFileBytes)
	if err != nil {
		return PhotoDraft{}, present(err)
	}
	if !picked {
		return PhotoDraft{}, nil
	}
	return s.stagePhoto(photo.Data)
}

// readChosenFile reads a chosen file, failing with photos.ErrTooLarge past limit bytes.
func readChosenFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		clear(data)
		return nil, photos.ErrTooLarge
	}
	return data, nil
}

func (s *Service) stagePhoto(data []byte) (PhotoDraft, error) {
	picture, err := photos.Open(data)
	if err != nil {
		return PhotoDraft{}, present(err)
	}
	preview, size, err := picture.Preview()
	if err != nil {
		return PhotoDraft{}, present(err)
	}
	s.photo.put(picture)
	return PhotoDraft{Chosen: true, Preview: base64.StdEncoding.EncodeToString(preview), Width: size.X, Height: size.Y}, nil
}

// CropIdentityPhoto returns the square at x, y of side size, in preview pixels, as a base64 PNG and empties the slot.
func (s *Service) CropIdentityPhoto(x int, y int, size int) (string, error) {
	s.photo.mu.Lock()
	defer s.photo.mu.Unlock()
	if s.photo.picture == nil {
		return "", fail(failureFileNotSelected)
	}
	square, err := s.photo.picture.Crop(x, y, size)
	if err != nil {
		return "", present(err)
	}
	s.photo.picture = nil
	return base64.StdEncoding.EncodeToString(square), nil
}

// DiscardIdentityPhoto forgets the kept picture.
func (s *Service) DiscardIdentityPhoto() error {
	s.photo.clear()
	return nil
}
