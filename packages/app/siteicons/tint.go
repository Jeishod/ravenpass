package siteicons

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/mat/besticon/v3/colorfinder"
)

// minTintChroma is the channel spread, out of 255, below which a colour counts as grey.
const minTintChroma = 48

// Tint returns a stored icon's main colour as "#rrggbb", empty for a grey or undecodable icon.
func Tint(icon string) string {
	data, err := base64.StdEncoding.DecodeString(icon)
	if err != nil || len(data) == 0 {
		return ""
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	var finder colorfinder.ColorFinder
	main, err := finder.FindMainColor(decoded)
	if err != nil || !chromatic(main.R, main.G, main.B) {
		return ""
	}
	return hexColor(main.R, main.G, main.B)
}

// chromatic reports whether a colour is not a grey.
func chromatic(red, green, blue uint8) bool {
	return int(max(red, green, blue))-int(min(red, green, blue)) >= minTintChroma
}

func hexColor(red, green, blue uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", red, green, blue)
}
