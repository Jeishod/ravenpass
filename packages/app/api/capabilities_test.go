package api

import (
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/dortanes/ravenpass/packages/app/siteicons"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// newServiceWithoutExtensions builds a service with no link server or panel window, offering offers.
func newServiceWithoutExtensions(t *testing.T, offers Capabilities) *Service {
	t.Helper()
	return newServiceOnHost(t, Host{Offers: offers})
}

// newServiceOnHost builds the service host wires, with no link server, panel window or app window.
func newServiceOnHost(t *testing.T, host Host) *Service {
	t.Helper()
	return newServiceOnHostWith(t, host, newTestPreferences(t))
}

// newServiceOnHostWith is newServiceOnHost with the host's preferences.
func newServiceOnHostWith(t *testing.T, host Host, settings *preferences.Store) *Service {
	t.Helper()
	files, err := storage.NewManager(
		filepath.Join(t.TempDir(), "storage.json"),
		storage.Target{Kind: storage.LocalFile, Path: filepath.Join(t.TempDir(), localfile.DefaultVaultName)},
		MaxVaultBytes,
		localfile.Backend{},
	)
	if err != nil {
		t.Fatal(err)
	}
	core, err := vaultservice.New(files, newStubKeys(), newStubDevice().bindings())
	if err != nil {
		t.Fatal(err)
	}
	icons, err := siteicons.New(filepath.Join(t.TempDir(), "icons"), core, silentSites{}, settings.SiteIcons())
	if err != nil {
		t.Fatal(err)
	}
	queue, err := confirmation.New(core)
	if err != nil {
		t.Fatal(err)
	}
	windows := &fakeWindows{}
	host.Confirmations = Confirmations{
		Queue: queue, ShowMain: windows.showMain, ReloadMain: windows.reloadMain,
		Owner: ownerVerifier(t, core, &answeringOwner{}, queue),
	}
	host.CurrentApp = func() *application.App { return nil }
	service, err := New(core, settings, icons, host)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestCapabilitiesReportWhatTheHostOffers(t *testing.T) {
	offers := Capabilities{Shortcuts: true, SaveFiles: true, CopyScans: true}
	if got := newServiceWithoutExtensions(t, offers).Capabilities(); got != offers {
		t.Fatalf("capabilities = %+v, want %+v", got, offers)
	}
}

func TestALinkServerOffersExtensions(t *testing.T) {
	service, _ := newServiceWithStorage(t, filepath.Join(t.TempDir(), "storage.json"), filepath.Join(t.TempDir(), localfile.DefaultVaultName))
	if !service.Capabilities().Extensions {
		t.Fatal("a host with a link server does not offer extensions")
	}
}

func TestAHostWithoutExtensionsLinksNone(t *testing.T) {
	service := newServiceWithoutExtensions(t, Capabilities{})
	if service.Capabilities().Extensions {
		t.Fatal("a host without a link server offers extensions")
	}
	links, err := service.ExtensionLinks()
	if err != nil || len(links.Extensions) != 0 || links.Reachable {
		t.Fatalf("extension links = %+v, %v; want none, unreachable", links, err)
	}
	if _, err := service.BeginExtensionLink(); err == nil || err.Error() != fail(failureLinkUnavailable).Error() {
		t.Fatalf("beginning a link = %v, want %s", err, failureLinkUnavailable)
	}
	service.FitConfirmation(320)
	if err := service.Lock(); err != nil {
		t.Fatalf("locking = %v", err)
	}
}
