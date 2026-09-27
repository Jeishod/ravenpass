// Package photos decodes a JPEG, PNG or WebP picture upright in pure Go and cuts photos and scans from it.
package photos

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	"github.com/dortanes/ravenpass/packages/vault"
	"github.com/xgfone/go-imagex/orientation"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	// MaxFileBytes bounds a picture file.
	MaxFileBytes = 32 << 20
	// maxPixels bounds a picture's declared pixel count, checked before decoding.
	maxPixels = 50_000_000
	// previewSide bounds the long side of the preview a square is picked on.
	previewSide    = 1600
	previewQuality = 85
)

var (
	// ErrUnsupported reports a file that is not a supported picture or does not decode.
	ErrUnsupported = errors.New("picture is not a supported format or does not decode")
	// ErrTooLarge reports a file or picture over the bounds.
	ErrTooLarge = errors.New("picture is too large")
	// ErrOutside reports a square that does not lie inside the preview.
	ErrOutside = errors.New("square does not lie inside the preview")
)

// decoders are keyed by the format name image.DecodeConfig reports; each returns the picture upright.
var decoders = map[string]func([]byte) (image.Image, error){
	"jpeg": decodeJPEG,
	"png":  func(data []byte) (image.Image, error) { return png.Decode(bytes.NewReader(data)) },
	"webp": func(data []byte) (image.Image, error) { return webp.Decode(bytes.NewReader(data)) },
}

func decodeJPEG(data []byte) (image.Image, error) {
	picture, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return orientation.ApplyOrientation(picture, orientation.ExtractOrientationFromJPEG(data)), nil
}

// Picture is a decoded picture turned upright, with the size of its preview.
type Picture struct {
	upright image.Image
	preview image.Point
}

// Open recognises the format by content and refuses a picture over the bounds before decoding any pixel.
func Open(data []byte) (*Picture, error) {
	if len(data) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	decode, supported := decoders[format]
	if !supported || config.Width <= 0 || config.Height <= 0 {
		return nil, ErrUnsupported
	}
	if int64(config.Width)*int64(config.Height) > maxPixels {
		return nil, ErrTooLarge
	}
	upright, err := decode(data)
	if err != nil || upright.Bounds().Empty() {
		return nil, ErrUnsupported
	}
	return &Picture{upright: upright, preview: vault.FitWithin(upright.Bounds().Size(), previewSide)}, nil
}

// Scan reports the whole picture as a PNG at most vault.ScanSide on its long side.
func (p *Picture) Scan() ([]byte, error) {
	scaled := image.NewNRGBA(image.Rectangle{Max: vault.FitWithin(p.upright.Bounds().Size(), vault.ScanSide)})
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), p.upright, p.upright.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, scaled); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// Preview reports the picture as a JPEG of the preview size, laid on white, and that size.
func (p *Picture) Preview() ([]byte, image.Point, error) {
	canvas := image.NewRGBA(image.Rectangle{Max: p.preview})
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(canvas, canvas.Bounds(), p.upright, p.upright.Bounds(), draw.Over, nil)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, canvas, &jpeg.Options{Quality: previewQuality}); err != nil {
		return nil, image.Point{}, err
	}
	return encoded.Bytes(), p.preview, nil
}

// Crop cuts the square at x, y of side size, in preview pixels, from the full picture as a vault.PhotoSize PNG.
func (p *Picture) Crop(x, y, size int) ([]byte, error) {
	if size <= 0 || x < 0 || y < 0 || x > p.preview.X-size || y > p.preview.Y-size {
		return nil, ErrOutside
	}
	bounds := p.upright.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	source := image.Rect(
		x*width/p.preview.X, y*height/p.preview.Y,
		(x+size)*width/p.preview.X, (y+size)*height/p.preview.Y,
	).Add(bounds.Min)
	square := image.NewNRGBA(image.Rect(0, 0, vault.PhotoSize, vault.PhotoSize))
	draw.CatmullRom.Scale(square, square.Bounds(), p.upright, source, draw.Src, nil)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, square); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
