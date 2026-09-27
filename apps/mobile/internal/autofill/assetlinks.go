package autofill

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/app/autofill"
)

const (
	maxStatementsSize = 128 << 10
	// fetchTimeout bounds one lookup, which outlives the request that started it.
	fetchTimeout      = 10 * time.Second
	concurrentFetches = 6
	// defaultAge keeps an answer without max-age, and a refusal.
	defaultAge = time.Hour
	minAge     = 5 * time.Minute
	maxAge     = 24 * time.Hour
	// failedAge keeps a lookup that did not reach the site.
	failedAge = 5 * time.Minute
	maxSites  = 1024
)

// The Asset Links relations under which a site lets an app use its sign-ins.
var loginRelations = []string{
	"delegate_permission/common.get_login_creds",
	"delegate_permission/common.handle_all_urls",
}

// AssetLinks reads the Android apps a site's own https://<site>/.well-known/assetlinks.json names.
type AssetLinks struct {
	client http.Client
	now    func() time.Time
	slots  chan struct{}

	mu         sync.Mutex
	generation uint64
	known      map[string]siteLinks
	pending    map[string]chan struct{}
}

type siteLinks struct {
	apps    []linkedApp
	expires time.Time
}

type linkedApp struct {
	pkg     string
	signers [][32]byte
}

// NewAssetLinks looks sites up with a copy of client that follows no redirect.
func NewAssetLinks(client *http.Client) *AssetLinks {
	links := &AssetLinks{
		client:  *client,
		now:     time.Now,
		slots:   make(chan struct{}, concurrentFetches),
		known:   make(map[string]siteLinks),
		pending: make(map[string]chan struct{}),
	}
	links.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return links
}

// Verified returns the sites whose Asset Links name app for sign-in, omitting any still being looked up when ctx ends.
func (a *AssetLinks) Verified(ctx context.Context, sites []string, app autofill.App) []string {
	if len(app.Signers) == 0 {
		return nil
	}
	for _, done := range a.lookUp(sites) {
		select {
		case <-done:
		case <-ctx.Done():
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	var verified []string
	for _, site := range sites {
		answer, known := a.known[site]
		if known && now.Before(answer.expires) && answer.names(app) && !slices.Contains(verified, site) {
			verified = append(verified, site)
		}
	}
	return verified
}

// Forget drops every answer, and those of lookups still under way once they end.
func (a *AssetLinks) Forget() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.generation++
	clear(a.known)
}

// lookUp returns the channels that close as the lookups of the sites without a fresh answer end.
func (a *AssetLinks) lookUp(sites []string) []chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	var waits []chan struct{}
	for _, site := range sites {
		if answer, known := a.known[site]; known && now.Before(answer.expires) {
			continue
		}
		done, running := a.pending[site]
		if !running {
			done = make(chan struct{})
			a.pending[site] = done
			go a.fetch(site, a.generation, done)
		}
		waits = append(waits, done)
	}
	return waits
}

func (a *AssetLinks) fetch(site string, generation uint64, done chan struct{}) {
	a.slots <- struct{}{}
	apps, age := a.statements(site)
	<-a.slots
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pending, site)
	close(done)
	if generation != a.generation {
		return
	}
	now := a.now()
	if len(a.known) >= maxSites {
		a.evict(now)
	}
	a.known[site] = siteLinks{apps: apps, expires: now.Add(age)}
}

// evict drops the expired answers, or else the one that expires first. The caller holds a.mu.
func (a *AssetLinks) evict(now time.Time) {
	var soonest string
	for site, answer := range a.known {
		if !now.Before(answer.expires) {
			delete(a.known, site)
			continue
		}
		if soonest == "" || answer.expires.Before(a.known[soonest].expires) {
			soonest = site
		}
	}
	if len(a.known) >= maxSites {
		delete(a.known, soonest)
	}
}

// statements returns the apps a site's Asset Links let sign in, and how long the answer holds.
func (a *AssetLinks) statements(site string) ([]linkedApp, time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+site+"/.well-known/assetlinks.json", nil)
	if err != nil {
		return nil, defaultAge
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, failedAge
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !jsonContent(response.Header.Get("Content-Type")) {
		return nil, defaultAge
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxStatementsSize+1))
	if err != nil {
		return nil, failedAge
	}
	apps, err := parseStatements(body)
	if err != nil {
		return nil, defaultAge
	}
	return apps, cacheAge(response.Header.Get("Cache-Control"))
}

func jsonContent(header string) bool {
	media, _, err := mime.ParseMediaType(header)
	return err == nil && media == "application/json"
}

// cacheAge is the max-age a Cache-Control header sets, within bounds, or defaultAge.
func cacheAge(header string) time.Duration {
	for _, directive := range strings.Split(header, ",") {
		name, value, found := strings.Cut(strings.TrimSpace(directive), "=")
		if !found || !strings.EqualFold(name, "max-age") {
			continue
		}
		seconds, err := strconv.ParseInt(strings.Trim(value, `"`), 10, 64)
		if err != nil || seconds < 0 {
			return defaultAge
		}
		age := maxAge
		if seconds < int64(maxAge/time.Second) {
			age = time.Duration(seconds) * time.Second
		}
		return max(age, minAge)
	}
	return defaultAge
}

var errStatements = errors.New("the Asset Links file is not a list of statements")

// parseStatements keeps a statement's well-formed digests when another is malformed.
func parseStatements(body []byte) ([]linkedApp, error) {
	if len(body) > maxStatementsSize {
		return nil, errStatements
	}
	var statements []struct {
		Relation []string `json:"relation"`
		Target   struct {
			Namespace    string   `json:"namespace"`
			PackageName  string   `json:"package_name"`
			Fingerprints []string `json:"sha256_cert_fingerprints"`
		} `json:"target"`
	}
	if err := json.Unmarshal(body, &statements); err != nil {
		return nil, errStatements
	}
	var apps []linkedApp
	for _, statement := range statements {
		target := statement.Target
		if target.Namespace != "android_app" || target.PackageName == "" ||
			!slices.ContainsFunc(statement.Relation, func(relation string) bool {
				return slices.Contains(loginRelations, relation)
			}) {
			continue
		}
		app := linkedApp{pkg: target.PackageName}
		for _, text := range target.Fingerprints {
			if digest, ok := fingerprint(text); ok {
				app.signers = append(app.signers, digest)
			}
		}
		if len(app.signers) > 0 {
			apps = append(apps, app)
		}
	}
	return apps, nil
}

func (s siteLinks) names(app autofill.App) bool {
	return slices.ContainsFunc(s.apps, func(linked linkedApp) bool {
		return linked.pkg == app.Package && slices.ContainsFunc(app.Signers, func(signer [32]byte) bool {
			return slices.Contains(linked.signers, signer)
		})
	})
}
