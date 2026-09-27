package api

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/localfile"
)

type fakeAbout struct {
	name, version string
	notices       []byte
	noticesErr    error
}

func (a fakeAbout) System() (string, string) { return a.name, a.version }

func (a fakeAbout) Notices() ([]byte, error) { return a.notices, a.noticesErr }

func newAboutService(t *testing.T, about fakeAbout) *Service {
	t.Helper()
	home := t.TempDir()
	service, _ := newServiceWithStorage(t, filepath.Join(home, "storage.json"), filepath.Join(home, localfile.DefaultVaultName))
	service.about = about
	return service
}

func TestSupportDetailsNameTheHostsSystem(t *testing.T) {
	service := newAboutService(t, fakeAbout{name: "Android", version: "15 (API 35)"})
	details, err := service.SupportDetails()
	if err != nil || details != (SupportDetails{Platform: "Android", OSVersion: "15 (API 35)"}) {
		t.Fatalf("support details = %+v, %v", details, err)
	}
}

func TestThirdPartyNoticesAreTheShippedText(t *testing.T) {
	service := newAboutService(t, fakeAbout{notices: []byte("Ravenpass is licensed under GPL-3.0-or-later.\n")})
	text, err := service.ThirdPartyNotices()
	if err != nil || text != "Ravenpass is licensed under GPL-3.0-or-later.\n" {
		t.Fatalf("notices = %q, %v", text, err)
	}
}

func TestABuildWithoutNoticesReportsNoText(t *testing.T) {
	missing := fmt.Errorf("open THIRD_PARTY_NOTICES.txt: %w", fs.ErrNotExist)
	service := newAboutService(t, fakeAbout{noticesErr: missing})
	if text, err := service.ThirdPartyNotices(); err != nil || text != "" {
		t.Fatalf("notices of a build without them = %q, %v", text, err)
	}
}

func TestUnreadableNoticesFail(t *testing.T) {
	service := newAboutService(t, fakeAbout{noticesErr: errors.New("read failed")})
	if _, err := service.ThirdPartyNotices(); err == nil {
		t.Fatal("unreadable notices were reported as read")
	}
}

func TestNoticesAreValidText(t *testing.T) {
	service := newAboutService(t, fakeAbout{notices: []byte("MIT \xff License")})
	if text, err := service.ThirdPartyNotices(); err != nil || text != "MIT � License" {
		t.Fatalf("notices = %q, %v", text, err)
	}
}
