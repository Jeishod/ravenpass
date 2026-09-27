package savedfile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const testLimit = 1 << 10

func assertOnlyFile(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d files, want only the chosen one", len(entries))
	}
}

func assertContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s holds %q, want %q", filepath.Base(path), got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("%s permissions = %04o, want 0600", filepath.Base(path), info.Mode().Perm())
	}
}

func TestReplaceRoundTripAndConfirmedReplacement(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "vault.rpv")
	ciphertext := []byte{0x50, 0x41, 0x53, 0x59, 0x00, 0xff}
	if err := Replace(path, ciphertext, testLimit); err != nil {
		t.Fatal(err)
	}
	assertContent(t, path, ciphertext)
	read, err := Read(path, testLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read, ciphertext) {
		t.Fatal("the file read back other bytes")
	}
	replacement := []byte("replacement")
	if err := Replace(path, replacement, testLimit); err != nil {
		t.Fatalf("replace error = %v", err)
	}
	assertContent(t, path, replacement)
	assertOnlyFile(t, directory)
}

func TestARejectedWriteKeepsTheExistingFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "vault.rpv")
	original := []byte("original")
	if err := Replace(path, original, testLimit); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"empty": nil, "oversized": make([]byte, testLimit+1)} {
		if err := Replace(path, data, testLimit); !errors.Is(err, ErrInvalidSize) {
			t.Fatalf("%s replacement error = %v, want invalid size", name, err)
		}
		if err := Create(filepath.Join(directory, "new.rpv"), data, testLimit); !errors.Is(err, ErrInvalidSize) {
			t.Fatalf("%s creation error = %v, want invalid size", name, err)
		}
	}
	assertContent(t, path, original)
	assertOnlyFile(t, directory)
}

func TestReadRejectsNonRegularEmptyAndOversizedFiles(t *testing.T) {
	directory := t.TempDir()
	if _, err := Read(directory, testLimit); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("directory error = %v", err)
	}
	empty := filepath.Join(directory, "empty.rpv")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(empty, testLimit); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("empty file error = %v", err)
	}
	large := filepath.Join(directory, "large.rpv")
	file, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(testLimit + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(large, testLimit); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("oversized file error = %v", err)
	}
}

func TestCreateWritesANewVerifiedFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Personal 2026-09-25 10-00-00.rpv")
	data := []byte("encrypted copy")
	if err := Create(path, data, testLimit); err != nil {
		t.Fatal(err)
	}
	assertContent(t, path, data)
	assertOnlyFile(t, directory)
}

func TestCreateNeverReplacesAFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "vault.rpv")
	original := []byte("the owner's file")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Create(path, []byte("backup"), testLimit); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("creating over a file = %v, want fs.ErrExist", err)
	}
	assertContent(t, path, original)
	assertOnlyFile(t, directory)

	folder := filepath.Join(directory, "folder.rpv")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Create(folder, []byte("backup"), testLimit); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("creating over a folder = %v, want fs.ErrExist", err)
	}
}

func TestCreateInAMissingFolderLeavesNothing(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "unplugged")
	if err := Create(filepath.Join(directory, "vault.rpv"), []byte("backup"), testLimit); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("creating in a missing folder = %v, want fs.ErrNotExist", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("creating in a missing folder made it: %v", err)
	}
}
