package siteicons

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/dortanes/ravenpass/packages/vault"
	"github.com/mazznoer/csscolorparser"
)

const (
	maxPageBytes     = 2 << 20
	maxManifestBytes = 256 << 10
)

var errUnsuccessful = errors.New("site answered without success")

// Brand is the name and "#rrggbb" colour a site declares, each empty for none usable.
type Brand struct {
	Name  string
	Color string
}

// manifest holds the members of a web app manifest a brand is read from.
type manifest struct {
	ThemeColor string `json:"theme_color"`
	ShortName  string `json:"short_name"`
	Name       string `json:"name"`
}

// Brand reads a site's name and colour from its home page, then its manifest, with Fetch's transport and bounds.
func (f *Fetcher) Brand(ctx context.Context, site string) (Brand, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	answers := &answerRecorder{base: f.transport, ctx: ctx}
	client := newClient(answers)
	page, found := fetchPage(client, &url.URL{Scheme: "https", Host: site, Path: "/"})
	if err := ctx.Err(); err != nil {
		return Brand{}, err
	}
	if !answers.answered() {
		return Brand{}, ErrUnanswered
	}
	if !found {
		return Brand{}, nil
	}
	colors, names := pageColors(page.document), pageNames(page.document)
	brand := Brand{Color: firstColor(colors), Name: firstName(names)}
	if brand.Color != "" && brand.Name != "" {
		return brand, nil
	}
	if declared, found := fetchManifest(client, page); found {
		if brand.Color == "" {
			brand.Color = firstColor([]string{declared.ThemeColor})
		}
		if brand.Name == "" {
			brand.Name = firstName([]string{declared.ShortName, declared.Name})
		}
	}
	if brand.Name == "" {
		brand.Name = firstName([]string{titleName(page.document, site)})
	}
	if err := ctx.Err(); err != nil {
		return Brand{}, err
	}
	return brand, nil
}

type fetchedPage struct {
	document *goquery.Document
	address  *url.URL
}

// fetchPage reads at most maxPageBytes of a page and records its address after redirects.
func fetchPage(client *http.Client, address *url.URL) (fetchedPage, bool) {
	response, err := get(client, address)
	if err != nil {
		return fetchedPage{}, false
	}
	defer response.Body.Close()
	document, err := goquery.NewDocumentFromReader(io.LimitReader(response.Body, maxPageBytes))
	if err != nil {
		return fetchedPage{}, false
	}
	return fetchedPage{document: document, address: response.Request.URL}, true
}

// fetchManifest reads the manifest a page links to, over HTTPS only, of at most maxManifestBytes.
func fetchManifest(client *http.Client, page fetchedPage) (manifest, bool) {
	var link string
	page.document.Find("link[rel][href]").EachWithBreak(func(_ int, element *goquery.Selection) bool {
		if hasToken(element.AttrOr("rel", ""), "manifest") {
			link = strings.TrimSpace(element.AttrOr("href", ""))
			return false
		}
		return true
	})
	if link == "" {
		return manifest{}, false
	}
	reference, err := url.Parse(link)
	if err != nil {
		return manifest{}, false
	}
	address := page.address.ResolveReference(reference)
	if address.Scheme != "https" {
		return manifest{}, false
	}
	response, err := get(client, address)
	if err != nil {
		return manifest{}, false
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
	if err != nil || len(data) > maxManifestBytes {
		return manifest{}, false
	}
	var declared manifest
	if err := json.Unmarshal(data, &declared); err != nil {
		return manifest{}, false
	}
	return declared, true
}

// get requests an address and keeps only a successful response.
func get(client *http.Client, address *url.URL) (*http.Response, error) {
	response, err := client.Get(address.String())
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		response.Body.Close()
		return nil, errUnsuccessful
	}
	return response, nil
}

// pageColors lists a page's theme colours without a media query, then the first one with one.
func pageColors(document *goquery.Document) []string {
	var plain []string
	var queried []string
	document.Find("meta[name][content]").Each(func(_ int, element *goquery.Selection) {
		if !strings.EqualFold(strings.TrimSpace(element.AttrOr("name", "")), "theme-color") {
			return
		}
		if _, hasMedia := element.Attr("media"); hasMedia {
			queried = append(queried, element.AttrOr("content", ""))
			return
		}
		plain = append(plain, element.AttrOr("content", ""))
	})
	if len(queried) > 0 {
		plain = append(plain, queried[0])
	}
	return plain
}

// pageNames lists a page's declared names in the order they are tried.
func pageNames(document *goquery.Document) []string {
	var siteNames, applicationNames []string
	document.Find("meta[content]").Each(func(_ int, element *goquery.Selection) {
		content := element.AttrOr("content", "")
		if strings.EqualFold(strings.TrimSpace(element.AttrOr("property", "")), "og:site_name") {
			siteNames = append(siteNames, content)
		}
		if strings.EqualFold(strings.TrimSpace(element.AttrOr("name", "")), "application-name") {
			applicationNames = append(applicationNames, content)
		}
	})
	return append(append(siteNames, applicationNames...), organizationNames(document)...)
}

// organizationTypes are the schema.org types whose name is the business behind a site.
var organizationTypes = map[string]bool{
	"Organization":      true,
	"Corporation":       true,
	"BankOrCreditUnion": true,
	"FinancialService":  true,
}

// maxStructuredDepth bounds how deep structured data is searched, graph and nesting included.
const maxStructuredDepth = 8

// organizationNames lists the organization names in a page's valid JSON-LD blocks, in page order.
func organizationNames(document *goquery.Document) []string {
	var names []string
	document.Find(`script[type="application/ld+json"]`).Each(func(_ int, element *goquery.Selection) {
		var data any
		if json.Unmarshal([]byte(element.Text()), &data) == nil {
			names = appendOrganizationNames(names, data, 0)
		}
	})
	return names
}

func appendOrganizationNames(names []string, value any, depth int) []string {
	if depth > maxStructuredDepth {
		return names
	}
	switch node := value.(type) {
	case []any:
		for _, item := range node {
			names = appendOrganizationNames(names, item, depth+1)
		}
	case map[string]any:
		if name, named := node["name"].(string); named && isOrganization(node["@type"]) {
			names = append(names, name)
		}
		for _, key := range slices.Sorted(maps.Keys(node)) {
			names = appendOrganizationNames(names, node[key], depth+1)
		}
	}
	return names
}

// isOrganization reports whether a JSON-LD @type, one type or a list, names an organization.
func isOrganization(declared any) bool {
	switch types := declared.(type) {
	case string:
		return organizationTypes[types]
	case []any:
		for _, item := range types {
			if name, isText := item.(string); isText && organizationTypes[name] {
				return true
			}
		}
	}
	return false
}

// titleSeparator splits a title at a spaced bar, dash, dot or colon, as in "Monzo | Open a Free Account".
var titleSeparator = regexp.MustCompile(`\s+[|–—·:\-]\s+`)

// minSiteWordLength keeps labels such as "co" in "bank.co.uk" from matching a title part.
const minSiteWordLength = 3

// titleName is the first title part holding a host word, as "Chase Online" for chase.com, else the first part.
func titleName(document *goquery.Document, site string) string {
	parts := titleSeparator.Split(strings.TrimSpace(document.Find("title").First().Text()), -1)
	labels := strings.Split(site, ".")
	words := labels[:max(len(labels)-1, 0)]
	for _, part := range parts {
		letters := lettersOf(part)
		for _, word := range words {
			if len(word) >= minSiteWordLength && strings.Contains(letters, lettersOf(word)) {
				return part
			}
		}
	}
	return parts[0]
}

// lettersOf keeps a text's letters and digits in lowercase: "Example Bank" becomes "examplebank".
func lettersOf(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, text)
}

// firstColor is the first value that parses as a CSS colour that is opaque and not a grey.
func firstColor(values []string) string {
	for _, value := range values {
		parsed, err := csscolorparser.Parse(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		red, green, blue, alpha := parsed.RGBA255()
		if alpha == 255 && chromatic(red, green, blue) {
			return hexColor(red, green, blue)
		}
	}
	return ""
}

// firstName is the first value usable as a card's bank name, whitespace collapsed.
func firstName(values []string) string {
	for _, value := range values {
		name := strings.Join(strings.Fields(value), " ")
		if name != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= vault.MaxBankNameLength {
			return name
		}
	}
	return ""
}

func hasToken(list, token string) bool {
	for _, field := range strings.Fields(list) {
		if strings.EqualFold(field, token) {
			return true
		}
	}
	return false
}
