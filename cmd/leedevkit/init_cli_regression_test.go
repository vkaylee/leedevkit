package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCLIUsesLocalAssetsWithoutCreatingRuntime(t *testing.T) {
	project, assets := t.TempDir(), t.TempDir()
	for _, dir := range []string{"templates", ".agent/rules"} {
		if err := os.MkdirAll(filepath.Join(assets, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(assets, "templates", "leedevkit.default.toml"), []byte("[project]\nname = 'fixture'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, ".agent", "rules", "fixture.md"), []byte("rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	t.Setenv("DEVKIT_LOCAL_PATH", assets)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"init"}, strings.NewReader(""), &stdout, &stderr); code != 0 || !strings.Contains(stderr.String(), "Project initialized.") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	if got, err := os.ReadFile(filepath.Join(project, "leedevkit.toml")); err != nil || !strings.Contains(string(got), "fixture") {
		t.Fatalf("config = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(project, ".agent", "rules", "fixture.md")); err != nil || string(got) != "rule" {
		t.Fatalf("rule = %q, %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".leedevkit")); !os.IsNotExist(err) {
		t.Fatalf("unexpected project runtime: %v", err)
	}
}
