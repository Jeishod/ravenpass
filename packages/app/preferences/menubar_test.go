package preferences

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestMenuBarFollowsTheRecordedLanguage(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "preferences.json"))
	english := MenuBar{Open: "Open Ravenpass", Lock: "Lock vault", Quit: "Quit Ravenpass"}
	if got := store.MenuBar(); got != english {
		t.Fatalf("menu without a record = %+v", got)
	}
	if err := store.SetLanguage(Russian); err != nil {
		t.Fatal(err)
	}
	russian := MenuBar{Open: "Открыть Ravenpass", Lock: "Заблокировать хранилище", Quit: "Выйти из Ravenpass"}
	if got := store.MenuBar(); got != russian {
		t.Fatalf("menu after choosing Russian = %+v", got)
	}
}

func TestLanguageListenersHearOnlyRecordedChanges(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "preferences.json"))
	heard := 0
	store.OnLanguageChange(func() { heard++ })
	if err := store.SetLanguage("de"); err == nil {
		t.Fatal("an unsupported language was recorded")
	}
	if heard != 0 {
		t.Fatalf("a refused language was announced %d times", heard)
	}
	if err := store.SetLanguage(Russian); err != nil {
		t.Fatal(err)
	}
	if heard != 1 {
		t.Fatalf("a recorded language was announced %d times", heard)
	}
	if count, err := store.AwaitLanguageChange(context.Background(), 0); err != nil || count != 1 {
		t.Fatalf("languages recorded = %d, %v; want 1", count, err)
	}
	waiting, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := store.AwaitLanguageChange(waiting, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a wait with no new language: got %v, want DeadlineExceeded", err)
	}
}
