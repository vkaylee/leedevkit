package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidSkillConfigurationPreservesLock(t *testing.T) {
	for _, config := range []string{"[addons\n", "[addons]\nskills = [{url = 'file:///tmp/skill', version = 'main'}]\n"} {
		t.Run(config, func(t *testing.T) {
			root := t.TempDir()
			lock := filepath.Join(root, "leedevkit.lock")
			if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lock, []byte("preserve"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := (&Manager{Root: root}).Run([]string{"install"}); err == nil {
				t.Fatal("invalid configuration accepted")
			}
			if got, err := os.ReadFile(lock); err != nil || string(got) != "preserve" {
				t.Fatalf("lock changed: %q, %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(root, ".agent", "skills.d")); !os.IsNotExist(err) {
				t.Fatalf("skill directory created: %v", err)
			}
		})
	}
}

func TestEmptySkillsCommandsDoNotCreateState(t *testing.T) {
	root := t.TempDir()
	for _, action := range []string{"list", "update"} {
		if err := (&Manager{Root: root}).Run([]string{action}); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("empty project changed: %v, %v", entries, err)
	}
}
