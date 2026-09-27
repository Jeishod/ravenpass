package autofill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/autofill"
)

// siteResponse is what a fake site answers for its Asset Links.
type siteResponse struct {
	status       int
	contentType  string
	cacheControl string
	location     string
	body         string
}

// fakeSites answers Asset Links requests by host, and fails for a host it does not know.
type fakeSites struct {
	mu        sync.Mutex
	responses map[string]siteResponse
	fetched   map[string]int
	paths     []string
}

func newFakeSites(responses map[string]siteResponse) *fakeSites {
	return &fakeSites{responses: responses, fetched: make(map[string]int)}
}

func (s *fakeSites) RoundTrip(request *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetched[request.URL.Host]++
	s.paths = append(s.paths, request.URL.Scheme+"://"+request.URL.Host+request.URL.Path)
	answer, known := s.responses[request.URL.Host]
	if !known {
		return nil, errors.New("the site is unreachable")
	}
	header := http.Header{}
	header.Set("Content-Type", answer.contentType)
	if answer.cacheControl != "" {
		header.Set("Cache-Control", answer.cacheControl)
	}
	if answer.location != "" {
		header.Set("Location", answer.location)
	}
	return &http.Response{StatusCode: answer.status, Header: header, Request: request,
		Body: io.NopCloser(strings.NewReader(answer.body))}, nil
}

func (s *fakeSites) count(host string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetched[host]
}

func signedApp(pkg, certificate string) autofill.App {
	return autofill.App{Package: pkg, Signers: [][32]byte{sha256.Sum256([]byte(certificate))}}
}

func colonHex(digest [32]byte) string {
	var parts []string
	for _, b := range digest {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return strings.Join(parts, ":")
}

// statementFor is an Asset Links file naming app under relation.
func statementFor(app autofill.App, relation string) string {
	return fmt.Sprintf(`[{"relation":[%q],"target":{"namespace":"android_app","package_name":%q,"sha256_cert_fingerprints":[%q]}}]`,
		relation, app.Package, colonHex(app.Signers[0]))
}

func served(body string) siteResponse {
	return siteResponse{status: http.StatusOK, contentType: "application/json", body: body}
}

func newLinks(sites *fakeSites) *AssetLinks {
	return NewAssetLinks(&http.Client{Transport: sites})
}

func TestASiteVerifiesTheAppItNamesForSignIn(t *testing.T) {
	app := signedApp("com.example.app", "example release key")
	lookalike := signedApp("com.example.app", "someone else's key")
	sites := newFakeSites(map[string]siteResponse{
		"example.com":      served(statementFor(app, "delegate_permission/common.get_login_creds")),
		"example.org":      served(statementFor(app, "delegate_permission/common.handle_all_urls")),
		"example.net":      served(statementFor(app, "delegate_permission/common.use_as_origin")),
		"other.example":    served(statementFor(signedApp("com.other", "other key"), "delegate_permission/common.get_login_creds")),
		"broken.example":   served(`{"not":"a list"}`),
		"notfound.example": {status: http.StatusNotFound, contentType: "application/json", body: "[]"},
	})
	links := newLinks(sites)
	all := []string{"example.com", "example.org", "example.net", "other.example", "broken.example", "notfound.example", "offline.example"}
	got := links.Verified(context.Background(), all, app)
	if !slices.Equal(got, []string{"example.com", "example.org"}) {
		t.Fatalf("verified %v", got)
	}
	if got := links.Verified(context.Background(), all, lookalike); len(got) != 0 {
		t.Fatalf("an app signed with another certificate was verified by %v", got)
	}
	for _, path := range sites.paths {
		if !strings.HasPrefix(path, "https://") || !strings.HasSuffix(path, "/.well-known/assetlinks.json") {
			t.Fatalf("looked up %s", path)
		}
	}
}

func TestAssetLinksRefuseRedirectsOtherTypesAndOversizedFiles(t *testing.T) {
	app := signedApp("com.example.app", "example release key")
	statement := statementFor(app, "delegate_permission/common.get_login_creds")
	sites := newFakeSites(map[string]siteResponse{
		"moved.example": {status: http.StatusFound, contentType: "application/json",
			location: "https://target.example/.well-known/assetlinks.json"},
		"target.example": served(statement),
		"text.example":   {status: http.StatusOK, contentType: "text/plain", body: statement},
		"large.example":  served(statement + strings.Repeat(" ", maxStatementsSize)),
		"typed.example":  {status: http.StatusOK, contentType: "application/json; charset=utf-8", body: statement},
	})
	links := newLinks(sites)
	got := links.Verified(context.Background(), []string{"moved.example", "text.example", "large.example", "typed.example"}, app)
	if !slices.Equal(got, []string{"typed.example"}) {
		t.Fatalf("verified %v", got)
	}
	if sites.count("target.example") != 0 {
		t.Fatal("a redirect was followed")
	}
}

func TestAnAnswerIsKeptForItsMaxAge(t *testing.T) {
	app := signedApp("com.example.app", "example release key")
	sites := newFakeSites(map[string]siteResponse{
		"example.com": {status: http.StatusOK, contentType: "application/json", cacheControl: "public, max-age=600",
			body: statementFor(app, "delegate_permission/common.get_login_creds")},
	})
	links := newLinks(sites)
	now := time.Unix(1_800_000_000, 0)
	links.now = func() time.Time { return now }
	for range 2 {
		if got := links.Verified(context.Background(), []string{"example.com"}, app); len(got) != 1 {
			t.Fatalf("verified %v", got)
		}
	}
	if sites.count("example.com") != 1 {
		t.Fatalf("fetched %d times within the max-age", sites.count("example.com"))
	}
	now = now.Add(601 * time.Second)
	links.Verified(context.Background(), []string{"example.com"}, app)
	if sites.count("example.com") != 2 {
		t.Fatalf("fetched %d times after the max-age", sites.count("example.com"))
	}
	links.Forget()
	links.Verified(context.Background(), []string{"example.com"}, app)
	if sites.count("example.com") != 3 {
		t.Fatalf("fetched %d times after forgetting", sites.count("example.com"))
	}
}

func TestMaxAgeStaysWithinBounds(t *testing.T) {
	for header, want := range map[string]time.Duration{
		"":                             defaultAge,
		"no-store":                     defaultAge,
		"max-age=7200":                 2 * time.Hour,
		"public, MAX-AGE=\"3600\"":     time.Hour,
		"max-age=1":                    minAge,
		"max-age=99999999999999999999": defaultAge,
		"max-age=99999999999999999":    maxAge,
		"max-age=31536000":             maxAge,
		"max-age=-5":                   defaultAge,
		"s-maxage=10, max-age=86400":   maxAge,
	} {
		if got := cacheAge(header); got != want {
			t.Errorf("%q: age %v, want %v", header, got, want)
		}
	}
}

func TestAWaitThatEndsLeavesTheLookupRunningForTheNextRequest(t *testing.T) {
	app := signedApp("com.example.app", "example release key")
	release := make(chan struct{})
	slow := roundTrip(func(request *http.Request) (*http.Response, error) {
		<-release
		return &http.Response{StatusCode: http.StatusOK, Request: request,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   io.NopCloser(strings.NewReader(statementFor(app, "delegate_permission/common.get_login_creds")))}, nil
	})
	links := NewAssetLinks(&http.Client{Transport: slow})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := links.Verified(ctx, []string{"example.com"}, app); len(got) != 0 {
		t.Fatalf("verified %v before the site answered", got)
	}
	close(release)
	if got := links.Verified(context.Background(), []string{"example.com"}, app); len(got) != 1 {
		t.Fatalf("verified %v once the site answered", got)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestStatementsKeepWellFormedDigestsOfAndroidApps(t *testing.T) {
	digest := sha256.Sum256([]byte("key"))
	body := fmt.Sprintf(`[
		{"relation":["delegate_permission/common.get_login_creds"],"target":{"namespace":"web","site":"https://example.com"}},
		{"relation":["delegate_permission/common.get_login_creds"],"target":{"namespace":"android_app","package_name":"com.example","sha256_cert_fingerprints":["zz",%q,%q]}},
		{"relation":["delegate_permission/common.get_login_creds"],"target":{"namespace":"android_app","package_name":"com.empty","sha256_cert_fingerprints":["12:34"]}}
	]`, colonHex(digest), hex.EncodeToString(digest[:]))
	apps, err := parseStatements([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].pkg != "com.example" || len(apps[0].signers) != 2 || apps[0].signers[0] != digest {
		t.Fatalf("apps %+v", apps)
	}
	if _, err := parseStatements([]byte("not json")); err == nil {
		t.Fatal("a file that is not JSON parsed")
	}
}
