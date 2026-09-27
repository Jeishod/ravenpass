package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/vault"
)

func TestAwaitVaultChangeAnswersASaveAndEndsWithItsCall(t *testing.T) {
	service := newReadyService(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.AwaitVaultChange(canceled, 0); err == nil {
		t.Fatal("a canceled wait answered")
	}
	capture := vaultservice.Capture{Requester: vaultservice.OriginRequester("https://example.com"), Account: "alex", Password: "typed"}
	if _, err := service.vault.SaveCapture(capture, vaultservice.CaptureChoice{Name: "Example", Account: "alex"}, ""); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithTimeout(context.Background(), hangLimit)
	defer stop()
	if count, err := service.AwaitVaultChange(ctx, 0); err != nil || count != 1 {
		t.Fatalf("change count = %d, error = %v", count, err)
	}
}

// sharedVaultFile is a synced vault file open on the creating device's service and a recovering phone's vault service.
func sharedVaultFile(t *testing.T) (*Service, *vaultservice.Service, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), localfile.DefaultVaultName)
	device := func() *vaultservice.Service {
		files, err := storage.NewManager(filepath.Join(t.TempDir(), "storage.json"), storage.Target{Kind: storage.LocalFile, Path: path}, MaxVaultBytes, localfile.Backend{})
		if err != nil {
			t.Fatal(err)
		}
		if err := files.Open(); err != nil {
			t.Fatal(err)
		}
		core, err := vaultservice.New(files, newStubKeys(), newStubDevice().bindings())
		if err != nil {
			t.Fatal(err)
		}
		return core
	}
	mac := device()
	service := newTestService(t, mac, silentSites{}, filepath.Join(t.TempDir(), "icons"))
	phrase, err := mac.BeginCreation()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mac.ConfirmCreation(phrase, vaultservice.MethodChoice{Biometry: true}); err != nil {
		t.Fatal(err)
	}
	phone := device()
	if _, err := phone.BeginRecovery(phrase); err != nil {
		t.Fatal(err)
	}
	if _, err := phone.ConfirmRecovery(true, vaultservice.MethodChoice{Biometry: true}); err != nil {
		t.Fatal(err)
	}
	return service, phone, path
}

func saveOn(t *testing.T, core *vaultservice.Service, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if _, err := core.CreateCredential(vault.CredentialInput{Label: label, Password: "secret"}, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFollowingTheVaultFileShowsAnotherDevicesSaves(t *testing.T) {
	service, phone, _ := sharedVaultFile(t)
	saveOn(t, phone, "saved on the phone")
	if ControlsOf(service).FollowVaultFile() {
		t.Fatal("a successor locked the vault")
	}
	ctx, stop := context.WithTimeout(context.Background(), hangLimit)
	defer stop()
	if count, err := service.AwaitVaultChange(ctx, 0); err != nil || count != 1 {
		t.Fatalf("change count = %d, error = %v", count, err)
	}
	if listed, err := service.ListCredentials(); err != nil || len(listed) != 1 || listed[0].Label != "saved on the phone" {
		t.Fatalf("credentials = %+v, error = %v", listed, err)
	}
}

func TestAVaultChangedAtTheSameTimeOpensOnceItsOwnerConfirms(t *testing.T) {
	service, phone, path := sharedVaultFile(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateCredential(CredentialInput{Label: "saved on this device"}, nil); err != nil {
		t.Fatal(err)
	}
	// The synced folder keeps the phone's file, written from the version before the other device's save.
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	saveOn(t, phone, "saved on the phone", "saved on the phone again")
	if !ControlsOf(service).FollowVaultFile() {
		t.Fatal("a diverged file left the vault open")
	}
	if _, err := service.ListCredentials(); !errors.Is(err, failureVaultLocked) {
		t.Fatalf("listing after the lock: %v", err)
	}
	if err := service.Unlock(); !errors.Is(err, failureVaultDiverged) {
		t.Fatalf("unlocking: got %v, want vault-diverged", err)
	}
	if state, err := service.GetState(); err != nil || state.Phase != "locked" {
		t.Fatalf("state = %+v, error = %v", state, err)
	}
	if err := service.AdoptChangedVault(); !errors.Is(err, failureSetupNotActive) {
		t.Fatalf("adopting after the screen loaded again: got %v, want setup-not-active", err)
	}
	if err := service.Unlock(); !errors.Is(err, failureVaultDiverged) {
		t.Fatalf("unlocking again: got %v, want vault-diverged", err)
	}
	if err := service.AdoptChangedVault(); err != nil {
		t.Fatal(err)
	}
	listed, err := service.ListCredentials()
	if err != nil || len(listed) != 2 {
		t.Fatalf("the adopted version lists %+v, error = %v", listed, err)
	}
	for _, credential := range listed {
		if credential.Label == "saved on this device" {
			t.Fatal("this device's replaced change is still listed")
		}
	}
}
