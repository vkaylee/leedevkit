package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitSelectsRustTemplateAndKeepsExistingRule(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	for _, dir := range []string{filepath.Join(assets, "templates"), filepath.Join(assets, ".agent", "rules"), filepath.Join(root, ".agent", "rules")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(root, "Cargo.toml"): "[package]", filepath.Join(root, ".agent", "rules", "base.md"): "user",
		filepath.Join(assets, "templates", "leedevkit.rust.toml"): "rust config", filepath.Join(assets, ".agent", "rules", "base.md"): "release",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Init(root, assets, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "leedevkit.toml")); err != nil || string(got) != "rust config" {
		t.Fatalf("config = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, ".agent", "rules", "base.md")); err != nil || string(got) != "user" {
		t.Fatalf("rule = %q, %v", got, err)
	}
}
