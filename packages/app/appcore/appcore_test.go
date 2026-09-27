package appcore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/api"
	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/unlock/unlocktest"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type fakeOwner struct{ available bool }

func (o fakeOwner) Available() bool { return o.available }

func (fakeOwner) Authenticate(context.Context, string) error { return errors.New("no prompt in tests") }

type fakeDestination struct{}

func (fakeDestination) Save(string, string, []byte) (string, error) {
	return "", errors.New("no folder")
}

func (fakeDestination) Remove(string) error { return errors.New("no folder") }

func platform(t *testing.T) Platform {
	t.Helper()
	return Platform{
		Directory:         t.TempDir(),
		Languages:         func() []string { return []string{"en"} },
		Owner:             fakeOwner{available: true},
		PIN:               unlocktest.NewBinding(),
		Presence:          unlocktest.NewPresenceBinding(),
		BackupDestination: fakeDestination{},
	}
}

func TestBuildAssemblesEveryServiceOverTheHostsDirectory(t *testing.T) {
	services, err := Build(platform(t))
	if err != nil {
		t.Fatal(err)
	}
	if services.Files == nil || services.Settings == nil || services.Vault == nil || services.Icons == nil ||
		services.Confirmations == nil || services.Verifier == nil || services.Autofill == nil || services.Backups == nil {
		t.Fatalf("services missing: %+v", services)
	}
	if !services.Owner.DeviceOwnerAvailable() {
		t.Fatal("the host's owner authentication was not used")
	}
	if services.Vault.Unlocked() {
		t.Fatal("a vault opened without its owner")
	}
}

func TestBuildKeepsTheLocalFileFirstAndTheHostsBackendsAfterIt(t *testing.T) {
	p := platform(t)
	p.Backends = []storage.Backend{fakeBackend{}}
	services, err := Build(p)
	if err != nil {
		t.Fatal(err)
	}
	status := services.Files.Status()
	want := []storage.Kind{storage.LocalFile, fakeKind}
	if len(status.Kinds) != len(want) || status.Kinds[0] != want[0] || status.Kinds[1] != want[1] {
		t.Fatalf("kinds %v, want %v", status.Kinds, want)
	}
	wantDefault := storage.Target{Kind: storage.LocalFile, Path: filepath.Join(p.Directory, localfile.DefaultVaultName)}
	if status.Default != wantDefault {
		t.Fatalf("default location %+v, want %+v", status.Default, wantDefault)
	}
}

func TestAnUnreachableLocationStillBuilds(t *testing.T) {
	p := platform(t)
	if err := os.WriteFile(filepath.Join(p.Directory, "storage.json"), []byte("not a record"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(p); err != nil {
		t.Fatalf("an unreadable location record stopped the build: %v", err)
	}
}

func TestBuildRefusesAPlatformWithoutItsRequiredParts(t *testing.T) {
	for name, change := range map[string]func(*Platform){
		"directory":          func(p *Platform) { p.Directory = "" },
		"languages":          func(p *Platform) { p.Languages = nil },
		"backup destination": func(p *Platform) { p.BackupDestination = nil },
	} {
		p := platform(t)
		change(&p)
		if _, err := Build(p); err == nil {
			t.Errorf("built without the %s", name)
		}
	}
}

func TestWordsUseTheInterfaceLanguage(t *testing.T) {
	services, err := Build(platform(t))
	if err != nil {
		t.Fatal(err)
	}
	reason := confirmation.SavingPasskey("example.com")
	if got, want := services.Words(reason), reason.Words(services.Settings.Dialogs()); got != want {
		t.Fatalf("words %q, want %q", got, want)
	}
}

func TestServeWiresTheConfirmationsVerifierAndBackups(t *testing.T) {
	services, err := Build(platform(t))
	if err != nil {
		t.Fatal(err)
	}
	service, err := services.Serve(api.Host{
		Confirmations: api.Confirmations{ShowMain: func() {}, ReloadMain: func() {}},
		CurrentApp:    func() *application.App { return nil },
	})
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	if service == nil {
		t.Fatal("no service")
	}
}

const fakeKind storage.Kind = "fake"

type fakeBackend struct{}

func (fakeBackend) Kind() storage.Kind { return fakeKind }

func (fakeBackend) Check(string) error { return nil }

func (fakeBackend) Open(storage.Target, int64) (storage.Store, error) {
	return nil, errors.New("no store")
}
