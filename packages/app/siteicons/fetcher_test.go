package siteicons

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mat/besticon/v3/besticon"
)

func picture(width, height int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			result.Set(x, y, color.NRGBA{R: 220, G: 70, B: 60, A: 255})
		}
	}
	return result
}

func encodePNG(t *testing.T, source image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func decodeIcon(t *testing.T, data []byte) image.Image {
	t.Helper()
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "png" {
		t.Fatalf("stored icon is not a PNG: %s, %v", format, err)
	}
	if bounds := decoded.Bounds(); bounds.Dx() != IconSize || bounds.Dy() != IconSize {
		t.Fatalf("stored icon is %dx%d", bounds.Dx(), bounds.Dy())
	}
	return decoded
}

func alphaAt(source image.Image, x, y int) uint32 {
	_, _, _, alpha := source.At(x, y).RGBA()
	return alpha
}

func TestNormalizeFitsEveryFormatIntoATransparentSquare(t *testing.T) {
	var gifData, jpegData bytes.Buffer
	if err := gif.Encode(&gifData, picture(16, 16), nil); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, picture(256, 256), nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"small gif":  gifData.Bytes(),
		"large jpeg": jpegData.Bytes(),
		"square png": encodePNG(t, picture(180, 180)),
	} {
		t.Run(name, func(t *testing.T) {
			icon, err := normalize(data)
			if err != nil {
				t.Fatal(err)
			}
			decoded := decodeIcon(t, icon)
			if alphaAt(decoded, IconSize/2, IconSize/2) == 0 {
				t.Fatal("the icon lost its pixels")
			}
		})
	}
}

// alphaBitmapICO is a 16 × 16 32-bit bitmap ICO, transparent but for an opaque red centre.
func alphaBitmapICO() []byte {
	const side = 16
	var data bytes.Buffer
	pixels := side * side * 4
	mask := side * 4
	write := func(values ...any) {
		for _, value := range values {
			_ = binary.Write(&data, binary.LittleEndian, value)
		}
	}
	write(uint16(0), uint16(1), uint16(1))
	write(uint8(side), uint8(side), uint8(0), uint8(0), uint16(1), uint16(32), uint32(40+pixels+mask), uint32(22))
	write(uint32(40), int32(side), int32(side*2), uint16(1), uint16(32), uint32(0), uint32(pixels+mask), int32(0), int32(0), uint32(0), uint32(0))
	for y := range side {
		for x := range side {
			if x >= side/4 && x < side*3/4 && y >= side/4 && y < side*3/4 {
				write(uint8(40), uint8(40), uint8(220), uint8(255))
			} else {
				write(uint8(0), uint8(0), uint8(0), uint8(0))
			}
		}
	}
	data.Write(make([]byte, mask))
	return data.Bytes()
}

func TestNormalizeKeepsTheAlphaOfABitmapICO(t *testing.T) {
	icon, err := normalize(alphaBitmapICO())
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeIcon(t, icon)
	if alphaAt(decoded, 1, 1) != 0 {
		t.Fatal("a transparent corner of the icon became opaque")
	}
	if alphaAt(decoded, IconSize/2, IconSize/2) == 0 {
		t.Fatal("the icon lost its pixels")
	}
}

func TestNormalizeKeepsTheAspectRatio(t *testing.T) {
	wide, err := normalize(encodePNG(t, picture(128, 32)))
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeIcon(t, wide)
	if alphaAt(decoded, IconSize/2, 2) != 0 || alphaAt(decoded, IconSize/2, IconSize-3) != 0 {
		t.Fatal("a wide icon was stretched to the full height")
	}
	if alphaAt(decoded, 1, IconSize/2) == 0 || alphaAt(decoded, IconSize-2, IconSize/2) == 0 {
		t.Fatal("a wide icon does not span the width")
	}
	tall, err := normalize(encodePNG(t, picture(10, 40)))
	if err != nil {
		t.Fatal(err)
	}
	decoded = decodeIcon(t, tall)
	if alphaAt(decoded, 2, IconSize/2) != 0 || alphaAt(decoded, IconSize/2, 1) == 0 {
		t.Fatal("a tall icon was not centred at full height")
	}
}

// mark draws a filled centred square of one colour, half the side, on transparency.
func mark(side int, ink color.NRGBA) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := side / 4; y < side*3/4; y++ {
		for x := side / 4; x < side*3/4; x++ {
			result.Set(x, y, ink)
		}
	}
	return result
}

func TestNormalizePlatesOnlyDarkMarksOnWhite(t *testing.T) {
	cases := map[string]struct {
		source image.Image
		plated bool
	}{
		"dark mark on transparency":    {mark(64, color.NRGBA{A: 255}), true},
		"light mark on transparency":   {mark(64, color.NRGBA{R: 250, G: 250, B: 250, A: 255}), false},
		"coloured mark":                {mark(64, color.NRGBA{R: 230, G: 120, B: 20, A: 255}), false},
		"dark icon filling its square": {picture(64, 64), false},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			icon, err := normalize(encodePNG(t, test.source))
			if err != nil {
				t.Fatal(err)
			}
			decoded := decodeIcon(t, icon)
			red, green, blue, alpha := decoded.At(1, 1).RGBA()
			white := alpha == 0xffff && red == 0xffff && green == 0xffff && blue == 0xffff
			if test.plated != white {
				t.Fatalf("corner = %v, want plated %t", decoded.At(1, 1), test.plated)
			}
		})
	}
}

func TestNormalizeRefusesWhatIsNotAnImage(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("<html></html>"), encodePNG(t, picture(8, 8))[:20]} {
		if _, err := normalize(data); err == nil {
			t.Fatalf("%q was accepted as an icon", data)
		}
	}
}

func TestOutcomeStoresIconsAndMissesButNotSilence(t *testing.T) {
	icon := &besticon.Icon{Format: "png", Width: 32, Height: 32, ImageData: encodePNG(t, picture(32, 32))}
	broken := &besticon.Icon{Format: "png", Width: 32, Height: 32, ImageData: []byte("not an image")}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name     string
		ctx      context.Context
		chosen   *besticon.Icon
		answered bool
		wantIcon bool
		wantErr  error
	}{
		{"an icon", context.Background(), icon, true, true, nil},
		{"answered without an icon", context.Background(), nil, true, false, nil},
		{"answered with an icon that does not decode", context.Background(), broken, true, false, nil},
		{"never answered", context.Background(), nil, false, false, ErrUnanswered},
		{"stopped", canceled, icon, true, false, context.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := outcome(test.ctx, test.chosen, test.answered)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if (got != nil) != test.wantIcon {
				t.Fatalf("icon = %d bytes, want one: %t", len(got), test.wantIcon)
			}
			if got != nil {
				decodeIcon(t, got)
			}
		})
	}
}

func TestRedirectsStayOnHTTPSForFiveHops(t *testing.T) {
	request := func(address string) *http.Request {
		parsed, err := url.Parse(address)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Request{URL: parsed}
	}
	via := func(count int) []*http.Request {
		return make([]*http.Request, count)
	}
	if err := checkRedirect(request("https://example.com/icon"), via(maxRedirects)); err != nil {
		t.Fatalf("fifth redirect refused: %v", err)
	}
	if err := checkRedirect(request("https://example.com/icon"), via(maxRedirects+1)); !errors.Is(err, errTooManyRedirects) {
		t.Fatalf("sixth redirect: %v", err)
	}
	if err := checkRedirect(request("http://example.com/icon"), via(1)); !errors.Is(err, errInsecureRedirect) {
		t.Fatalf("redirect to HTTP: %v", err)
	}
	if client := newClient(http.DefaultTransport); client.Jar != nil {
		t.Fatal("the client keeps cookies")
	}
}

func TestFetcherNeverConnectsToALoopbackSite(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.Header().Set("Content-Type", "image/png")
		writer.Write(encodePNG(t, picture(32, 32)))
	}))
	defer server.Close()
	site := strings.TrimPrefix(server.URL, "https://")
	icon, err := NewFetcher().Fetch(context.Background(), site)
	if !errors.Is(err, ErrUnanswered) || icon != nil {
		t.Fatalf("loopback site gave %d bytes, %v", len(icon), err)
	}
	if count := requests.Load(); count != 0 {
		t.Fatalf("the loopback site received %d requests", count)
	}
}

type scriptedTransport struct {
	err     error
	context context.Context
}

func (s *scriptedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	s.context = request.Context()
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody, Request: request}, nil
}

func TestAnswerRecorderCountsAnyResponseAndBindsTheFetchContext(t *testing.T) {
	fetch, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}

	refused := &scriptedTransport{err: errors.New("connection refused")}
	silent := &answerRecorder{base: refused, ctx: fetch}
	if _, err := silent.RoundTrip(request); err == nil {
		t.Fatal("a refused connection returned a response")
	}
	if silent.answered() {
		t.Fatal("a refused connection counted as an answer")
	}
	if refused.context != fetch {
		t.Fatal("the request did not carry the fetch context")
	}

	notFound := &scriptedTransport{}
	answering := &answerRecorder{base: notFound, ctx: fetch}
	response, err := answering.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if !answering.answered() {
		t.Fatal("a 404 response did not count as an answer")
	}
}
