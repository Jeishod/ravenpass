package preferences

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func newHiddenLockStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := New(path, devicePrefers(), LockWhenHidden())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAutoLockDefaultsToOnAfterFifteenMinutes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	want := AutoLock{Enabled: true, After: 15 * time.Minute}
	if got := newStore(t, path).AutoLock(); got != want {
		t.Fatalf("auto-lock without a record = %+v", got)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"ru","clipboardKept":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := newStore(t, path).AutoLock(); got != want {
		t.Fatalf("auto-lock of a record that does not name the choice = %+v", got)
	}
}

func TestAutoLockSurvivesOtherChangesAndAReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	chosen := AutoLock{Enabled: false, After: 5 * time.Minute}
	if err := store.SetAutoLock(chosen); err != nil {
		t.Fatal(err)
	}
	if err := store.SetClipboardClearing(ClipboardClearing{Enabled: true, After: 30 * time.Second}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLanguage(Russian); err != nil {
		t.Fatal(err)
	}
	reopened := newStore(t, path)
	if got := reopened.AutoLock(); got != chosen {
		t.Fatalf("auto-lock after a restart = %+v, want %+v", got, chosen)
	}
	if got := reopened.ClipboardClearing().After; got != 30*time.Second {
		t.Fatalf("recording auto-lock lost the clipboard delay: %v", got)
	}
}

func TestUnofferedAutoLockDelaysAreRefusedAndIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	for _, after := range []time.Duration{0, 30 * time.Second, 90 * time.Second, 1500 * time.Millisecond, 2 * time.Hour} {
		if err := store.SetAutoLock(AutoLock{Enabled: true, After: after}); !errors.Is(err, ErrUnsupportedAutoLockDelay) {
			t.Fatalf("delay %v: got %v, want ErrUnsupportedAutoLockDelay", after, err)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused delay wrote a record: %v", err)
	}
	for _, recorded := range []string{"7", "0"} {
		if err := os.WriteFile(path, []byte(`{"version":1,"autoLockOff":true,"autoLockSeconds":`+recorded+`}`), 0600); err != nil {
			t.Fatal(err)
		}
		got := newStore(t, path).AutoLock()
		if got.Enabled || got.After != 15*time.Minute {
			t.Fatalf("record with the unoffered delay %s = %+v", recorded, got)
		}
	}
}

func TestADeviceThatLocksWhenHiddenLocksAtOnceByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newHiddenLockStore(t, path)
	if !store.LocksWhenHidden() || newStore(t, path).LocksWhenHidden() {
		t.Fatal("only the store made with LockWhenHidden locks when hidden")
	}
	if got := store.AutoLock(); got != (AutoLock{Enabled: true, After: 0}) {
		t.Fatalf("auto-lock without a record = %+v, want at once", got)
	}
	want := []time.Duration{0, time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour}
	if got := store.AutoLockDelays(); !slices.Equal(got, want) {
		t.Fatalf("offered delays = %v, want %v", got, want)
	}
	if got := newStore(t, path).AutoLockDelays(); slices.Contains(got, 0) {
		t.Fatalf("a device that counts from its last input offers locking at once: %v", got)
	}
}

func TestAChosenDelayOnADeviceThatLocksWhenHiddenOutlastsTheDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := newHiddenLockStore(t, path).SetAutoLock(AutoLock{Enabled: true, After: 15 * time.Minute}); err != nil {
		t.Fatal(err)
	}
	if got := newHiddenLockStore(t, path).AutoLock(); got.After != 15*time.Minute {
		t.Fatalf("auto-lock after choosing fifteen minutes = %+v", got)
	}
	if err := newHiddenLockStore(t, path).SetAutoLock(AutoLock{Enabled: false, After: 0}); err != nil {
		t.Fatal(err)
	}
	if got := newHiddenLockStore(t, path).AutoLock(); got != (AutoLock{Enabled: false, After: 0}) {
		t.Fatalf("auto-lock after choosing at once and turning it off = %+v", got)
	}
}
