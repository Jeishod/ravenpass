package photos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

// Every fixture shows upright quadrants red, green, blue, white; orientation-N.jpg needs its Exif orientation for that.
var quadrantColors = [4]color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func near(got color.Color, want color.RGBA) bool {
	r, g, b, _ := got.RGBA()
	distance := func(a uint32, b uint8) int { return max(int(a>>8)-int(b), int(b)-int(a>>8)) }
	return distance(r, want.R) < 48 && distance(g, want.G) < 48 && distance(b, want.B) < 48
}

func assertQuadrants(t *testing.T, picture image.Image, width, height int) {
	t.Helper()
	bounds := picture.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		t.Fatalf("picture is %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
	}
	for i, want := range quadrantColors {
		x := bounds.Min.X + (i%2)*width/2 + width/4
		y := bounds.Min.Y + (i/2)*height/2 + height/4
		if got := picture.At(x, y); !near(got, want) {
			t.Fatalf("quadrant %d is %v, want %v", i, got, want)
		}
	}
}

func openFixture(t *testing.T, name string) *Picture {
	t.Helper()
	picture, err := Open(fixture(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return picture
}

func TestEveryExifOrientationComesOutUpright(t *testing.T) {
	for value := 1; value <= 8; value++ {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			assertQuadrants(t, openFixture(t, fmt.Sprintf("orientation-%d.jpg", value)).upright, 48, 32)
		})
	}
}

func TestEachFormatDecodes(t *testing.T) {
	for _, name := range []string{"upright.png", "upright.webp"} {
		t.Run(name, func(t *testing.T) {
			assertQuadrants(t, openFixture(t, name).upright, 48, 32)
		})
	}
}

func TestHEICIsRefusedAsUnsupported(t *testing.T) {
	heic := []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic")
	if _, err := Open(heic); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("got %v, want ErrUnsupported", err)
	}
}

// pngHeader is a PNG that declares a size in its header and holds no pixels.
func pngHeader(width, height uint32) []byte {
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:], width)
	binary.BigEndian.PutUint32(header[4:], height)
	header[8], header[9] = 8, 6
	chunk := append([]byte("IHDR"), header...)
	data := append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0, 13)
	data = append(data, chunk...)
	return binary.BigEndian.AppendUint32(data, crc32.ChecksumIEEE(chunk))
}

func TestOpenRefusesWhatItCannotUse(t *testing.T) {
	orientation := fixture(t, "orientation-1.jpg")
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"over the file size", make([]byte, MaxFileBytes+1), ErrTooLarge},
		{"over the pixel count", pngHeader(10_000, 5_001), ErrTooLarge},
		{"at the pixel count but truncated", pngHeader(10_000, 5_000), ErrUnsupported},
		{"empty", nil, ErrUnsupported},
		{"text", []byte("not a picture"), ErrUnsupported},
		{"GIF", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), ErrUnsupported},
		{"truncated JPEG", orientation[:len(orientation)/2], ErrUnsupported},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Open(test.data); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestPreviewKeepsASmallPictureAndShrinksALargeOne(t *testing.T) {
	preview, size, err := openFixture(t, "orientation-6.jpg").Preview()
	if err != nil {
		t.Fatal(err)
	}
	if size != image.Pt(48, 32) {
		t.Fatalf("preview size = %v", size)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(preview))
	if err != nil {
		t.Fatal(err)
	}
	assertQuadrants(t, decoded, 48, 32)

	wide := image.NewRGBA(image.Rect(0, 0, 3200, 400))
	draw.Draw(wide, image.Rect(0, 0, 1600, 400), image.NewUniform(quadrantColors[0]), image.Point{}, draw.Src)
	draw.Draw(wide, image.Rect(1600, 0, 3200, 400), image.NewUniform(quadrantColors[2]), image.Point{}, draw.Src)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, wide); err != nil {
		t.Fatal(err)
	}
	picture, err := Open(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if _, size, err := picture.Preview(); err != nil || size != image.Pt(previewSide, 200) {
		t.Fatalf("preview size = %v, error = %v", size, err)
	}
	square, err := picture.Crop(900, 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	cropped, err := png.Decode(bytes.NewReader(square))
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range []image.Point{{5, 5}, {256, 256}, {506, 506}} {
		if got := cropped.At(point.X, point.Y); !near(got, quadrantColors[2]) {
			t.Fatalf("a square right of the middle of the preview holds %v at %v", got, point)
		}
	}
}

func TestScanKeepsThePictureUprightWithinItsSide(t *testing.T) {
	scan, err := openFixture(t, "orientation-6.jpg").Scan()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(scan))
	if err != nil {
		t.Fatal(err)
	}
	assertQuadrants(t, decoded, 48, 32)

	tall := image.NewRGBA(image.Rect(0, 0, 100, vault.ScanSide*2))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, tall); err != nil {
		t.Fatal(err)
	}
	picture, err := Open(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	scan, err = picture.Scan()
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(scan))
	if err != nil || config.Width != 50 || config.Height != vault.ScanSide {
		t.Fatalf("scan is %dx%d, error = %v", config.Width, config.Height, err)
	}
}

func TestCropCutsTheChosenSquareFromTheUprightPicture(t *testing.T) {
	picture := openFixture(t, "orientation-8.jpg")
	square, err := picture.Crop(0, 0, 32)
	if err != nil {
		t.Fatal(err)
	}
	cropped, err := png.Decode(bytes.NewReader(square))
	if err != nil {
		t.Fatal(err)
	}
	if cropped.Bounds() != image.Rect(0, 0, vault.PhotoSize, vault.PhotoSize) {
		t.Fatalf("crop is %v", cropped.Bounds())
	}
	if got := cropped.At(100, 100); !near(got, quadrantColors[0]) {
		t.Fatalf("top left of the crop is %v, want red", got)
	}
	if got := cropped.At(400, 400); !near(got, quadrantColors[3]) {
		t.Fatalf("bottom right of the crop is %v, want white", got)
	}
	for _, square := range [][3]int{{0, 0, 0}, {0, 0, -1}, {-1, 0, 10}, {0, -1, 10}, {17, 0, 32}, {0, 1, 32}, {0, 0, 33}, {48, 0, 1}} {
		if _, err := picture.Crop(square[0], square[1], square[2]); !errors.Is(err, ErrOutside) {
			t.Fatalf("square %v: got %v, want ErrOutside", square, err)
		}
	}
	if _, err := picture.Crop(16, 0, 32); err != nil {
		t.Fatalf("a square touching the right edge was refused: %v", err)
	}
}
