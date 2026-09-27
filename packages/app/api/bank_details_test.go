package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/siteicons"
)

// scriptedBrands answers every site with one brand or error and records which sites were asked.
type scriptedBrands struct {
	mu    sync.Mutex
	brand siteicons.Brand
	err   error
	asked []string
}

func (s *scriptedBrands) Brand(_ context.Context, site string) (siteicons.Brand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, site)
	return s.brand, s.err
}

func (s *scriptedBrands) sites() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.asked)
}

// colouredSites answers every site with an icon of one orange.
func colouredSites(t *testing.T) *recordingSites {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			picture.Set(x, y, color.NRGBA{R: 230, G: 120, B: 20, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	return &recordingSites{icon: encoded.Bytes()}
}

func TestBankDetailsDefaultToOnAndCanBeTurnedOff(t *testing.T) {
	service := newReadyService(t)
	brands := &scriptedBrands{brand: siteicons.Brand{Name: "Jyske Bank", Color: "#00a0e1"}}
	service.brands = brands
	if current, err := service.GetBankDetails(); err != nil || !current.Enabled {
		t.Fatalf("default = %+v, %v", current, err)
	}
	if err := service.SetBankDetails(false); err != nil {
		t.Fatal(err)
	}
	if current, _ := service.GetBankDetails(); current.Enabled {
		t.Fatal("the bank lookup stayed on")
	}
	if lookup, err := service.LookupBank("jyskebank.dk"); err != nil || lookup != (BankLookup{}) {
		t.Fatalf("a lookup while off = %+v, %v", lookup, err)
	}
	if asked := brands.sites(); len(asked) != 0 {
		t.Fatalf("sites contacted while off = %q", asked)
	}
	if err := service.SetBankDetails(true); err != nil {
		t.Fatal(err)
	}
	if lookup, err := service.LookupBank("https://www.JyskeBank.dk/privat"); err != nil || lookup != (BankLookup{Name: "Jyske Bank", Color: "#00a0e1"}) {
		t.Fatalf("a lookup while on = %+v, %v", lookup, err)
	}
	if asked := brands.sites(); !slices.Equal(asked, []string{"jyskebank.dk"}) {
		t.Fatalf("sites contacted = %q", asked)
	}
}

func TestLookupBankAsksOnlyAWebSite(t *testing.T) {
	service := newReadyService(t)
	brands := &scriptedBrands{brand: siteicons.Brand{Name: "Bank"}}
	service.brands = brands
	for _, site := range []string{"", "  ", "mailto:bank@example.com", "ftp://bank.example"} {
		if lookup, err := service.LookupBank(site); err != nil || lookup != (BankLookup{}) {
			t.Fatalf("%q gave %+v, %v", site, lookup, err)
		}
	}
	if asked := brands.sites(); len(asked) != 0 {
		t.Fatalf("sites contacted = %q", asked)
	}
}

func TestLookupBankFillsAMissingColourFromTheSiteIcon(t *testing.T) {
	sites := colouredSites(t)
	service := newReadyServiceWithIcons(t, sites, filepath.Join(t.TempDir(), "icons"))
	service.brands = &scriptedBrands{brand: siteicons.Brand{Name: "Northwind"}}
	lookup, err := service.LookupBank("northwind.example")
	if err != nil || lookup != (BankLookup{Name: "Northwind", Color: "#e67814"}) {
		t.Fatalf("lookup = %+v, %v", lookup, err)
	}
	if asked := sites.sites(); !slices.Equal(asked, []string{"northwind.example"}) {
		t.Fatalf("icons fetched = %q", asked)
	}
	service.brands = &scriptedBrands{brand: siteicons.Brand{Name: "Monzo", Color: "#ff4f40"}}
	if lookup, err := service.LookupBank("monzo.com"); err != nil || lookup.Color != "#ff4f40" {
		t.Fatalf("a declared colour = %+v, %v", lookup, err)
	}
	if asked := sites.sites(); len(asked) != 1 {
		t.Fatalf("a declared colour fetched an icon: %q", asked)
	}
	if err := service.SetSiteIcons(false); err != nil {
		t.Fatal(err)
	}
	service.brands = &scriptedBrands{brand: siteicons.Brand{Name: "Harbour"}}
	if lookup, err := service.LookupBank("harbourbank.example"); err != nil || lookup != (BankLookup{Name: "Harbour"}) {
		t.Fatalf("a lookup with icons off = %+v, %v", lookup, err)
	}
	if asked := sites.sites(); len(asked) != 1 {
		t.Fatalf("icons off fetched an icon: %q", asked)
	}
}

func TestLookupBankOfASiteThatDoesNotAnswerIsEmpty(t *testing.T) {
	sites := colouredSites(t)
	service := newReadyServiceWithIcons(t, sites, filepath.Join(t.TempDir(), "icons"))
	service.brands = &scriptedBrands{err: siteicons.ErrUnanswered}
	if lookup, err := service.LookupBank("sberbank.ru"); err != nil || lookup != (BankLookup{}) {
		t.Fatalf("a silent site gave %+v, %v", lookup, err)
	}
	if asked := sites.sites(); len(asked) != 0 {
		t.Fatalf("a silent site's icon was fetched: %q", asked)
	}
}
