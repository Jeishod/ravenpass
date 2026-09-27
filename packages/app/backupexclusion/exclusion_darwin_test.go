//go:build darwin && cgo

package backupexclusion

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// excludeItemAttribute is the xattr NSURLIsExcludedFromBackupKey writes; tmutil reports every temp file excluded.
const excludeItemAttribute = "com.apple.metadata:com_apple_backup_excludeItem"

func excludedFromTimeMachine(t *testing.T, path string) bool {
	t.Helper()
	output, err := exec.Command("xattr", path).CombinedOutput()
	if err != nil {
		t.Fatalf("xattr: %v: %s", err, output)
	}
	return slices.Contains(strings.Fields(string(output)), excludeItemAttribute)
}

func TestAnExcludedFileKeepsItsMarkThroughARename(t *testing.T) {
	directory := t.TempDir()
	temporary := filepath.Join(directory, ".records")
	final := filepath.Join(directory, "records.json")
	if err := os.WriteFile(temporary, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if excludedFromTimeMachine(t, temporary) {
		t.Fatal("a new file started out excluded")
	}
	if err := (System{}).Exclude(temporary); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, final); err != nil {
		t.Fatal(err)
	}
	if !excludedFromTimeMachine(t, final) {
		t.Fatal("the renamed file lost its exclusion")
	}
}

func TestExcludingAMissingFileFails(t *testing.T) {
	if err := (System{}).Exclude(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a missing file was reported excluded")
	}
}
