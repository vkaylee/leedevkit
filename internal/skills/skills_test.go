package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerRemovesSkillAndUpdatesLock(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, ".leedevkit", "skills.d", "demo")
	if err := os.MkdirAll(filepath.Join(skill, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "leedevkit.lock"), []byte(`{"demo":"sha"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&Manager{Root: root}).Run([]string{"remove", "demo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatalf("skill still exists: %v", err)
	}
	lock, err := os.ReadFile(filepath.Join(root, "leedevkit.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(lock), "demo") {
		t.Fatalf("removed skill remains locked: %s", lock)
	}
}

func TestManagerRejectsUnsafeAddURL(t *testing.T) {
	if err := (&Manager{Root: t.TempDir()}).Run([]string{"add", "./local", "--version", "main"}); err == nil {
		t.Fatal("unsafe skill URL accepted")
	}
}

func TestManagerParsesConfiguredSkills(t *testing.T) {
	root := t.TempDir()
	config := `[addons]
skills = [{ url = "https://example.test/alpha.git", version = "v1" }]
`
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := loadConfigured(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "alpha" || entries[0].Version != "v1" {
		t.Fatalf("entries = %#v", entries)
	}
}
