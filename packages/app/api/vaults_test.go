package api

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/confirmation"
	"github.com/dortanes/ravenpass/packages/app/localfile"
	"github.com/dortanes/ravenpass/packages/app/ownerauth"
	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/app/verification"
)

// promptingOwner is a device whose owner answers each authentication prompt with answer, recording it.
type promptingOwner struct {
	answer error

	mu      sync.Mutex
	reasons []string
}

func (o *promptingOwner) AuthenticateOwner(_ context.Context, reason string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reasons = append(o.reasons, reason)
	return o.answer
}

func (o *promptingOwner) said() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.reasons)
}

// promptOwner makes owner the device service asks, with prompts worded in the recorded language.
func promptOwner(t *testing.T, service *Service, answer error) *promptingOwner {
	t.Helper()
	owner := &promptingOwner{answer: answer}
	words := func(reason confirmation.Reason) string { return reason.Words(service.preferences.Dialogs()) }
	verifier, err := verification.New(service.vault, owner, words, service.confirmations)
	if err != nil {
		t.Fatal(err)
	}
	service.owner = verifier
	return owner
}

// newServiceHoldingVault is a service whose device holds one vault, "vault.rpv" in home, open.
func newServiceHoldingVault(t *testing.T) (*Service, string) {
	t.Helper()
	home := t.TempDir()
	service, files := newServiceWithStorage(t, filepath.Join(home, "storage.json"), filepath.Join(home, localfile.DefaultVaultName))
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	createStagedVault(t, service)
	return service, home
}

// createStagedVault creates a vault where setup runs, opening with the device's authentication.
func createStagedVault(t *testing.T, service *Service) {
	t.Helper()
	phrase, err := service.BeginCreation()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ConfirmCreation(phrase, UnlockChoice{Biometry: true}); err != nil {
		t.Fatal(err)
	}
}

func currentPath(t *testing.T, service *Service) string {
	t.Helper()
	status, err := service.GetStorage()
	if err != nil {
		t.Fatal(err)
	}
	return status.Path
}

func TestCreatingAnotherVaultAsksTheDeviceFirst(t *testing.T) {
	service, home := newServiceHoldingVault(t)
	owner := promptOwner(t, service, nil)
	change, err := service.CreateVault(context.Background())
	if err != nil || !change.Changed || change.Path != filepath.Join(home, "vault 2.rpv") {
		t.Fatalf("creating with the owner's approval = %+v, %v", change, err)
	}
	if said := owner.said(); !slices.Equal(said, []string{"create a new vault"}) {
		t.Fatalf("the device's prompt said %q", said)
	}
}

func TestOpeningAVaultFileAsksTheDeviceBeforeThePicker(t *testing.T) {
	service, _ := newServiceHoldingVault(t)
	owner := promptOwner(t, service, nil)
	picker := &pickedFiles{}
	service.files = picker
	change, err := service.OpenVault(context.Background())
	if err != nil || change.Changed || picker.shown != 1 {
		t.Fatalf("opening with the owner's approval = %+v, %v, picker shown %d times", change, err, picker.shown)
	}
	if said := owner.said(); !slices.Equal(said, []string{"open another vault file"}) {
		t.Fatalf("the device's prompt said %q", said)
	}
}

func TestADeclinedCheckCreatesOpensAndDeletesNothing(t *testing.T) {
	service, home := newServiceHoldingVault(t)
	first := filepath.Join(home, localfile.DefaultVaultName)
	promptOwner(t, service, nil)
	if _, err := service.CreateVault(context.Background()); err != nil {
		t.Fatal(err)
	}
	createStagedVault(t, service)
	owner := promptOwner(t, service, ownerauth.ErrCanceled)
	picker := &pickedFiles{existing: storage.Target{Kind: storage.LocalFile, Path: first}}
	service.files = picker

	change, err := service.CreateVault(context.Background())
	if err != nil || change.Changed {
		t.Fatalf("a declined creation = %+v, %v", change, err)
	}
	if change, err = service.OpenVault(context.Background()); err != nil || change.Changed || picker.shown != 0 {
		t.Fatalf("a declined open = %+v, %v, picker shown %d times", change, err, picker.shown)
	}
	deletion, err := service.DeleteVault(context.Background(), first)
	if err != nil || deletion.Deleted {
		t.Fatalf("a declined deletion = %+v, %v", deletion, err)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("a declined deletion removed the file: %v", err)
	}
	if state, err := service.GetState(); err != nil || state.Phase != "ready" {
		t.Fatalf("state after declined checks = %+v, %v", state, err)
	}
	status, err := service.GetStorage()
	if err != nil || status.Path != filepath.Join(home, "vault 2.rpv") || len(status.Vaults) != 2 {
		t.Fatalf("storage after declined checks = %+v, %v", status, err)
	}
	want := []string{"create a new vault", "open another vault file", "delete the vault “vault”"}
	if said := owner.said(); !slices.Equal(said, want) {
		t.Fatalf("the device's prompts said %q, want %q", said, want)
	}
}

func TestAFailedOrUnavailableCheckSaysWhatHappened(t *testing.T) {
	for cause, want := range map[error]failure{
		ownerauth.ErrFailed:      failureAuthenticationFailed,
		ownerauth.ErrUnavailable: failureUnlockUnavailable,
	} {
		service, home := newServiceHoldingVault(t)
		promptOwner(t, service, cause)
		picker := &pickedFiles{}
		service.files = picker
		change, err := service.CreateVault(context.Background())
		assertFailure(t, err, want)
		if change.Changed {
			t.Fatalf("%v: a refused creation reported %+v", cause, change)
		}
		_, err = service.OpenVault(context.Background())
		assertFailure(t, err, want)
		if picker.shown != 0 {
			t.Fatalf("%v: a refused open showed the picker", cause)
		}
		deletion, err := service.DeleteVault(context.Background(), filepath.Join(home, localfile.DefaultVaultName))
		assertFailure(t, err, want)
		if deletion.Deleted || currentPath(t, service) != filepath.Join(home, localfile.DefaultVaultName) {
			t.Fatalf("%v: a refused deletion = %+v", cause, deletion)
		}
	}
}

func TestDeletingAVaultAsksTheDeviceWithItsName(t *testing.T) {
	service, home := newServiceHoldingVault(t)
	promptOwner(t, service, nil)
	if _, err := service.CreateVault(context.Background()); err != nil {
		t.Fatal(err)
	}
	createStagedVault(t, service)
	owner := promptOwner(t, service, nil)
	first := filepath.Join(home, localfile.DefaultVaultName)
	deletion, err := service.DeleteVault(context.Background(), first)
	if err != nil || !deletion.Deleted || deletion.Path != first {
		t.Fatalf("deleting with the owner's approval = %+v, %v", deletion, err)
	}
	if said := owner.said(); !slices.Equal(said, []string{"delete the vault “vault”"}) {
		t.Fatalf("the device's prompt said %q", said)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("the approved deletion left the file: %v", err)
	}
}

func TestSwitchingBetweenKnownVaultsAsksNothing(t *testing.T) {
	service, home := newServiceHoldingVault(t)
	promptOwner(t, service, nil)
	if _, err := service.CreateVault(context.Background()); err != nil {
		t.Fatal(err)
	}
	createStagedVault(t, service)
	owner := promptOwner(t, service, ownerauth.ErrCanceled)
	if err := service.SwitchVault(filepath.Join(home, localfile.DefaultVaultName)); err != nil {
		t.Fatal(err)
	}
	if said := owner.said(); len(said) != 0 {
		t.Fatalf("switching asked the device: %q", said)
	}
}

func TestADeviceHoldingNoVaultAsksNothing(t *testing.T) {
	home := t.TempDir()
	service, files := newServiceWithStorage(t, filepath.Join(home, "storage.json"), filepath.Join(home, localfile.DefaultVaultName))
	if err := files.Open(); err != nil {
		t.Fatal(err)
	}
	owner := promptOwner(t, service, ownerauth.ErrCanceled)
	picker := &pickedFiles{}
	service.files = picker
	if change, err := service.OpenVault(context.Background()); err != nil || change.Changed || picker.shown != 1 {
		t.Fatalf("opening on a first run = %+v, %v, picker shown %d times", change, err, picker.shown)
	}
	if change, err := service.CreateVault(context.Background()); err != nil || !change.Changed {
		t.Fatalf("creating on a first run = %+v, %v", change, err)
	}
	if said := owner.said(); len(said) != 0 {
		t.Fatalf("a first run asked the device: %q", said)
	}
}
