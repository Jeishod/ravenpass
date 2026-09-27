//go:build unix

package api

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/importers/bitwarden"
)

func TestNamedPipeIsRefusedWithoutWaitingForAWriter(t *testing.T) {
	service := newReadyService(t)
	pipe := filepath.Join(t.TempDir(), "export.json")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("named pipes are unavailable: %v", err)
	}
	refused := make(chan error, 1)
	go func() {
		_, err := service.stageImport(pickedFile{path: pipe}, bitwarden.Open)
		refused <- err
	}()
	select {
	case err := <-refused:
		assertFailure(t, err, failureImportUnrecognized)
	case <-time.After(hangLimit):
		t.Fatal("choosing a named pipe waits for a writer")
	}
}
