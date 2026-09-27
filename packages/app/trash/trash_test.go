package trash

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestMoveToTrashRefusesWhatItCannotMove(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	for _, path := range []string{"", missing} {
		if err := MoveToTrash(path); !errors.Is(err, ErrRefused) && !errors.Is(err, ErrUnavailable) {
			t.Fatalf("MoveToTrash(%q) = %v", path, err)
		}
	}
}
