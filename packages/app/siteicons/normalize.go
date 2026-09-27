package siteicons

import (
	"bytes"
	"errors"
	"image"
	"image/png"

	"github.com/sergeymakinen/go-ico"
	"golang.org/x/image/draw"
)

// IconSize is the side of the square every stored icon is scaled to fit.
const IconSize = 64

var errEmptyImage = errors.New("icon has no pixels")

// A dark mark is at least darkMarkCoverage transparent with visible pixels below darkMarkLuminance.
const (
	darkMarkCoverage  = 0.85
	darkMarkLuminance = 0.3
)

// icoSignature starts every ICO file.
const icoSignature = "\x00\x00\x01\x00"

// decodeFetched decodes ICO with go-ico; besticon's decoder drops a 32-bit bitmap entry's alpha channel.
func decodeFetched(data []byte) (image.Image, error) {
	if bytes.HasPrefix(data, []byte(icoSignature)) {
		return ico.Decode(bytes.NewReader(data))
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	return source, err
}

// normalize returns an icon as a PNG fitted to an IconSize square on transparency, a dark mark on white.
func normalize(data []byte) ([]byte, error) {
	source, err := decodeFetched(data)
	if err != nil {
		return nil, err
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, errEmptyImage
	}
	fitWidth, fitHeight := IconSize, IconSize
	if width > height {
		fitHeight = max(1, (height*IconSize+width/2)/width)
	} else if height > width {
		fitWidth = max(1, (width*IconSize+height/2)/height)
	}
	left, top := (IconSize-fitWidth)/2, (IconSize-fitHeight)/2
	canvas := image.NewNRGBA(image.Rect(0, 0, IconSize, IconSize))
	draw.CatmullRom.Scale(canvas, image.Rect(left, top, left+fitWidth, top+fitHeight), source, bounds, draw.Over, nil)
	if darkMark(canvas) {
		plated := image.NewNRGBA(canvas.Bounds())
		draw.Draw(plated, plated.Bounds(), image.White, image.Point{}, draw.Src)
		draw.Draw(plated, plated.Bounds(), canvas, image.Point{}, draw.Over)
		canvas = plated
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// darkMark reports a dark mark, weighting each pixel's Rec. 709 luminance by its opacity.
func darkMark(icon *image.NRGBA) bool {
	var opacity, light float64
	for offset := 0; offset < len(icon.Pix); offset += 4 {
		pixel := icon.Pix[offset : offset+4 : offset+4]
		alpha := float64(pixel[3]) / 255
		opacity += alpha
		light += alpha * (0.2126*float64(pixel[0]) + 0.7152*float64(pixel[1]) + 0.0722*float64(pixel[2])) / 255
	}
	if opacity == 0 {
		return false
	}
	coverage := opacity / float64(len(icon.Pix)/4)
	return coverage < darkMarkCoverage && light/opacity < darkMarkLuminance
}
