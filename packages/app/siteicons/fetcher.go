package siteicons

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/mat/besticon/v3/besticon"
)

const (
	fetchTimeout = 8 * time.Second
	maxRedirects = 5
	userAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15"
)

// iconFormats omits SVG, which needs a rasterizer.
var iconFormats = []string{"png", "ico", "gif", "jpg"}

var iconSizes = besticon.SizeRange{Min: 16, Perfect: 64, Max: 1024}

// ErrUnanswered means the site sent no HTTP response, so nothing is learned about its icon.
var ErrUnanswered = errors.New("site did not answer")

// Source returns a site's icon as an IconSize PNG; nil with no error means the site offers none.
type Source interface {
	Fetch(ctx context.Context, site string) ([]byte, error)
}

// Fetcher is the Source that asks each site over HTTPS for its declared, then conventional, icons.
type Fetcher struct {
	transport http.RoundTripper
}

// NewFetcher returns a Fetcher that refuses private and reserved addresses at every dial, redirects included.
func NewFetcher() *Fetcher {
	return &Fetcher{transport: besticon.NewDefaultHTTPTransport(userAgent)}
}

// Fetch bounds the whole fetch, page and icons together, by fetchTimeout.
func (f *Fetcher) Fetch(ctx context.Context, site string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	answers := &answerRecorder{base: f.transport, ctx: ctx}
	finder := besticon.New(
		besticon.WithHTTPClient(newClient(answers)),
		besticon.WithLogger(silentLogger{}),
		besticon.WithDefaultFormats(iconFormats...),
	).NewIconFinder()
	// FetchIcons fails only before any request; an unreachable site leaves the recorder unanswered.
	if _, err := finder.FetchIcons("https://" + site); err != nil {
		return nil, err
	}
	return outcome(ctx, finder.IconInSizeRange(iconSizes), answers.answered())
}

// outcome turns what the finder chose into the icon to store.
func outcome(ctx context.Context, chosen *besticon.Icon, answered bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !answered {
		return nil, ErrUnanswered
	}
	if chosen == nil {
		return nil, nil
	}
	icon, err := normalize(chosen.ImageData)
	if err != nil {
		return nil, nil
	}
	return icon, nil
}

// newClient keeps no cookies; besticon builds requests without a context, so the transport attaches it.
func newClient(transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: transport, CheckRedirect: checkRedirect}
}

var (
	errTooManyRedirects = errors.New("too many redirects")
	errInsecureRedirect = errors.New("redirect leaves HTTPS")
)

func checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) > maxRedirects {
		return errTooManyRedirects
	}
	if request.URL.Scheme != "https" {
		return errInsecureRedirect
	}
	return nil
}

// answerRecorder binds every request of one fetch to its context and records whether any response arrived.
type answerRecorder struct {
	base     http.RoundTripper
	ctx      context.Context
	received atomic.Bool
}

func (r *answerRecorder) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := r.base.RoundTrip(request.WithContext(r.ctx))
	if err == nil {
		r.received.Store(true)
	}
	return response, err
}

func (r *answerRecorder) answered() bool { return r.received.Load() }

type silentLogger struct{}

func (silentLogger) LogError(error) {}

func (silentLogger) LogResponse(*http.Request, *http.Response, time.Duration, error) {}
