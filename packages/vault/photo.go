package vault

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
)

// PhotoSize is the side of the square a photo is given and stored at, in pixels.
const PhotoSize = 512

// A stored picture is a metadata-free JPEG the core encoded, its quality stepped down from first to last until it fits its budget.
const (
	storedFirstQuality    = 85
	storedLastQuality     = 45
	thumbnailFirstQuality = 80
	thumbnailLastQuality  = 40
	qualityStep           = 10
	maxPhotoBytes         = 256 << 10
	maxThumbnailBytes     = 8 << 10
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// pictureRule is how the core takes one kind of picture.
type pictureRule struct {
	jpegGiven       bool
	givenBytes      int
	accepts         func(width, height int) bool
	budget          int
	thumbnailSide   int
	thumbnailBudget int
}

var photoRule = pictureRule{
	jpegGiven:       true,
	givenBytes:      4 << 20,
	accepts:         func(width, height int) bool { return width == PhotoSize && height == PhotoSize },
	budget:          maxPhotoBytes,
	thumbnailSide:   96,
	thumbnailBudget: maxThumbnailBytes,
}

// FitWithin reports the size a picture takes scaled in proportion to a long side of at most side.
func FitWithin(size image.Point, side int) image.Point {
	long := max(size.X, size.Y)
	if long <= side {
		return size
	}
	return image.Pt(max(1, size.X*side/long), max(1, size.Y*side/long))
}

// encode turns a given picture into the stored JPEG and its thumbnail.
func (r pictureRule) encode(given []byte) ([]byte, []byte, error) {
	picture, err := r.decode(given)
	if err != nil {
		return nil, nil, err
	}
	stored, err := encodeWithin(picture, storedFirstQuality, storedLastQuality, r.budget)
	if err != nil {
		return nil, nil, err
	}
	small := image.NewRGBA(image.Rectangle{Max: FitWithin(picture.Bounds().Size(), r.thumbnailSide)})
	draw.CatmullRom.Scale(small, small.Bounds(), picture, picture.Bounds(), draw.Src, nil)
	thumbnail, err := encodeWithin(small, thumbnailFirstQuality, thumbnailLastQuality, r.thumbnailBudget)
	if err != nil {
		return nil, nil, err
	}
	return stored, thumbnail, nil
}

// decode reads a PNG, or a JPEG where the rule takes one, by signature only, flattened onto white.
func (r pictureRule) decode(given []byte) (*image.RGBA, error) {
	if len(given) > r.givenBytes {
		return nil, ErrInvalidInput
	}
	decodeConfig, decode := png.DecodeConfig, png.Decode
	switch {
	case bytes.HasPrefix(given, pngSignature):
	case r.jpegGiven && bytes.HasPrefix(given, []byte{0xff, 0xd8}):
		decodeConfig, decode = jpeg.DecodeConfig, jpeg.Decode
	default:
		return nil, ErrInvalidInput
	}
	config, err := decodeConfig(bytes.NewReader(given))
	if err != nil || config.Width <= 0 || config.Height <= 0 || !r.accepts(config.Width, config.Height) {
		return nil, ErrInvalidInput
	}
	decoded, err := decode(bytes.NewReader(given))
	if err != nil {
		return nil, ErrInvalidInput
	}
	canvas := image.NewRGBA(image.Rect(0, 0, config.Width, config.Height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), decoded, decoded.Bounds().Min, draw.Over)
	return canvas, nil
}

// acceptPhoto returns the stored photo and thumbnail: none for empty, the stored pair for unchanged bytes, else a new encoding.
func acceptPhoto(given, stored, storedThumbnail []byte) ([]byte, []byte, error) {
	if len(given) == 0 {
		return nil, nil, nil
	}
	if len(stored) > 0 && bytes.Equal(given, stored) {
		return stored, storedThumbnail, nil
	}
	return photoRule.encode(given)
}

func encodeWithin(picture image.Image, first, last, budget int) ([]byte, error) {
	for quality := first; quality >= last; quality -= qualityStep {
		var encoded bytes.Buffer
		if err := jpeg.Encode(&encoded, picture, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		if encoded.Len() <= budget {
			return encoded.Bytes(), nil
		}
	}
	return nil, ErrResourceLimit
}
