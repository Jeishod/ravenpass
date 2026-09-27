package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// recordingSites answers every site with one icon and records which sites were asked.
type recordingSites struct {
	mu    sync.Mutex
	icon  []byte
	asked []string
}

func (r *recordingSites) Fetch(_ context.Context, site string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked = append(r.asked, site)
	return r.icon, nil
}

func (r *recordingSites) sites() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.asked)
}

func newRecordingSites(t *testing.T) *recordingSites {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	return &recordingSites{icon: encoded.Bytes()}
}

func TestCredentialsCarryTheirSite(t *testing.T) {
	service := newReadyService(t)
	websites := []string{"androidapp://com.example", "https://www.Example.com/login", "https://accounts.example.net", "example.com/other"}
	id, err := service.CreateCredential(CredentialInput{Label: "Example", Websites: websites, Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bare, err := service.CreateCredential(CredentialInput{Label: "Router", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	summaries, err := service.ListCredentials()
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range summaries {
		want := map[string]string{id: "example.com", bare: ""}[summary.ID]
		if summary.Site != want {
			t.Fatalf("summary site of %s = %q, want %q", summary.Label, summary.Site, want)
		}
		wantSites := map[string][]string{id: {"example.com", "accounts.example.net"}, bare: {}}[summary.ID]
		if summary.Sites == nil || !slices.Equal(summary.Sites, wantSites) {
			t.Fatalf("summary sites of %s = %#v, want %q", summary.Label, summary.Sites, wantSites)
		}
	}
	credential, err := service.ReadCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Site != "example.com" || !slices.Equal(credential.Websites, websites) {
		t.Fatalf("credential site = %q, websites = %q", credential.Site, credential.Websites)
	}
	if read, err := service.ReadCredential(bare); err != nil || read.Websites == nil || len(read.Websites) != 0 {
		t.Fatalf("websites of a credential without any = %#v, %v", read.Websites, err)
	}
}

func TestSiteIconIsServedOnlyForASiteACredentialNames(t *testing.T) {
	sites := newRecordingSites(t)
	service := newReadyServiceWithIcons(t, sites, filepath.Join(t.TempDir(), "icons"))
	if _, err := service.CreateCredential(CredentialInput{Label: "Example", Websites: []string{"example.com"}, Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	icon, err := service.SiteIcon("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if icon.Image != base64.StdEncoding.EncodeToString(sites.icon) {
		t.Fatalf("icon = %q", icon.Image)
	}
	for _, site := range []string{"", "internal.example", "10.0.0.1", "EXAMPLE.COM"} {
		icon, err := service.SiteIcon(site)
		if err != nil || icon != (SiteIcon{}) {
			t.Fatalf("site %q gave %q, %v", site, icon, err)
		}
	}
	if asked := sites.sites(); !slices.Equal(asked, []string{"example.com"}) {
		t.Fatalf("sites contacted = %q", asked)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SiteIcon("example.com"); err == nil {
		t.Fatal("a locked vault served an icon")
	}
}

func TestLockWritesTheIconCacheWhileTheVaultCanSealIt(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "icons")
	service := newReadyServiceWithIcons(t, newRecordingSites(t), directory)
	if _, err := service.CreateCredential(CredentialInput{Label: "Example", Websites: []string{"example.com"}, Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	id, err := service.vault.OpenVaultID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SiteIcon("example.com"); err != nil {
		t.Fatal(err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, id.String())); err != nil {
		t.Fatalf("locking did not write the icon cache: %v", err)
	}
}

func TestTurningSiteIconsOffDeletesTheCacheAndStopsLoading(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "icons")
	sites := newRecordingSites(t)
	service := newReadyServiceWithIcons(t, sites, directory)
	if current, err := service.GetSiteIcons(); err != nil || !current.Enabled {
		t.Fatalf("default = %+v, %v", current, err)
	}
	if _, err := service.CreateCredential(CredentialInput{Label: "Example", Websites: []string{"example.com"}, Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SiteIcon("example.com"); err != nil {
		t.Fatal(err)
	}
	service.icons.Release()
	if _, err := os.Stat(directory); err != nil {
		t.Fatal(err)
	}
	if err := service.SetSiteIcons(false); err != nil {
		t.Fatal(err)
	}
	if current, _ := service.GetSiteIcons(); current.Enabled {
		t.Fatal("website icons stayed on")
	}
	if _, err := os.Stat(directory); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the icon cache was kept: %v", err)
	}
	if icon, err := service.SiteIcon("example.com"); err != nil || icon != (SiteIcon{}) {
		t.Fatalf("icons off gave %q, %v", icon, err)
	}
	if asked := sites.sites(); len(asked) != 1 {
		t.Fatalf("sites contacted = %q", asked)
	}
	if err := service.SetSiteIcons(true); err != nil {
		t.Fatal(err)
	}
	if icon, err := service.SiteIcon("example.com"); err != nil || icon.Image == "" {
		t.Fatalf("icons back on gave %q, %v", icon, err)
	}
}
