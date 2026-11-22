package skills

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneSkillActivatesAndRollsBackOnFailure(t *testing.T) {
	repos := t.TempDir()
	repo := filepath.Join(repos, "demo.git")
	for _, args := range [][]string{
		{"init", "--initial-branch=main", repo},
		{"-C", repo, "config", "user.name", "Fixture"},
		{"-C", repo, "config", "user.email", "fixture@example.invalid"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("fixture skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", repo, "add", "SKILL.md"}, {"-C", repo, "commit", "-m", "fixture"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	revision, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "url.file://"+filepath.ToSlash(repos)+"/.insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://fixture.invalid/")
	t.Setenv("GIT_CONFIG_KEY_1", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_1", "always")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	for _, tc := range []struct {
		name, branch, lock, wantError string
	}{
		{"success", "main", `{"existing":"unchanged"}`, ""},
		{"missing branch", "missing", `{"existing":"unchanged"}`, "git clone"},
		{"invalid lock", "main", `not-json`, "read skill lock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			lockPath := filepath.Join(root, "leedevkit.lock")
			if err := os.WriteFile(lockPath, []byte(tc.lock), 0o600); err != nil {
				t.Fatal(err)
			}
			manager := Manager{Root: root}
			err := manager.Run([]string{"add", "https://fixture.invalid/demo.git", "--version", tc.branch})
			if tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("add error = %v, want %q", err, tc.wantError)
			}
			target := filepath.Join(root, ".agent", "skills.d", "demo")
			data, err := os.ReadFile(lockPath)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" {
				if string(data) != tc.lock {
					t.Fatalf("lock changed: %s", data)
				}
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatalf("target survived failed add: %v", err)
				}
			} else {
				var lock map[string]string
				if err := json.Unmarshal(data, &lock); err != nil || lock["demo"] != strings.TrimSpace(string(revision)) || lock["existing"] != "unchanged" {
					t.Fatalf("lock = %s, error = %v", data, err)
				}
				if got, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil || string(got) != "fixture skill\n" {
					t.Fatalf("skill = %q, error = %v", got, err)
				}
			}
			entries, err := os.ReadDir(filepath.Dir(target))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".skills-stage-") {
					t.Fatalf("staging directory remains: %s", entry.Name())
				}
			}
		})
	}
}
