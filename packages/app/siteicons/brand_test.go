package siteicons

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/dortanes/ravenpass/packages/vault"
)

// brandSite serves a home page and a manifest over TLS and records the paths asked.
type brandSite struct {
	server   *httptest.Server
	mu       sync.Mutex
	asked    []string
	page     string
	status   int
	manifest string
}

func newBrandSite(t *testing.T, page, manifest string) *brandSite {
	t.Helper()
	site := &brandSite{page: page, manifest: manifest, status: http.StatusOK}
	site.server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		site.mu.Lock()
		site.asked = append(site.asked, request.URL.Path)
		site.mu.Unlock()
		switch request.URL.Path {
		case "/":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(site.status)
			writer.Write([]byte(site.page))
		case "/site.webmanifest":
			writer.Header().Set("Content-Type", "application/manifest+json")
			writer.Write([]byte(site.manifest))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(site.server.Close)
	return site
}

func (b *brandSite) host() string { return strings.TrimPrefix(b.server.URL, "https://") }

func (b *brandSite) paths() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.asked...)
}

func (b *brandSite) brand(t *testing.T) Brand {
	t.Helper()
	fetcher := &Fetcher{transport: b.server.Client().Transport}
	brand, err := fetcher.Brand(context.Background(), b.host())
	if err != nil {
		t.Fatal(err)
	}
	return brand
}

func head(tags ...string) string {
	return "<!doctype html><html><head>" + strings.Join(tags, "") + "</head><body>Bank</body></html>"
}

const manifestLink = `<link rel="icon manifest" href="/site.webmanifest">`

func TestBrandReadsEachColourAndNameSource(t *testing.T) {
	tests := []struct {
		name     string
		page     string
		manifest string
		want     Brand
	}{
		{"theme colour and site name", head(`<meta name="theme-color" content="#00A0E1">`, `<meta property="og:site_name" content="Jyske  Bank">`), "", Brand{Name: "Jyske Bank", Color: "#00a0e1"}},
		{"a colour without media before one with it", head(`<meta name="theme-color" media="(prefers-color-scheme: dark)" content="#112233">`, `<meta name="theme-color" content="rgb(20, 100, 200)">`, `<meta name="application-name" content="Monzo">`), "", Brand{Name: "Monzo", Color: "#1464c8"}},
		{"the first colour with media", head(`<meta name="Theme-Color" media="(prefers-color-scheme: light)" content="hsl(0, 100%, 50%)">`, `<meta name="theme-color" media="(prefers-color-scheme: dark)" content="#00ff00">`, `<meta name="application-name" content="Red">`), "", Brand{Name: "Red", Color: "#ff0000"}},
		{"a named colour", head(`<meta name="theme-color" content="rebeccapurple">`, `<meta property="og:site_name" content="Purple">`), "", Brand{Name: "Purple", Color: "#663399"}},
		{"site name before application name", head(`<meta name="application-name" content="App">`, `<meta property="og:site_name" content="Site">`, `<meta name="theme-color" content="#ff5500">`), "", Brand{Name: "Site", Color: "#ff5500"}},
		{"white, grey, translucent and broken colours skipped", head(`<meta name="theme-color" content="#ffffff">`, `<meta name="theme-color" content="#808088">`, `<meta name="theme-color" content="rgba(255, 0, 0, 0.5)">`, `<meta name="theme-color" content="not a colour">`, `<meta name="theme-color" content="#0f7b3c">`, `<meta name="application-name" content="Green">`), "", Brand{Name: "Green", Color: "#0f7b3c"}},
		{"manifest colour and short name", head(manifestLink), `{"theme_color": "#e30613", "short_name": "Alfa", "name": "Alfa-Bank"}`, Brand{Name: "Alfa", Color: "#e30613"}},
		{"manifest name without a short name", head(manifestLink, `<meta name="theme-color" content="#123456">`), `{"name": "  Example\n Bank "}`, Brand{Name: "Example Bank", Color: "#123456"}},
		{"a grey page colour left to the manifest", head(manifestLink, `<meta name="theme-color" content="white">`, `<meta property="og:site_name" content="Northwind">`), `{"theme_color": "#f14635"}`, Brand{Name: "Northwind", Color: "#f14635"}},
		{"a long name dropped", head(`<meta property="og:site_name" content="`+strings.Repeat("a", vault.MaxBankNameLength+1)+`">`, `<meta name="application-name" content="Short">`), "", Brand{Name: "Short"}},
		{"a name at its limit kept", head(`<meta property="og:site_name" content="` + strings.Repeat("ü", vault.MaxBankNameLength) + `">`), "", Brand{Name: strings.Repeat("ü", vault.MaxBankNameLength)}},
		{"an organization in structured data", head(`<script type="application/ld+json">{"@type": "WebSite", "name": "Site", "publisher": {"@type": "Organization", "name": "Банк Пример"}}</script>`), "", Brand{Name: "Банк Пример"}},
		{"a bank in a structured data graph", head(`<script type="application/ld+json">{"@graph": [{"@type": ["Thing", "BankOrCreditUnion"], "name": "Т‑Банк"}]}</script>`), "", Brand{Name: "Т‑Банк"}},
		{"broken structured data skipped", head(`<script type="application/ld+json">{"@type": </script>`, `<title>Harbour Bank | Harbour Holding</title>`), "", Brand{Name: "Harbour Bank"}},
		{"application name before structured data", head(`<script type="application/ld+json">{"@type": "Organization", "name": "Org"}</script>`, `<meta name="application-name" content="App">`), "", Brand{Name: "App"}},
		{"manifest name before the title", head(manifestLink, `<title>Title</title>`), `{"short_name": "Manifest"}`, Brand{Name: "Manifest"}},
		{"the first part of the title", head(`<title>  Harbour Bank официальный сайт | Harbour Holding Corp.  </title>`), "", Brand{Name: "Harbour Bank официальный сайт"}},
		{"nothing declared", head(), "", Brand{}},
		{"a manifest that is not JSON", head(manifestLink), "<html>", Brand{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			site := newBrandSite(t, test.page, test.manifest)
			if got := site.brand(t); got != test.want {
				t.Fatalf("brand = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestTitleNameTakesThePartNamingTheHost(t *testing.T) {
	tests := []struct {
		site, title, want string
	}{
		{"chase.com", "Credit Card, Mortgage, Banking, Auto | Chase Online | Chase.com", "Chase Online"},
		{"examplebank.example", "Платежи, Переводы, Кредиты - Example Bank", "Example Bank"},
		{"monzo.com", "Open a Free Account — Monzo", "Monzo"},
		{"bank.co.uk", "Savings | Co-op", "Savings"},
		{"hbfin.example", "Harbour Bank | Harbour Holding Corp.", "Harbour Bank"},
		{"northwind.example", "Northwind.example – Магазин, Платежи", "Northwind.example"},
		{"example.com", "", ""},
	}
	for _, test := range tests {
		document, err := goquery.NewDocumentFromReader(strings.NewReader(head("<title>" + test.title + "</title>")))
		if err != nil {
			t.Fatal(err)
		}
		if got := titleName(document, test.site); got != test.want {
			t.Errorf("titleName(%q, %q) = %q, want %q", test.title, test.site, got, test.want)
		}
	}
}

func TestBrandAsksForTheManifestOnlyWhenThePageLeavesSomethingOpen(t *testing.T) {
	full := newBrandSite(t, head(manifestLink, `<meta name="theme-color" content="#00a0e1">`, `<meta name="application-name" content="Bank">`), `{"name": "Other"}`)
	if got := full.brand(t); got != (Brand{Name: "Bank", Color: "#00a0e1"}) {
		t.Fatalf("brand = %+v", got)
	}
	if paths := full.paths(); len(paths) != 1 || paths[0] != "/" {
		t.Fatalf("paths asked = %q", paths)
	}
	open := newBrandSite(t, head(manifestLink, `<meta name="theme-color" content="#00a0e1">`), `{"name": "Bank"}`)
	if got := open.brand(t); got != (Brand{Name: "Bank", Color: "#00a0e1"}) {
		t.Fatalf("brand = %+v", got)
	}
	if paths := open.paths(); len(paths) != 2 || paths[1] != "/site.webmanifest" {
		t.Fatalf("paths asked = %q", paths)
	}
}

func TestBrandRefusesAManifestOverHTTPOrOverItsSize(t *testing.T) {
	insecure := newBrandSite(t, head(`<link rel="manifest" href="http://`+"example.test"+`/site.webmanifest">`), `{"name": "Bank"}`)
	if got := insecure.brand(t); got != (Brand{}) {
		t.Fatalf("brand from an HTTP manifest = %+v", got)
	}
	oversized := newBrandSite(t, head(manifestLink), `{"name": "Bank", "description": "`+strings.Repeat("a", maxManifestBytes)+`"}`)
	if got := oversized.brand(t); got != (Brand{}) {
		t.Fatalf("brand from an oversized manifest = %+v", got)
	}
}

func TestBrandReadsNoMoreOfAPageThanItsBound(t *testing.T) {
	page := "<!doctype html><html><head><meta property=\"og:site_name\" content=\"Early\"></head><body>" +
		strings.Repeat("a", maxPageBytes) + `<meta name="theme-color" content="#ff0000"></body></html>`
	site := newBrandSite(t, page, "")
	if got := site.brand(t); got != (Brand{Name: "Early"}) {
		t.Fatalf("brand of an oversized page = %+v", got)
	}
}

func TestBrandOfAPageThatFailsIsEmpty(t *testing.T) {
	site := newBrandSite(t, head(`<meta name="theme-color" content="#ff0000">`), "")
	site.status = http.StatusNotFound
	fetcher := &Fetcher{transport: site.server.Client().Transport}
	brand, err := fetcher.Brand(context.Background(), site.host())
	if err != nil || brand != (Brand{}) {
		t.Fatalf("brand of a missing page = %+v, %v", brand, err)
	}
}

func TestBrandOfASiteThatDoesNotAnswer(t *testing.T) {
	site := newBrandSite(t, head(), "")
	fetcher := &Fetcher{transport: site.server.Client().Transport}
	host := site.host()
	site.server.Close()
	if _, err := fetcher.Brand(context.Background(), host); !errors.Is(err, ErrUnanswered) {
		t.Fatalf("a closed site gave %v", err)
	}
	if _, err := NewFetcher().Brand(context.Background(), newBrandSite(t, head(), "").host()); !errors.Is(err, ErrUnanswered) {
		t.Fatalf("a loopback site through the default transport gave %v", err)
	}
}
