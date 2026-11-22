package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerRemovesSkillAndUpdatesLock(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, ".agent", "skills.d", "demo")
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

func TestManagerDoesNotMoveLegacySkillsAndRemoveDoesNotDeleteThem(t *testing.T) {
	root := t.TempDir()
	legacySkill := filepath.Join(root, ".leedevkit", "skills.d", "demo")
	currentSkill := filepath.Join(root, ".agent", "skills.d", "demo")
	otherSkill := filepath.Join(root, ".agent", "skills.d", "other")
	for path, content := range map[string]string{
		filepath.Join(legacySkill, "SKILL.md"):           "legacy",
		filepath.Join(legacySkill, "nested", "SKILL.md"): "legacy-other",
		filepath.Join(currentSkill, "SKILL.md"):          "project-owned",
		filepath.Join(otherSkill, "SKILL.md"):            "other",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lockPath := filepath.Join(root, "leedevkit.lock")
	if err := os.WriteFile(lockPath, []byte(`{"demo":"old","unrelated":"keep"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (&Manager{Root: root}).Run([]string{"update"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(currentSkill, "SKILL.md"): "project-owned",
		filepath.Join(otherSkill, "SKILL.md"):   "other",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("read %s = %q, %v; want %q", path, got, err, want)
		}
	}
	lock, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var entries map[string]string
	if err := json.Unmarshal(lock, &entries); err != nil {
		t.Fatal(err)
	}
	if entries["unrelated"] != "keep" || entries["demo"] != "old" {
		t.Fatalf("lock entries = %#v", entries)
	}
	if err := (&Manager{Root: root}).Run([]string{"remove", "demo"}); err == nil {
		t.Fatal("remove deleted a legacy-only skill")
	}
	for path, want := range map[string]string{
		filepath.Join(root, ".leedevkit", "skills.d", "demo", "SKILL.md"):           "legacy",
		filepath.Join(root, ".leedevkit", "skills.d", "demo", "nested", "SKILL.md"): "legacy-other",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("legacy data %s = %q, %v; want %q", path, got, err, want)
		}
	}
}

func TestManagerRejectsLegacyNameCollisionWithoutChangingDataOrLock(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, ".leedevkit", "skills.d", "demo")
	project := filepath.Join(root, ".agent", "skills.d", "demo")
	for path, content := range map[string]string{
		filepath.Join(legacy, "SKILL.md"):  "legacy",
		filepath.Join(project, "SKILL.md"): "project",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, "leedevkit.lock")
	lockBefore := []byte(`{"demo":"project-sha","other":"keep"}`)
	if err := os.WriteFile(lockPath, lockBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{Root: root}
	if err := manager.add("https://example.test/demo.git", "main"); err == nil {
		t.Fatal("add accepted name colliding with legacy skill")
	}
	if err := manager.remove("demo"); err == nil {
		t.Fatal("remove accepted name colliding with legacy skill")
	}
	if err := manager.update(); err != nil {
		t.Fatal(err)
	}
	if err := manager.writeLock(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(legacy, "SKILL.md"):  "legacy",
		filepath.Join(project, "SKILL.md"): "project",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("skill %s = %q, %v; want %q", path, got, err, want)
		}
	}
	lockAfter, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var lockEntries map[string]string
	if err := json.Unmarshal(lockAfter, &lockEntries); err != nil {
		t.Fatal(err)
	}
	if lockEntries["demo"] != "project-sha" || lockEntries["other"] != "keep" {
		t.Fatalf("lock entries = %#v; want legacy/project and unrelated entries preserved", lockEntries)
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
skills = [
  { url = "https://example.test/alpha.git", version = "v1" },
  { url = "https://example.test/beta.git" },
]
`
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := loadConfigured(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "alpha" || entries[0].Version != "v1" || entries[1].Name != "beta" || entries[1].Version != "main" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestManagerRejectsInvalidActionsAndNames(t *testing.T) {
	manager := &Manager{Root: t.TempDir()}
	for _, args := range [][]string{
		{"unknown"},
		{"remove"},
		{"remove", "../escape"},
		{"add", "https://example.test/skill.git"},
	} {
		if err := manager.Run(args); err == nil {
			t.Fatalf("accepted invalid args %#v", args)
		}
	}
}
