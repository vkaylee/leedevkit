package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitProjectRelativeSourceCreatesProjectOwnedAssets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "release")
	for _, dir := range []string{"templates", ".agent/rules"} {
		if err := os.MkdirAll(filepath.Join(source, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.default.toml"), []byte("[project]\nname = 'fixture'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".agent", "rules", "test.md"), []byte("rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InitProject(root, "release", false); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "leedevkit.toml")); err != nil || !strings.Contains(string(data), "fixture") {
		t.Fatalf("project config = %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(root, ".agent", "rules", "test.md")); err != nil || string(data) != "rule" {
		t.Fatalf("project rule = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".leedevkit")); !os.IsNotExist(err) {
		t.Fatalf("project runtime unexpectedly created: %v", err)
	}
}

func TestInitProjectRejectsMissingReleaseAssetsWithoutProjectMutation(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root, "missing", false); err == nil || !strings.Contains(err.Error(), "release rules asset unavailable") {
		t.Fatalf("init error = %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("project mutated: %v, %v", entries, err)
	}
}

func TestCopyExecutableRejectsNonExecutableAndPreservesTarget(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	if err := os.WriteFile(source, []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyExecutable(source, target); err == nil {
		t.Fatal("non-executable source accepted")
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "preserve" {
		t.Fatalf("target changed = %q, %v", data, err)
	}
}
