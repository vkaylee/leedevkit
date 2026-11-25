package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyPayloadRejectsSymlinksAndPreservesSource(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "VERSION")); err != nil {
		t.Fatal(err)
	}
	if err := copyPayload(source, target); err == nil || !strings.Contains(err.Error(), "refusing symlink") {
		t.Fatalf("copy error = %v", err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "unchanged" {
		t.Fatalf("outside file changed: %q, %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(target, "VERSION")); !os.IsNotExist(err) {
		t.Fatalf("symlink copied: %v", err)
	}
}

func TestSyncRulesRejectsSymlinkAndPreservesExistingFiles(t *testing.T) {
	root, source, outside := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".agent", "rules")); err != nil {
		t.Fatal(err)
	}
	if err := SyncRules(root, source); err == nil || !strings.Contains(err.Error(), "symlinked rules") {
		t.Fatalf("sync error = %v", err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside changed: %v, %v", entries, err)
	}
}

func TestCopyPathDoesNotOverwriteExistingDestination(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	if err := os.WriteFile(source, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyPath(source, target); err == nil {
		t.Fatal("existing target overwritten")
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "original" {
		t.Fatalf("target = %q, %v", got, err)
	}
}
