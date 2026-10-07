package api

import (
	"path/filepath"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/genhistory"
)

func newGeneratorService(t *testing.T) *Service {
	t.Helper()
	service := newReadyService(t)
	generator, err := genhistory.New(filepath.Join(t.TempDir(), "generator"), service.vault, func() int {
		return service.preferences.GeneratorHistory().Days
	})
	if err != nil {
		t.Fatal(err)
	}
	service.generator = generator
	return service
}

func TestATurnedOffGeneratorHistoryRecordsNothing(t *testing.T) {
	service := newGeneratorService(t)
	if err := service.SetGeneratorHistorySetting(false, 30); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordGeneratedPassword("secret", genhistory.ModeCharacters); err != nil {
		t.Fatal(err)
	}
	history, err := service.GeneratorHistory()
	if err != nil || len(history) != 0 {
		t.Fatalf("history = %+v, %v", history, err)
	}
}

func TestTurningTheGeneratorHistoryOffForgetsIt(t *testing.T) {
	service := newGeneratorService(t)
	if err := service.RecordGeneratedPassword("secret", genhistory.ModeWords); err != nil {
		t.Fatal(err)
	}
	if count, err := service.CountGeneratorHistoryPast(0); err != nil || count != 1 {
		t.Fatalf("CountGeneratorHistoryPast(0) = %d, %v", count, err)
	}
	if err := service.SetGeneratorHistorySetting(false, 30); err != nil {
		t.Fatal(err)
	}
	history, err := service.GeneratorHistory()
	if err != nil || len(history) != 0 {
		t.Fatalf("history after turning it off = %+v, %v", history, err)
	}
	setting, err := service.GetGeneratorHistorySetting()
	if err != nil || setting.Enabled || setting.Days != 30 {
		t.Fatalf("setting = %+v, %v", setting, err)
	}
}

func TestAGeneratorHistoryPeriodNotOfferedIsRefused(t *testing.T) {
	service := newGeneratorService(t)
	assertFailure(t, service.SetGeneratorHistorySetting(true, 12), failureInvalidItem)
}
