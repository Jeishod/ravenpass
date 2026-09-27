package siteicons

import (
	"encoding/base64"
	"image/color"
	"testing"
)

func TestTintIsTheMainColourOfAColouredIcon(t *testing.T) {
	icon, err := normalize(encodePNG(t, mark(64, color.NRGBA{R: 230, G: 120, B: 20, A: 255})))
	if err != nil {
		t.Fatal(err)
	}
	if got := Tint(base64.StdEncoding.EncodeToString(icon)); got != "#e67814" {
		t.Fatalf("tint = %q, want #e67814", got)
	}
}

func TestGreyAndBrokenIconsHaveNoTint(t *testing.T) {
	grey, err := normalize(encodePNG(t, mark(64, color.NRGBA{R: 120, G: 120, B: 125, A: 255})))
	if err != nil {
		t.Fatal(err)
	}
	for name, icon := range map[string]string{
		"grey":       base64.StdEncoding.EncodeToString(grey),
		"empty":      "",
		"not base64": "%%%",
		"not a PNG":  base64.StdEncoding.EncodeToString([]byte("GIF89a")),
	} {
		if got := Tint(icon); got != "" {
			t.Fatalf("%s: tint = %q, want none", name, got)
		}
	}
}
