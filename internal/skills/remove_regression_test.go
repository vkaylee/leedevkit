package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveRollsBackSkillWhenLockIsInvalid(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".agent", "skills.d", "demo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(root, "leedevkit.lock")
	if err := os.WriteFile(lock, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&Manager{Root: root}).Run([]string{"remove", "demo"}); err == nil || !strings.Contains(err.Error(), "read skill lock") {
		t.Fatalf("remove error = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil || string(data) != "preserve" {
		t.Fatalf("skill = %q, %v", data, err)
	}
	if data, err := os.ReadFile(lock); err != nil || string(data) != "invalid" {
		t.Fatalf("lock = %q, %v", data, err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(target), ".demo.remove-backup")); !os.IsNotExist(err) {
		t.Fatalf("backup remains: %v", err)
	}
}
