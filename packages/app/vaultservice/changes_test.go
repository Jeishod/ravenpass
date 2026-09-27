package vaultservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// quietFor reports whether a wait from seen is still waiting after a short while.
func quietFor(t *testing.T, service *Service, seen uint64) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := service.AwaitVaultChange(ctx, seen)
	return errors.Is(err, context.DeadlineExceeded)
}

func TestAVaultChangeIsAnsweredOnceACaptureIsSaved(t *testing.T) {
	service, _, _ := captureVault(t)
	if !quietFor(t, service, 0) {
		t.Fatal("a wait answered before any save")
	}
	createTestCredential(t, service, vault.CredentialInput{Label: "Example", Websites: []string{captureOrigin}, Login: "alex", Password: "old"})
	if _, err := service.CaptureOffer(Capture{Requester: OriginRequester(captureOrigin), Account: "sam", Password: "typed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveCapture(Capture{Requester: OriginRequester(captureOrigin), Account: "sam", Password: "typed"}, CaptureChoice{}, ""); !errors.Is(err, ErrNameRefused) {
		t.Fatalf("a refused save: got %v", err)
	}
	if !quietFor(t, service, 0) {
		t.Fatal("a wait answered for the workspace's own save, a comparison or a refused save")
	}

	answered := make(chan uint64, 1)
	go func() {
		count, err := service.AwaitVaultChange(context.Background(), 0)
		if err != nil {
			t.Error(err)
		}
		answered <- count
	}()
	if _, err := service.SaveCapture(Capture{Requester: OriginRequester(captureOrigin), Account: "sam", Password: "typed"}, CaptureChoice{Name: "Example", Account: "sam"}, ""); err != nil {
		t.Fatal(err)
	}
	select {
	case count := <-answered:
		if count != 1 {
			t.Fatalf("change count = %d, want 1", count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a wait was not answered after a save")
	}
	if count, err := service.AwaitVaultChange(context.Background(), 0); err != nil || count != 1 {
		t.Fatalf("a wait from an older count = %d, error = %v", count, err)
	}
	if !quietFor(t, service, 1) {
		t.Fatal("a wait from the current count answered before the next save")
	}
}
