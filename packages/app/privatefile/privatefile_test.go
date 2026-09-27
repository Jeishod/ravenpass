package privatefile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesPrivately(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "nested")
	path := filepath.Join(directory, "record")
	for _, content := range [][]byte{[]byte("first"), []byte("second, longer")} {
		if err := Write(path, content); err != nil {
			t.Fatal(err)
		}
		read, err := Read(path, 64)
		if err != nil || !bytes.Equal(read, content) {
			t.Fatalf("read %q, %v", read, err)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %04o, want 0600", info.Mode().Perm())
	}
	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if directoryInfo.Mode().Perm() != 0700 {
		t.Fatalf("directory mode = %04o, want 0700", directoryInfo.Mode().Perm())
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d files, want no leftover temporary file", len(entries))
	}
}

type recordingExclusion struct {
	err    error
	marked []string
}

func (r *recordingExclusion) Exclude(path string) error {
	r.marked = append(r.marked, path)
	return r.err
}

func TestWriteMarksTheTemporaryFileBeforeItTakesItsName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	for _, failure := range []error{nil, errors.New("no backup mark")} {
		exclusion := &recordingExclusion{err: failure}
		if err := Write(path, []byte("content"), ExcludedFromBackups(exclusion)); err != nil {
			t.Fatalf("mark error %v failed the write: %v", failure, err)
		}
		if len(exclusion.marked) != 1 {
			t.Fatalf("marked %d files, want 1", len(exclusion.marked))
		}
		marked := exclusion.marked[0]
		if marked == path || filepath.Dir(marked) != filepath.Dir(path) {
			t.Fatalf("marked %q, want a file beside %q", marked, path)
		}
	}
}

func TestPrepareNeverCreatesTheDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "unplugged")
	if _, err := Prepare(filepath.Join(directory, "file"), []byte("data")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Prepare in a missing directory: got %v, want fs.ErrNotExist", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Prepare created the directory: %v", err)
	}
}

func TestADiscardedPendingFileLeavesNothing(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "file")
	pending, err := Prepare(destination, []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(pending.Path()); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("pending file: %v, %v", info, err)
	}
	pending.Discard()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 || pending.Placed() {
		t.Fatalf("a discarded file left %d entries", len(entries))
	}
}

func TestReadRefusesWhatIsNotABoundedRegularFile(t *testing.T) {
	directory := t.TempDir()
	empty := filepath.Join(directory, "empty")
	large := filepath.Join(directory, "large")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(large, make([]byte, 65), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{directory, empty, large} {
		if _, err := Read(path, 64); !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("%s: got %v, want fs.ErrInvalid", filepath.Base(path), err)
		}
	}
	if _, err := Read(filepath.Join(directory, "missing"), 64); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: got %v", err)
	}
}
