package preferences

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTheGeneratorHistoryIsKeptThirtyDaysByDefault(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "preferences.json"))
	if got := store.GeneratorHistory(); got != (GeneratorHistory{Enabled: true, Days: 30}) {
		t.Fatalf("default = %+v", got)
	}
}

func TestTheGeneratorHistoryChoiceSurvivesAReloadAndKeepsItsPeriodWhileOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := newStore(t, path).SetGeneratorHistory(GeneratorHistory{Enabled: false, Days: 90}); err != nil {
		t.Fatal(err)
	}
	if got := newStore(t, path).GeneratorHistory(); got != (GeneratorHistory{Enabled: false, Days: 90}) {
		t.Fatalf("after a restart = %+v", got)
	}
}

func TestAGeneratorHistoryPeriodNotOfferedIsRefusedAndIgnoredOnRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := newStore(t, path)
	if err := store.SetGeneratorHistory(GeneratorHistory{Enabled: true, Days: 12}); !errors.Is(err, ErrUnsupportedGeneratorHistoryDays) {
		t.Fatalf("err = %v, want ErrUnsupportedGeneratorHistoryDays", err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"generatorHistoryDays":12}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := newStore(t, path).GeneratorHistory(); got.Days != 30 {
		t.Fatalf("a stored period not offered read as %d days", got.Days)
	}
}
