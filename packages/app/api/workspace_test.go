package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/devicerecords"
	"github.com/dortanes/ravenpass/packages/app/extensionaccess"
	"github.com/dortanes/ravenpass/packages/app/linkserver"
	"github.com/dortanes/ravenpass/packages/app/linkstore"
	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/dortanes/ravenpass/packages/app/siteicons"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/unlock/unlocktest"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/app/verification"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type stubKeys struct {
	items map[string][]byte
}

func newStubKeys() *stubKeys { return &stubKeys{items: make(map[string][]byte)} }

func (s *stubKeys) save(kind, vaultID string, data []byte) error {
	s.items[kind+":"+vaultID] = bytes.Clone(data)
	return nil
}

func (s *stubKeys) load(kind, vaultID string) ([]byte, error) {
	value, exists := s.items[kind+":"+vaultID]
	if !exists {
		return nil, devicerecords.ErrNotFound
	}
	return bytes.Clone(value), nil
}

func (s *stubKeys) remove(kind, vaultID string) error {
	delete(s.items, kind+":"+vaultID)
	return nil
}

func (s *stubKeys) SaveHeadWitness(vaultID string, witness []byte) error {
	return s.save("head", vaultID, witness)
}
func (s *stubKeys) LoadHeadWitness(vaultID string) ([]byte, error) { return s.load("head", vaultID) }
func (s *stubKeys) SaveUsageRecord(vaultID string, usage []byte) error {
	return s.save("usage", vaultID, usage)
}
func (s *stubKeys) LoadUsageRecord(vaultID string) ([]byte, error) { return s.load("usage", vaultID) }
func (s *stubKeys) DeleteUsageRecord(vaultID string) error         { return s.remove("usage", vaultID) }
func (s *stubKeys) SaveExportRecord(vaultID string, record []byte) error {
	return s.save("export", vaultID, record)
}
func (s *stubKeys) LoadExportRecord(vaultID string) ([]byte, error) { return s.load("export", vaultID) }
func (s *stubKeys) DeleteExportRecord(vaultID string) error         { return s.remove("export", vaultID) }
func (s *stubKeys) DeleteHeadWitness(vaultID string) error          { return s.remove("head", vaultID) }
func (s *stubKeys) SaveUnlockPolicy(vaultID string, policy []byte) error {
	return s.save("policy", vaultID, policy)
}
func (s *stubKeys) LoadUnlockPolicy(vaultID string) ([]byte, error) {
	return s.load("policy", vaultID)
}
func (s *stubKeys) DeleteUnlockPolicy(vaultID string) error { return s.remove("policy", vaultID) }
func (s *stubKeys) SaveKeyRecord(vaultID string, keys []byte) error {
	return s.save("keys", vaultID, keys)
}
func (s *stubKeys) LoadKeyRecord(vaultID string) ([]byte, error) { return s.load("keys", vaultID) }
func (s *stubKeys) DeleteKeyRecord(vaultID string) error         { return s.remove("keys", vaultID) }

// stubDevice is the hardware and the owner a test vault opens with.
type stubDevice struct {
	noDeviceOwner bool
	pin           *unlocktest.Binding
	platform      *unlocktest.PresenceBinding
}

func newStubDevice() *stubDevice {
	return &stubDevice{pin: unlocktest.NewBinding(), platform: unlocktest.NewPresenceBinding()}
}

func (d *stubDevice) DeviceOwnerAvailable() bool { return !d.noDeviceOwner }

func (d *stubDevice) bindings() vaultservice.Device {
	return vaultservice.Device{Owner: d, PIN: d.pin, Platform: d.platform, PINKey: unlocktest.PINKey}
}

func newTestPreferences(t *testing.T) *preferences.Store {
	t.Helper()
	settings, err := preferences.New(filepath.Join(t.TempDir(), "preferences.json"), func() []string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

// silentSites is an icon source for which no site ever answers.
type silentSites struct{}

func (silentSites) Fetch(context.Context, string) ([]byte, error) {
	return nil, siteicons.ErrUnanswered
}

type keptInBackups struct{}

func (keptInBackups) Exclude(string) error { return nil }

// silentOwner is a device whose owner never answers the system prompt.
type silentOwner struct{}

func (silentOwner) AuthenticateOwner(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

// answeringOwner is a device whose owner answers the system prompt at once with answer, counting asks.
type answeringOwner struct {
	answer error

	mu    sync.Mutex
	asked int
}

func (o *answeringOwner) AuthenticateOwner(context.Context, string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.asked++
	return o.answer
}

func (o *answeringOwner) times() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.asked
}

// ownerVerifier verifies the owner of core's vault with owner or with the PIN through queue.
func ownerVerifier(t *testing.T, core *vaultservice.Service, owner verification.OwnerPrompt, queue *confirmation.Queue) *verification.Verifier {
	t.Helper()
	verifier, err := verification.New(core, owner, func(confirmation.Reason) string { return "change how your vault unlocks" }, queue)
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

// newTestService builds a service over core whose device owner approves every unlock change.
func newTestService(t *testing.T, core *vaultservice.Service, source siteicons.Source, iconDirectory string) *Service {
	t.Helper()
	settings := newTestPreferences(t)
	icons, err := siteicons.New(iconDirectory, core, source, settings.SiteIcons())
	if err != nil {
		t.Fatal(err)
	}
	links, err := linkstore.New(filepath.Join(t.TempDir(), "extensions.json"), keptInBackups{})
	if err != nil {
		t.Fatal(err)
	}
	queue, err := confirmation.New(core)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := verification.New(core, silentOwner{}, func(reason confirmation.Reason) string {
		return reason.Words(settings.Dialogs())
	}, queue)
	if err != nil {
		t.Fatal(err)
	}
	access, err := extensionaccess.New(core, core, icons, verifier, settings, queue)
	if err != nil {
		t.Fatal(err)
	}
	linkSettings, err := extensionaccess.NewSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	server, err := linkserver.New(links, access, linkSettings)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	windows := &fakeWindows{}
	service, err := New(core, settings, icons, Host{
		Links: server,
		Confirmations: Confirmations{
			Queue: queue, Panel: windows, ShowMain: windows.showMain, ReloadMain: windows.reloadMain,
			Owner: ownerVerifier(t, core, &answeringOwner{}, queue),
		},
		CurrentApp: func() *application.App { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	attachPasteboard(service)
	return service
}

// fakeWindows is the confirmation panel and main window, recording what the service asks of them.
type fakeWindows struct {
	mu       sync.Mutex
	heights  []int
	shown    int
	reloaded int
}

func (f *fakeWindows) Fit(height int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heights = append(f.heights, height)
}

func (f *fakeWindows) showMain() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown++
}

func (f *fakeWindows) reloadMain() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloaded++
}

func newReadyService(t *testing.T) *Service {
	t.Helper()
	return newReadyServiceWithIcons(t, silentSites{}, filepath.Join(t.TempDir(), "icons"))
}

func newReadyServiceWithIcons(t *testing.T, source siteicons.Source, iconDirectory string) *Service {
	t.Helper()
	return newReadyServiceWithKeys(t, newStubKeys(), source, iconDirectory)
}

// newReadyServiceWithKeys is a ready service whose device records live in keys.
func newReadyServiceWithKeys(t *testing.T, keys *stubKeys, source siteicons.Source, iconDirectory string) *Service {
	t.Helper()
	return newCreatedService(t, keys, source, iconDirectory)
}

// newCreatedService is a service whose vault was just created to open with device authentication.
func newCreatedService(t *testing.T, keys *stubKeys, source siteicons.Source, iconDirectory string) *Service {
	t.Helper()
	return newCreatedServiceOn(t, keys, newStubDevice(), source, iconDirectory)
}

// newCreatedServiceOn is newCreatedService with its vault opened on device.
func newCreatedServiceOn(t *testing.T, keys *stubKeys, device *stubDevice, source siteicons.Source, iconDirectory string) *Service {
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
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	core, err := vaultservice.New(files, keys, device.bindings())
	if err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, core, source, iconDirectory)
	phrase, err := core.BeginCreation()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.ConfirmCreation(phrase, vaultservice.MethodChoice{Biometry: true}); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestCredentialSummariesCarryPinAndRecentUse(t *testing.T) {
	service := newReadyService(t)
	id, err := service.CreateCredential(CredentialInput{Label: "Mail", Login: "alice", Email: "alice@example.com", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	summaries, err := service.ListCredentials()
	if err != nil || len(summaries) != 1 {
		t.Fatalf("summaries = %+v, error = %v", summaries, err)
	}
	if summaries[0].ID != id || summaries[0].Label != "Mail" || summaries[0].Login != "alice" || summaries[0].Email != "alice@example.com" {
		t.Fatalf("summary = %+v", summaries[0])
	}
	if summaries[0].Pinned || summaries[0].LastUsedAt != 0 {
		t.Fatalf("new credential reported as pinned or used: %+v", summaries[0])
	}
	if err := service.SetPinned(id, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadCredential(id); err != nil {
		t.Fatal(err)
	}
	summaries, err = service.ListCredentials()
	if err != nil || len(summaries) != 1 {
		t.Fatalf("summaries after use = %+v, error = %v", summaries, err)
	}
	if !summaries[0].Pinned || summaries[0].LastUsedAt <= 0 {
		t.Fatalf("pin or recent use was lost: %+v", summaries[0])
	}
	if err := service.SetPinned(id, false); err != nil {
		t.Fatal(err)
	}
	summaries, err = service.ListCredentials()
	if err != nil || summaries[0].Pinned {
		t.Fatalf("unpinned summary = %+v, error = %v", summaries[0], err)
	}
	if err := service.SetPinned("not a credential", true); err == nil {
		t.Fatal("an unreadable identifier was accepted")
	}
}

func TestCopyCredentialFieldCopiesOneFieldAtATime(t *testing.T) {
	service := newReadyService(t)
	websites := []string{"https://mail.example.com", "https://example.com/login"}
	id, err := service.CreateCredential(CredentialInput{Label: "Mail", Login: "alice", Email: "alice@example.com", Password: "secret", Notes: "recovery codes", Websites: websites}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clipboard := attachPasteboard(service)
	var discard func()
	schedule := func(after time.Duration, clear func()) {
		if after != preferences.DefaultClipboardClearing().After {
			t.Fatalf("clear delay = %v", after)
		}
		discard = clear
	}
	fields := map[string]string{
		"login": "alice", "email": "alice@example.com", "notes": "recovery codes",
		"website:0": websites[0], "website:1": websites[1], "password": "secret",
	}
	for field, want := range fields {
		if err := service.copyCredentialField(id, field, schedule); err != nil {
			t.Fatalf("copy of %s failed: %v", field, err)
		}
		if clipboard.text != want {
			t.Fatalf("clipboard after copying %s = %q", field, clipboard.text)
		}
	}
	if err := service.copyCredentialField(id, "password", schedule); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"label", "websites", "website", "website:", "website:-1", "website:+1", "website:01", "website:x", "website:2", ""} {
		assertFailure(t, service.copyCredentialField(id, field, schedule), failureFieldNotCopyable)
	}
	if clipboard.text != "secret" {
		t.Fatalf("a rejected field changed the clipboard to %q", clipboard.text)
	}
	if err := service.copyCredentialField("not a credential", "password", schedule); err == nil {
		t.Fatal("an unreadable identifier was accepted")
	}
	discard()
	if clipboard.text != "" {
		t.Fatal("the copied password stayed on the clipboard after its lifetime")
	}
	if err := service.copyCredentialField(id, "password", schedule); err != nil {
		t.Fatal(err)
	}
	service.clearClipboard()
	if clipboard.text != "" {
		t.Fatal("locking left the copied password on the clipboard")
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.copyCredentialField(id, "password", schedule); err == nil {
		t.Fatal("a locked vault served a credential field")
	}
}

func TestExportStatusIsUnknownUntilAnExportIsRecorded(t *testing.T) {
	service := newReadyService(t)
	if _, err := service.CreateCredential(CredentialInput{Label: "Mail", Password: "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	status, err := service.ExportStatus()
	if err != nil || status.State != "unknown" {
		t.Fatalf("status without a record = %+v, error = %v", status, err)
	}
	data, head, err := service.vault.Export()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.vault.RecordExport(head, sha256.Sum256(data)); err != nil {
		t.Fatal(err)
	}
	status, err = service.ExportStatus()
	if err != nil || status.State != "current" {
		t.Fatalf("status after an export = %+v, error = %v", status, err)
	}
	if _, err := service.CreateCredential(CredentialInput{Label: "Calendar", Password: "another"}, nil); err != nil {
		t.Fatal(err)
	}
	status, err = service.ExportStatus()
	if err != nil || status.State != "stale" {
		t.Fatalf("status after a change = %+v, error = %v", status, err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExportStatus(); err == nil {
		t.Fatal("a locked vault reported export freshness")
	}
}
