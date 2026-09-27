package api

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/preferences"
)

func TestAutoLockSettings(t *testing.T) {
	service := &Service{preferences: newTestPreferences(t)}
	current, err := service.GetAutoLock()
	if err != nil {
		t.Fatal(err)
	}
	if !current.Enabled || current.Seconds != 900 || !slices.Equal(current.Offered, []int{60, 300, 900, 1800, 3600}) {
		t.Fatalf("default auto-lock = %+v", current)
	}
	if err := service.SetAutoLock(false, 300); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetAutoLock(); current.Enabled || current.Seconds != 300 {
		t.Fatalf("auto-lock after turning it off = %+v", current)
	}
	for _, seconds := range []int{0, -1, 30, 7200} {
		err := service.SetAutoLock(true, seconds)
		if err == nil || err.Error() != failurePrefix+string(failureAutoLockDelayUnsupported) {
			t.Fatalf("delay %d: error = %v", seconds, err)
		}
	}
	if current, _ = service.GetAutoLock(); current.Enabled || current.Seconds != 300 {
		t.Fatalf("a refused delay changed the setting to %+v", current)
	}
}

func TestAHostThatLocksWhenHiddenOffersLockingAtOnce(t *testing.T) {
	settings, err := preferences.New(filepath.Join(t.TempDir(), "preferences.json"), func() []string { return nil }, preferences.LockWhenHidden())
	if err != nil {
		t.Fatal(err)
	}
	service := newServiceOnHostWith(t, Host{}, settings)
	if !service.Capabilities().LockWhenHidden {
		t.Fatal("a host that locks when hidden does not say so")
	}
	current, err := service.GetAutoLock()
	if err != nil {
		t.Fatal(err)
	}
	if !current.Enabled || current.Seconds != 0 || !slices.Equal(current.Offered, []int{0, 60, 300, 900, 1800, 3600}) {
		t.Fatalf("default auto-lock = %+v", current)
	}
	if err := service.SetAutoLock(true, 900); err != nil {
		t.Fatal(err)
	}
	if err := service.SetAutoLock(true, 0); err != nil {
		t.Fatalf("choosing to lock at once = %v", err)
	}
	if newServiceOnHost(t, Host{}).Capabilities().LockWhenHidden {
		t.Fatal("a host that counts from the last input says it locks when hidden")
	}
}
